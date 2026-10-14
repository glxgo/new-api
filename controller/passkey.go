package controller

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	passkeysvc "github.com/QuantumNous/new-api/service/passkey"
	"github.com/QuantumNous/new-api/setting/system_setting"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"github.com/go-webauthn/webauthn/protocol"
	webauthnlib "github.com/go-webauthn/webauthn/webauthn"
)

func PasskeyRegisterBegin(c *gin.Context) {
	if !system_setting.PasskeySettingsSnapshot().Enabled {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "管理员未启用 Passkey 登录",
		})
		return
	}

	user, err := getSessionUser(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": common.SanitizePublicError(err.Error()),
		})
		return
	}

	if !service.RequireSecurityProof(c, "passkey.register") {
		return
	}

	credential, err := model.GetPasskeyByUserID(user.Id)
	if err != nil && !errors.Is(err, model.ErrPasskeyNotFound) {
		common.ApiError(c, err)
		return
	}
	if errors.Is(err, model.ErrPasskeyNotFound) {
		credential = nil
	}

	wa, err := passkeysvc.BuildWebAuthn(c.Request)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	waUser := passkeysvc.NewWebAuthnUser(user, credential)
	options := []webauthnlib.RegistrationOption{webauthnlib.WithAuthenticatorSelection(protocol.AuthenticatorSelection{UserVerification: protocol.VerificationRequired, ResidentKey: protocol.ResidentKeyRequirementRequired})}
	if credential != nil {
		descriptor := credential.ToWebAuthnCredential().Descriptor()
		options = append(options, webauthnlib.WithExclusions([]protocol.CredentialDescriptor{descriptor}))
	}

	creation, sessionData, err := wa.BeginRegistration(waUser, options...)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	if err := passkeysvc.SaveSessionData(c, passkeysvc.RegistrationSessionKey, sessionData); err != nil {
		common.ApiError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data": gin.H{
			"options": creation,
		},
	})
}

func PasskeyRegisterFinish(c *gin.Context) {
	if !system_setting.PasskeySettingsSnapshot().Enabled {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "管理员未启用 Passkey 登录",
		})
		return
	}

	user, err := getSessionUser(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": common.SanitizePublicError(err.Error()),
		})
		return
	}

	credentialRecord, err := model.GetPasskeyByUserID(user.Id)
	if err != nil && !errors.Is(err, model.ErrPasskeyNotFound) {
		common.ApiError(c, err)
		return
	}
	if errors.Is(err, model.ErrPasskeyNotFound) {
		credentialRecord = nil
	}

	sessionData, err := passkeysvc.PopSessionData(c, passkeysvc.RegistrationSessionKey)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	wa, err := passkeysvc.BuildWebAuthnForRPID(c.Request, sessionData.RelyingPartyID)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if sessionData.UserVerification != protocol.VerificationRequired {
		common.ApiError(c, model.ErrSecurityProof)
		return
	}

	waUser := passkeysvc.NewWebAuthnUser(user, credentialRecord)
	credential, err := wa.FinishRegistration(waUser, *sessionData, c.Request)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	passkeyCredential := model.NewPasskeyCredentialFromWebAuthn(user.Id, credential)
	if passkeyCredential == nil {
		common.ApiErrorMsg(c, "无法创建 Passkey 凭证")
		return
	}

	passkeyCredential.RPID = &sessionData.RelyingPartyID
	if err := model.UpsertPasskeyCredential(passkeyCredential); err != nil {
		common.ApiError(c, err)
		return
	}

	recordUserSecurityAudit(c, user.Id, "user.passkey_register", nil)
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Passkey 注册成功",
	})
}

func PasskeyDelete(c *gin.Context) {
	user, err := getSessionUser(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": common.SanitizePublicError(err.Error()),
		})
		return
	}

	if !service.RequireSecurityProof(c, "passkey.delete") {
		return
	}

	if err := service.RemoveLoginMethod(user.Id, true, 0); err != nil {
		common.ApiError(c, err)
		return
	}

	recordUserSecurityAudit(c, user.Id, "user.passkey_delete", nil)
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Passkey 已解绑",
	})
}

func PasskeyStatus(c *gin.Context) {
	user, err := getSessionUser(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": common.SanitizePublicError(err.Error()),
		})
		return
	}

	credential, err := model.GetPasskeyByUserID(user.Id)
	if errors.Is(err, model.ErrPasskeyNotFound) {
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "",
			"data": gin.H{
				"enabled": false,
			},
		})
		return
	}
	if err != nil {
		common.ApiError(c, err)
		return
	}

	data := gin.H{
		"enabled":      true,
		"last_used_at": credential.LastUsedAt,
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    data,
	})
}

func PasskeyLoginBegin(c *gin.Context) {
	if !system_setting.PasskeySettingsSnapshot().Enabled {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "管理员未启用 Passkey 登录",
		})
		return
	}

	var request struct {
		RPID string `json:"rp_id"`
	}
	if c.Request.ContentLength > 0 {
		if err := c.ShouldBindJSON(&request); err != nil {
			common.ApiError(c, err)
			return
		}
	}
	wa, rpIDs, err := passkeysvc.BuildLoginWebAuthn(c.Request, request.RPID, "")
	if err != nil {
		common.ApiError(c, err)
		return
	}

	assertion, sessionData, err := wa.BeginDiscoverableLogin(webauthnlib.WithUserVerification(protocol.VerificationRequired))
	if err != nil {
		common.ApiError(c, err)
		return
	}

	if err := passkeysvc.SaveSessionData(c, passkeysvc.LoginSessionKey, sessionData); err != nil {
		common.ApiError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data": gin.H{
			"options": assertion, "rp_ids": rpIDs,
		},
	})
}

func PasskeyLoginFinish(c *gin.Context) {
	if !system_setting.PasskeySettingsSnapshot().Enabled {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "管理员未启用 Passkey 登录",
		})
		return
	}

	sessionData, err := passkeysvc.PopSessionData(c, passkeysvc.LoginSessionKey)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	wa, err := passkeysvc.BuildWebAuthnForRPID(c.Request, sessionData.RelyingPartyID)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if sessionData.UserVerification != protocol.VerificationRequired {
		common.ApiError(c, model.ErrSecurityProof)
		return
	}

	handler := func(rawID, userHandle []byte) (webauthnlib.User, error) {
		// 首先通过凭证ID查找用户
		credential, err := model.GetPasskeyByCredentialID(rawID)
		if err != nil {
			return nil, fmt.Errorf("未找到 Passkey 凭证: %w", err)
		}

		if credential.RPID != nil && *credential.RPID != "" && *credential.RPID != sessionData.RelyingPartyID {
			return nil, model.ErrSecurityProof
		}
		// 通过凭证获取用户
		user := &model.User{Id: credential.UserID}
		if err := user.FillUserById(); err != nil {
			return nil, fmt.Errorf("用户信息获取失败: %w", err)
		}

		if user.Status != common.UserStatusEnabled {
			return nil, errors.New("该用户已被禁用")
		}

		if len(userHandle) > 0 {
			userID, parseErr := strconv.Atoi(string(userHandle))
			if parseErr != nil {
				return nil, model.ErrSecurityProof
			} else if userID != user.Id {
				return nil, errors.New("用户句柄与凭证不匹配")
			}
		}

		return passkeysvc.NewWebAuthnUser(user, credential), nil
	}

	waUser, credential, err := wa.FinishPasskeyLogin(handler, *sessionData, c.Request)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	userWrapper, ok := waUser.(*passkeysvc.WebAuthnUser)
	if !ok {
		common.ApiErrorMsg(c, "Passkey 登录状态异常")
		return
	}

	modelUser := userWrapper.ModelUser()
	if modelUser == nil {
		common.ApiErrorMsg(c, "Passkey 登录状态异常")
		return
	}

	if modelUser.Status != common.UserStatusEnabled {
		common.ApiErrorMsg(c, "该用户已被禁用")
		return
	}

	if err := model.UpdatePasskeyAssertionState(modelUser.Id, credential, time.Now(), sessionData.RelyingPartyID); err != nil {
		common.ApiError(c, err)
		return
	}

	setupLogin(modelUser, c)
}

func AdminResetPasskey(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiErrorMsg(c, "无效的用户 ID")
		return
	}

	user := &model.User{Id: id}
	if err := user.FillUserById(); err != nil {
		common.ApiError(c, err)
		return
	}
	myRole := c.GetInt("role")
	if !canManageTargetRole(myRole, user.Role) {
		common.ApiErrorMsg(c, "no permission")
		return
	}

	if _, err := model.GetPasskeyByUserID(user.Id); err != nil {
		if errors.Is(err, model.ErrPasskeyNotFound) {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "该用户尚未绑定 Passkey",
			})
			return
		}
		common.ApiError(c, err)
		return
	}

	if err := model.DeletePasskeyByUserID(user.Id); err != nil {
		common.ApiError(c, err)
		return
	}

	recordManageAuditFor(c, user.Id, "user.reset_passkey", map[string]interface{}{
		"username": user.Username,
		"id":       user.Id,
	})
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Passkey 已重置",
	})
}

func PasskeyVerifyBegin(c *gin.Context) {
	sessionHash, err := service.SecuritySessionHash(c)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	challenge := c.GetHeader("X-Security-Challenge")
	if _, err := model.GetSecurityProof(challenge, c.GetInt("id"), sessionHash); err != nil {
		common.ApiError(c, err)
		return
	}
	session := sessions.Default(c)
	session.Set("passkey_operation_challenge", challenge)
	if err := session.Save(); err != nil {
		common.ApiError(c, err)
		return
	}
	if !system_setting.PasskeySettingsSnapshot().Enabled {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "管理员未启用 Passkey 登录",
		})
		return
	}

	user, err := getSessionUser(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": common.SanitizePublicError(err.Error()),
		})
		return
	}

	credential, err := model.GetPasskeyByUserID(user.Id)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "该用户尚未绑定 Passkey",
		})
		return
	}

	credentialRPID := ""
	if credential.RPID != nil {
		credentialRPID = *credential.RPID
	}
	wa, rpIDs, err := passkeysvc.BuildLoginWebAuthn(c.Request, c.Query("rp_id"), credentialRPID)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	waUser := passkeysvc.NewWebAuthnUser(user, credential)
	assertion, sessionData, err := wa.BeginLogin(waUser, webauthnlib.WithUserVerification(protocol.VerificationRequired))
	if err != nil {
		common.ApiError(c, err)
		return
	}

	if err := passkeysvc.SaveSessionData(c, passkeysvc.VerifySessionKey, sessionData); err != nil {
		common.ApiError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data": gin.H{
			"options": assertion, "rp_ids": rpIDs,
		},
	})
}

func PasskeyVerifyFinish(c *gin.Context) {
	if !system_setting.PasskeySettingsSnapshot().Enabled {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "管理员未启用 Passkey 登录",
		})
		return
	}

	user, err := getSessionUser(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": common.SanitizePublicError(err.Error()),
		})
		return
	}

	credential, err := model.GetPasskeyByUserID(user.Id)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "该用户尚未绑定 Passkey",
		})
		return
	}

	sessionData, err := passkeysvc.PopSessionData(c, passkeysvc.VerifySessionKey)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	wa, err := passkeysvc.BuildWebAuthnForRPID(c.Request, sessionData.RelyingPartyID)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if sessionData.UserVerification != protocol.VerificationRequired {
		common.ApiError(c, model.ErrSecurityProof)
		return
	}

	if credential.RPID != nil && *credential.RPID != "" && *credential.RPID != sessionData.RelyingPartyID {
		common.ApiError(c, model.ErrSecurityProof)
		return
	}

	waUser := passkeysvc.NewWebAuthnUser(user, credential)
	validatedCredential, err := wa.FinishLogin(waUser, *sessionData, c.Request)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	if err := model.UpdatePasskeyAssertionState(user.Id, validatedCredential, time.Now(), sessionData.RelyingPartyID); err != nil {
		common.ApiError(c, err)
		return
	}

	session := sessions.Default(c)
	challenge := c.GetString("passkey_operation_challenge")
	session.Delete("passkey_operation_challenge")
	session.Delete(SecureVerificationSessionKey)
	session.Delete(secureVerificationMethodSessionKey)
	if err := session.Save(); err != nil {
		common.ApiError(c, fmt.Errorf("保存验证状态失败: %v", err))
		return
	}
	sessionHash, err := service.SecuritySessionHash(c)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if err := model.VerifySecurityProof(challenge, user.Id, sessionHash, "passkey"); err != nil {
		common.ApiError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Passkey 验证成功",
	})
}

func getSessionUser(c *gin.Context) (*model.User, error) { return service.DashboardUser(c) }

func requirePasskeyRegistrationVerification(c *gin.Context, userID int) bool {
	twoFA, err := model.GetTwoFAByUserId(userID)
	if err != nil {
		common.ApiError(c, err)
		return false
	}
	if twoFA == nil || !twoFA.IsEnabled {
		return true
	}
	return requireSecureVerificationMethod(c, secureVerificationMethod2FA)
}

func requirePasskeyDeleteVerification(c *gin.Context, userID int) bool {
	twoFA, err := model.GetTwoFAByUserId(userID)
	if err != nil {
		common.ApiError(c, err)
		return false
	}
	if twoFA != nil && twoFA.IsEnabled {
		return requireSecureVerificationMethod(c, secureVerificationMethod2FA)
	}

	_, err = model.GetPasskeyByUserID(userID)
	if err != nil {
		if errors.Is(err, model.ErrPasskeyNotFound) {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "该用户尚未绑定 Passkey",
			})
			return false
		}
		common.ApiError(c, err)
		return false
	}

	return requireSecureVerificationMethod(c, secureVerificationMethodPasskey)
}

func requireSecureVerificationMethod(c *gin.Context, method string) bool {
	session := sessions.Default(c)
	verifiedAt, ok := session.Get(SecureVerificationSessionKey).(int64)
	if !ok || time.Now().Unix()-verifiedAt >= SecureVerificationTimeout {
		session.Delete(SecureVerificationSessionKey)
		session.Delete(secureVerificationMethodSessionKey)
		_ = session.Save()
		common.ApiErrorMsg(c, "请先完成安全验证")
		return false
	}

	if verifiedMethod, ok := session.Get(secureVerificationMethodSessionKey).(string); !ok || verifiedMethod != method {
		common.ApiErrorMsg(c, "请先完成对应的安全验证")
		return false
	}

	return true
}
