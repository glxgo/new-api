package controller

import (
	"fmt"
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
)

// 提现配置(后续可改 option 可配, 先用常量)
const (
	withdrawFeeRate       = 0.026      // 手续费 2.6%(由支付平台收取)
	withdrawFreezeSeconds = 7 * 24 * 3600 // 充值冻结期 7 天(本金提现: 7 天内成功充值不可提)
	withdrawNotifyEmail   = "246907434@qq.com"
)

type requestWithdrawReq struct {
	Type          int    `json:"type"`           // 1 本金 2 分红
	Amount        int    `json:"amount"`         // 提现金额(quota 单位)
	AlipayName    string `json:"alipay_name"`    // 本金提现必填
	AlipayAccount string `json:"alipay_account"` // 本金提现必填
	WechatQrcode  string `json:"wechat_qrcode"` // base64, 备用
}

// RequestWithdraw 用户提现申请(本金/分红统一入口)。
// 本金(Type=1): 普通用户, 必填支付宝姓名/账户(+微信码备用), 校验充值冻结期(7 天内成功充值不可提)。
// 分红(Type=2): 管理员/超管, 不填收款信息(超管线下联系打款)。申请后冻结余额, 进超管审核队列。
func RequestWithdraw(c *gin.Context) {
	var req requestWithdrawReq
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiErrorMsg(c, "参数错误")
		return
	}
	if req.Amount <= 0 {
		common.ApiErrorMsg(c, "提现金额必须大于 0")
		return
	}
	userId := c.GetInt("id")
	username := c.GetString("username")
	role := c.GetInt("role")

	user, err := model.GetUserById(userId, false)
	if err != nil || user == nil {
		common.ApiErrorMsg(c, "用户不存在")
		return
	}

	w := model.Withdraw{
		UserId:       userId,
		Type:         req.Type,
		Amount:       req.Amount,
		Fee:          model.CalcWithdrawFee(req.Amount, withdrawFeeRate),
		Status:       model.WithdrawStatusPending,
	}
	if w.Fee >= req.Amount {
		common.ApiErrorMsg(c, "提现金额不足以抵扣手续费")
		return
	}
	w.ActualAmount = req.Amount - w.Fee

	if req.Type == model.WithdrawTypePrincipal {
		// 普通用户本金提现, 必填收款信息
		if req.AlipayName == "" || req.AlipayAccount == "" {
			common.ApiErrorMsg(c, "本金提现必填支付宝姓名和账户")
			return
		}
		w.AlipayName, w.AlipayAccount, w.WechatQrcode = req.AlipayName, req.AlipayAccount, req.WechatQrcode
		// 充值冻结期校验: 7 天内成功充值的额度不可提
		available, err := getPrincipalWithdrawable(userId, user.Quota)
		if err != nil {
			common.ApiError(c, err)
			return
		}
		if req.Amount > available {
			common.ApiErrorMsg(c, fmt.Sprintf("可提现本金不足(7 天内充值冻结), 可提 %s", logger.FormatQuota(available)))
			return
		}
		if err := model.FreezeUserBalance(userId, req.Type, req.Amount); err != nil {
			common.ApiError(c, err)
			return
		}
	} else if req.Type == model.WithdrawTypeDividend {
		// 分红提现(管理员/超管), 不填收款信息
		if role < common.RoleAdminUser {
			common.ApiErrorMsg(c, "仅管理员/超管可提现分红")
			return
		}
		if req.Amount > user.DividendBalance {
			common.ApiErrorMsg(c, "分红余额不足")
			return
		}
		if err := model.FreezeUserBalance(userId, req.Type, req.Amount); err != nil {
			common.ApiError(c, err)
			return
		}
	} else {
		common.ApiErrorMsg(c, "无效的提现类型")
		return
	}

	if err := model.CreateWithdraw(&w); err != nil {
		// 建记录失败, 回滚冻结
		_ = model.RejectUserWithdraw(userId, req.Type, req.Amount)
		common.ApiError(c, err)
		return
	}

	go notifyWithdrawRequest(username, req.Type, req.Amount, w.ActualAmount, user.Email)
	logger.LogInfo(c.Request.Context(), fmt.Sprintf("user %d request withdraw type=%d amount=%d fee=%d actual=%d",
		userId, req.Type, req.Amount, w.Fee, w.ActualAmount))
	common.ApiSuccess(c, w)
}

// getPrincipalWithdrawable 本金可提额度 = 当前本金余额 - 7 天内成功充值总额(下限 0)。
// 防「充值即提」: 新充值的钱需沉淀 7 天后才可提。
func getPrincipalWithdrawable(userId int, quota int) (int, error) {
	cutoff := common.GetTimestamp() - int64(withdrawFreezeSeconds)
	var recentSum int64
	err := model.DB.Model(&model.TopUp{}).
		Where("user_id = ? AND status = ? AND complete_time >= ?", userId, common.TopUpStatusSuccess, cutoff).
		Select("COALESCE(SUM(amount), 0)").Scan(&recentSum).Error
	if err != nil {
		return 0, err
	}
	available := quota - int(recentSum)
	if available < 0 {
		available = 0
	}
	return available, nil
}

type withdrawActionReq struct {
	Id     int    `json:"id"`
	Remark string `json:"remark"`
}

// ApproveWithdraw 超管审核通过(线下已打款 → 系统清冻结)。
func ApproveWithdraw(c *gin.Context) {
	reviewWithdraw(c, model.WithdrawStatusApproved)
}

// RejectWithdraw 超管审核拒绝(解冻退回可用余额)。
func RejectWithdraw(c *gin.Context) {
	reviewWithdraw(c, model.WithdrawStatusRejected)
}

func reviewWithdraw(c *gin.Context, status int) {
	var req withdrawActionReq
	if err := c.ShouldBindJSON(&req); err != nil || req.Id <= 0 {
		common.ApiErrorMsg(c, "参数错误")
		return
	}
	w, err := model.GetWithdrawById(req.Id)
	if err != nil || w == nil {
		common.ApiErrorMsg(c, "提现记录不存在")
		return
	}
	if w.Status != model.WithdrawStatusPending {
		common.ApiErrorMsg(c, "该提现申请已处理")
		return
	}
	handlerId := c.GetInt("id")
	handlerName := c.GetString("username")
	// 原子更新状态(WHERE Pending 防并发重复审核)
	if err := model.FinishWithdraw(w.Id, status, handlerId, handlerName, req.Remark); err != nil {
		common.ApiError(c, err)
		return
	}
	// 资金处理: 通过=清冻结(钱已出系统), 拒绝=退回可用余额
	if status == model.WithdrawStatusApproved {
		if err := model.ApproveUserWithdraw(w.UserId, w.Type, w.Amount); err != nil {
			common.SysError(fmt.Sprintf("approve withdraw %d clear frozen failed: %v", w.Id, err))
		}
	} else {
		if err := model.RejectUserWithdraw(w.UserId, w.Type, w.Amount); err != nil {
			common.SysError(fmt.Sprintf("reject withdraw %d refund failed: %v", w.Id, err))
		}
	}
	logger.LogInfo(c.Request.Context(), fmt.Sprintf("withdraw %d %s by admin %d(%s)",
		w.Id, withdrawStatusStr(status), handlerId, handlerName))
	common.ApiSuccess(c, nil)
}

// GetUserWithdraws 用户查自己的提现记录。
func GetUserWithdraws(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	userId := c.GetInt("id")
	ws, total, err := model.GetUserWithdraws(userId, page, pageSize)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{"data": ws, "total": total})
}

// GetAllWithdraws 超管查看提现审核队列。
func GetAllWithdraws(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	status, _ := strconv.Atoi(c.DefaultQuery("status", "-1"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	ws, total, err := model.GetAllWithdraws(status, page, pageSize)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{"data": ws, "total": total})
}

func notifyWithdrawRequest(username string, wtype, amount, actual int, userEmail string) {
	typeStr := "本金"
	if wtype == model.WithdrawTypeDividend {
		typeStr = "分红"
	}
	subject := fmt.Sprintf("[%s] 新提现申请 - %s %s", common.SystemName, typeStr, username)
	content := fmt.Sprintf("<p>用户 <b>%s</b> 申请提现%s, 金额 %s, 扣除手续费后实际到账 %s, 请及时审核。</p>",
		username, typeStr, logger.FormatQuota(amount), logger.FormatQuota(actual))
	to := withdrawNotifyEmail
	if userEmail != "" {
		to = withdrawNotifyEmail + ";" + userEmail
	}
	if err := common.SendEmail(subject, to, content); err != nil {
		common.SysError("send withdraw notify email failed: " + err.Error())
	}
}

func withdrawStatusStr(status int) string {
	switch status {
	case model.WithdrawStatusApproved:
		return "approved"
	case model.WithdrawStatusRejected:
		return "rejected"
	default:
		return "pending"
	}
}
