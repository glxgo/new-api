package controller

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

// Setup2FARequest 设置2FA请求结构
type Setup2FARequest struct {
	Code string `json:"code" binding:"required"`
}

// Verify2FARequest 验证2FA请求结构
type Verify2FARequest struct {
	Code string `json:"code" binding:"required"`
}

// Setup2FAResponse 设置2FA响应结构
type Setup2FAResponse struct {
	Secret      string   `json:"secret"`
	QRCodeData  string   `json:"qr_code_data"`
	BackupCodes []string `json:"backup_codes"`
}

// Setup2FA 初始化2FA设置
func Setup2FA(c *gin.Context) {
	userId := c.GetInt("id")

	// 检查用户是否已经启用2FA
	existing, err := model.GetTwoFAByUserId(userId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if existing != nil && existing.IsEnabled {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "用户已启用2FA，请先禁用后重新设置",
		})
		return
	}

	// 获取用户信息
	user, err := model.GetUserById(userId, false)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	// 生成TOTP密钥
	key, err := common.GenerateTOTPSecret(user.Username)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "生成2FA密钥失败",
		})
		common.SysLog("生成TOTP密钥失败: " + err.Error())
		return
	}

	// 生成备用码
	backupCodes, err := common.GenerateBackupCodes()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "生成备用码失败",
		})
		common.SysLog("生成备用码失败: " + err.Error())
		return
	}

	// 生成二维码数据
	qrCodeData := common.GenerateQRCodeData(key.Secret(), user.Username)

	if err := model.StorePendingTwoFA(userId, key.Secret(), backupCodes); err != nil {
		common.ApiError(c, err)
		return
	}

	session := sessions.Default(c)
	session.Set("twofa_setup", common.GenerateHMAC(key.Secret()))
	if err := session.Save(); err != nil {
		common.ApiError(c, err)
		return
	}
	model.RecordLog(userId, model.LogTypeSystem, "开始设置两步验证")

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "2FA设置初始化成功，请使用认证器扫描二维码并输入验证码完成设置",
		"data": Setup2FAResponse{
			Secret:      key.Secret(),
			QRCodeData:  qrCodeData,
			BackupCodes: backupCodes,
		},
	})
}

// Enable2FA 启用2FA
func Enable2FA(c *gin.Context) {
	var req Setup2FARequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "参数错误",
		})
		return
	}

	userId := c.GetInt("id")

	// 获取2FA记录
	twoFA, err := model.GetTwoFAByUserId(userId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if twoFA == nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "请先完成2FA初始化设置",
		})
		return
	}
	if twoFA.IsEnabled {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "2FA已经启用",
		})
		return
	}

	session := sessions.Default(c)
	setupHash, _ := session.Get("twofa_setup").(string)
	if setupHash == "" || setupHash != common.GenerateHMAC(twoFA.Secret) {
		common.ApiError(c, model.ErrSecurityProof)
		return
	}

	// 验证TOTP验证码
	cleanCode, err := common.ValidateNumericCode(req.Code)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": common.SanitizePublicError(err.Error()),
		})
		return
	}

	if !common.ValidateTOTPCode(twoFA.Secret, cleanCode) {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "验证码或备用码错误，请重试",
		})
		return
	}

	session.Delete("twofa_setup")
	if err := session.Save(); err != nil {
		common.ApiError(c, err)
		return
	}
	// 启用2FA
	if err := twoFA.Enable(); err != nil {
		common.ApiError(c, err)
		return
	}

	// 记录操作日志
	recordUserSecurityAudit(c, userId, "user.2fa_enable", nil)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "两步验证启用成功",
	})
}

// Disable2FA 禁用2FA
func Disable2FA(c *gin.Context) {

	userId := c.GetInt("id")

	// 获取2FA记录
	twoFA, err := model.GetTwoFAByUserId(userId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if twoFA == nil || !twoFA.IsEnabled {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "用户未启用2FA",
		})
		return
	}

	// 禁用2FA
	if err := model.DisableTwoFA(userId); err != nil {
		common.ApiError(c, err)
		return
	}

	// 记录操作日志
	recordUserSecurityAudit(c, userId, "user.2fa_disable", nil)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "两步验证已禁用",
	})
}

// Get2FAStatus 获取用户2FA状态
func Get2FAStatus(c *gin.Context) {
	userId := c.GetInt("id")

	twoFA, err := model.GetTwoFAByUserId(userId)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	status := map[string]interface{}{
		"enabled": false,
		"locked":  false,
	}

	if twoFA != nil {
		status["enabled"] = twoFA.IsEnabled
		status["locked"] = twoFA.IsLocked()
		if twoFA.IsEnabled {
			// 获取剩余备用码数量
			backupCount, err := model.GetUnusedBackupCodeCount(userId)
			if err != nil {
				common.SysLog("获取备用码数量失败: " + err.Error())
			} else {
				status["backup_codes_remaining"] = backupCount
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    status,
	})
}

// RegenerateBackupCodes 重新生成备用码
func RegenerateBackupCodes(c *gin.Context) {

	userId := c.GetInt("id")

	// 获取2FA记录
	twoFA, err := model.GetTwoFAByUserId(userId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if twoFA == nil || !twoFA.IsEnabled {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "用户未启用2FA",
		})
		return
	}

	// 生成新的备用码
	backupCodes, err := common.GenerateBackupCodes()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "生成备用码失败",
		})
		common.SysLog("生成备用码失败: " + err.Error())
		return
	}

	// 保存新的备用码
	if err := model.CreateBackupCodes(userId, backupCodes); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "保存备用码失败",
		})
		common.SysLog("保存备用码失败: " + err.Error())
		return
	}

	// 记录操作日志
	recordUserSecurityAudit(c, userId, "user.2fa_backup_codes", nil)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "备用码重新生成成功",
		"data": map[string]interface{}{
			"backup_codes": backupCodes,
		},
	})
}

// Verify2FALogin 登录时验证2FA
func Verify2FALogin(c *gin.Context) {
	var req Verify2FARequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "参数错误",
		})
		return
	}

	// 从会话中获取pending用户信息
	session := sessions.Default(c)
	token, _ := session.Get("pending_login").(string)
	session.Delete("pending_login")
	if err := session.Save(); err != nil {
		common.ApiError(c, err)
		return
	}
	payload, err := model.PopSecurityCeremony(token)
	var pending struct {
		UserID       int    `json:"user_id"`
		PasswordHash string `json:"password_hash"`
		AuthVersion  int    `json:"auth_version"`
	}
	if err != nil || common.UnmarshalJsonStr(payload, &pending) != nil {
		common.ApiErrorMsg(c, "会话已过期，请重新登录")
		return
	}
	user, err := model.GetUserById(pending.UserID, true)
	if err != nil || user.Status != common.UserStatusEnabled || (pending.PasswordHash != common.GenerateHMAC(user.Password) || pending.AuthVersion != user.AuthVersion) {
		common.ApiErrorMsg(c, "会话已过期，请重新登录")
		return
	}
	// 获取2FA记录
	twoFA, err := model.GetTwoFAByUserId(user.Id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if twoFA == nil || !twoFA.IsEnabled {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "用户未启用2FA",
		})
		return
	}

	// 验证TOTP验证码或备用码
	cleanCode, err := common.ValidateNumericCode(req.Code)
	isValidTOTP := false
	isValidBackup := false

	if err == nil {
		// 尝试验证TOTP
		isValidTOTP, _ = twoFA.ValidateTOTPAndUpdateUsage(cleanCode)
	}

	if !isValidTOTP {
		// 尝试验证备用码
		isValidBackup, err = twoFA.ValidateBackupCodeAndUpdateUsage(req.Code)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": common.SanitizePublicError(err.Error()),
			})
			return
		}
	}

	if !isValidTOTP && !isValidBackup {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "验证码或备用码错误，请重试",
		})
		return
	}

	// 2FA验证成功，清理pending会话信息并完成登录
	session.Delete("pending_username")
	session.Delete("pending_user_id")
	session.Save()

	setupLogin(user, c)
}

// Admin2FAStats 管理员获取2FA统计信息
func Admin2FAStats(c *gin.Context) {
	stats, err := model.GetTwoFAStats()
	if err != nil {
		common.ApiError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    stats,
	})
}

// AdminDisable2FA 管理员强制禁用用户2FA
func AdminDisable2FA(c *gin.Context) {
	userIdStr := c.Param("id")
	userId, err := strconv.Atoi(userIdStr)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "用户ID格式错误",
		})
		return
	}

	// 检查目标用户权限
	targetUser, err := model.GetUserById(userId, false)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	myRole := c.GetInt("role")
	if !canManageTargetRole(myRole, targetUser.Role) {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "无权操作同级或更高级用户的2FA设置",
		})
		return
	}

	// 禁用2FA
	if err := model.DisableTwoFA(userId); err != nil {
		if errors.Is(err, model.ErrTwoFANotEnabled) {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "用户未启用2FA",
			})
			return
		}
		common.ApiError(c, err)
		return
	}

	recordManageAuditFor(c, userId, "user.2fa_disable", nil)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "用户2FA已被强制禁用",
	})
}
