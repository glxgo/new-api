package model

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"

	"github.com/samber/hot"
	"golang.org/x/sync/singleflight"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func applyExplicitLogTextFilter(tx *gorm.DB, column string, value string) (*gorm.DB, error) {
	if value == "" {
		return tx, nil
	}
	if strings.Contains(value, "%") {
		pattern, err := sanitizeLikePattern(value)
		if err != nil {
			return nil, err
		}
		return tx.Where(column+" LIKE ? ESCAPE '!'", pattern), nil
	}
	return tx.Where(column+" = ?", value), nil
}

type Log struct {
	ClientKey            string `json:"client_key,omitempty" gorm:"type:varchar(96);default:''"`
	ClientFamily         string `json:"client_family,omitempty" gorm:"type:varchar(40);default:''"`
	Id                   int    `json:"id" gorm:"index:idx_created_at_id,priority:2;index:idx_user_id_id,priority:2"`
	UserId               int    `json:"user_id" gorm:"index;index:idx_user_id_id,priority:1;index:idx_user_type_created_at,priority:1"`
	CreatedAt            int64  `json:"created_at" gorm:"bigint;index:idx_created_at_id,priority:1;index:idx_created_at_type;index:idx_type_created_at,priority:2;index:idx_user_type_created_at,priority:3"`
	Type                 int    `json:"type" gorm:"index:idx_created_at_type;index:idx_type_created_at,priority:1;index:idx_user_type_created_at,priority:2"`
	Content              string `json:"content"`
	Username             string `json:"username" gorm:"index;index:index_username_model_name,priority:2;default:''"`
	TokenName            string `json:"token_name" gorm:"index;default:''"`
	ModelName            string `json:"model_name" gorm:"index;index:index_username_model_name,priority:1;default:''"`
	Quota                int    `json:"quota" gorm:"default:0"`
	PreDiscountQuota     int    `json:"pre_discount_quota" gorm:"default:0;column:pre_discount_quota"`
	PromptTokens         int    `json:"prompt_tokens" gorm:"default:0"`
	CacheTokens          int    `json:"cache_tokens" gorm:"default:0"` // prompt cache 命中 token(个人缓存率用)
	CompletionTokens     int    `json:"completion_tokens" gorm:"default:0"`
	UseTime              int    `json:"use_time" gorm:"default:0"`
	IsStream             bool   `json:"is_stream"`
	ChannelId            int    `json:"channel" gorm:"index"`
	ChannelName          string `json:"channel_name" gorm:"->"`
	TokenId              int    `json:"token_id" gorm:"default:0;index"`
	Group                string `json:"group" gorm:"index"`
	Ip                   string `json:"ip" gorm:"index;default:''"`
	RequestId            string `json:"request_id,omitempty" gorm:"type:varchar(64);index:idx_logs_request_id;default:''"`
	UpstreamRequestId    string `json:"upstream_request_id,omitempty" gorm:"type:varchar(128);index:idx_logs_upstream_request_id;default:''"`
	UserConcurrency      int    `json:"user_concurrency" gorm:"default:0;column:user_concurrency"`
	UserConcurrencyLimit int    `json:"user_concurrency_limit" gorm:"default:0;column:user_concurrency_limit"`
	UserRPM              int    `json:"user_rpm" gorm:"default:0;column:user_rpm"`
	UserRPMLimit         int    `json:"user_rpm_limit" gorm:"default:0;column:user_rpm_limit"`
	Other                string `json:"other"`
	// 历史利润结算兼容字段；新充值分润不读取这些字段。
	Cost                       int     `json:"cost" gorm:"default:0"`                                   // 该请求成本(quota单位,平台付上游)
	PaidQuota                  int     `json:"paid_quota" gorm:"default:0;column:paid_quota"`           // 本金分摊
	PaidGiftQuota              int     `json:"paid_gift_quota" gorm:"default:0;column:paid_gift_quota"` // 赠金分摊
	AffAdminIdSnap             int     `json:"aff_admin_id_snap" gorm:"default:0;column:aff_admin_id_snap;index:idx_logs_settle"`
	InviterIdSnap              int     `json:"inviter_id_snap" gorm:"default:0;column:inviter_id_snap"`
	Inviter2IdSnap             int     `json:"inviter2_id_snap" gorm:"default:0;column:inviter2_id_snap"`
	Settled                    bool    `json:"settled" gorm:"default:false;column:settled;index:idx_logs_settle"`
	SettleBatchId              string  `json:"settle_batch_id" gorm:"type:varchar(40);column:settle_batch_id;index:idx_logs_settle"`
	ProfitReconciliationStatus string  `json:"profit_reconciliation_status" gorm:"type:varchar(32);not null;default:'';index"`
	ProfitReconciliationReason string  `json:"profit_reconciliation_reason" gorm:"type:varchar(255);not null;default:''"`
	BillingSource              string  `json:"billing_source" gorm:"type:varchar(32);default:'';column:billing_source"` // wallet/subscription/virtual_membership，套餐类消费不计分润(购买时已分润)
	SubscriptionId             int     `json:"subscription_id" gorm:"default:0;column:subscription_id;index"`
	CostRuleVersion            int     `json:"cost_rule_version" gorm:"default:1;column:cost_rule_version;index"`
	ChannelCostRatioPPM        *int64  `json:"channel_cost_ratio_ppm" gorm:"column:channel_cost_ratio_ppm;default:null"`
	FinancialEventKey          *string `json:"-" gorm:"type:varchar(160);uniqueIndex"`
	// BalanceAfter 操作后余额快照(quota 单位, 本金+赠金)，仅供财务流水展示，
	// 不参与计费/扣费。扣费走异步缓存/批量更新，此值为 best-effort 快照。
	BalanceAfter *int64 `json:"balance_after" gorm:"column:balance_after"`
}

// don't use iota, avoid change log type value
const (
	LogTypeUnknown = 0
	LogTypeTopup   = 1
	LogTypeConsume = 2
	LogTypeManage  = 3
	LogTypeSystem  = 4
	LogTypeError   = 5
	LogTypeRefund  = 6
	LogTypeLogin   = 7
)

func formatUserLogs(logs []*Log, startIdx int) {
	for i := range logs {
		logs[i].ChannelName = ""
		var otherMap map[string]interface{}
		otherMap, _ = common.StrToMap(logs[i].Other)
		if otherMap != nil {
			// Remove admin-only debug fields.
			delete(otherMap, "admin_info")
			// Remove operation-audit details (operator/route info), admin-only.
			delete(otherMap, "audit_info")
			// delete(otherMap, "reject_reason")
			delete(otherMap, "stream_status")
		}
		logs[i].Other = common.MapToJsonStr(otherMap)
		logs[i].Id = startIdx + i + 1
	}
}

func GetLogByTokenId(tokenId int) (logs []*Log, err error) {
	return GetLogByTokenIdWithContext(context.Background(), tokenId)
}

func GetLogByTokenIdWithContext(ctx context.Context, tokenId int) (logs []*Log, err error) {
	err = LOG_DB.WithContext(ctx).Model(&Log{}).Where("token_id = ?", tokenId).Order("id desc").Limit(common.MaxRecentItems).Find(&logs).Error
	formatUserLogs(logs, 0)
	return logs, err
}

func RecordLog(userId int, logType int, content string) {
	if logType == LogTypeConsume && !common.LogConsumeEnabled {
		return
	}
	username, _ := GetUsernameById(userId, false)
	log := &Log{
		UserId:    userId,
		Username:  username,
		CreatedAt: common.GetTimestamp(),
		Type:      logType,
		Content:   content,
	}
	err := LOG_DB.Create(log).Error
	if err != nil {
		common.SysLog("failed to record log: " + err.Error())
	}
}

// EnsureTopupLog records one financial-flow row for a paid event. The nullable
// unique key keeps legacy logs compatible while making webhook replay repair a
// missing log without duplicating it.
func EnsureTopupLog(userId int, eventKey, content string) error {
	return ensureTopupLog(userId, eventKey, content, "", "", "")
}

// EnsureTopupPaymentLog is the provider-callback variant of EnsureTopupLog.
// It preserves the payment audit metadata while the financial event key makes
// webhook replay and manual-completion reconciliation idempotent.
func EnsureTopupPaymentLog(userId int, eventKey, content, callerIp, paymentMethod, callbackPaymentMethod string) error {
	return ensureTopupLog(userId, eventKey, content, callerIp, paymentMethod, callbackPaymentMethod)
}

func ensureTopupLog(userId int, eventKey, content, callerIp, paymentMethod, callbackPaymentMethod string) error {
	if LOG_DB == nil || userId <= 0 || strings.TrimSpace(eventKey) == "" {
		return errors.New("invalid topup log event")
	}
	username, _ := GetUsernameById(userId, false)
	key := "topup:" + strings.TrimSpace(eventKey)
	log := &Log{
		UserId: userId, Username: username, CreatedAt: common.GetTimestamp(),
		Type: LogTypeTopup, Content: content, Ip: callerIp, FinancialEventKey: &key,
	}
	if callerIp != "" || paymentMethod != "" || callbackPaymentMethod != "" {
		log.Other = common.MapToJsonStr(map[string]interface{}{
			"admin_info": map[string]interface{}{
				"server_ip": common.GetIp(), "node_name": common.NodeName,
				"caller_ip": callerIp, "payment_method": paymentMethod,
				"callback_payment_method": callbackPaymentMethod, "version": common.Version,
			},
		})
	}
	return LOG_DB.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "financial_event_key"}}, DoNothing: true}).Create(log).Error
}

// RecordLogWithAdminInfo 记录操作日志，并将管理员相关信息存入 Other.admin_info，
func RecordLogWithAdminInfo(userId int, logType int, content string, adminInfo map[string]interface{}) {
	if logType == LogTypeConsume && !common.LogConsumeEnabled {
		return
	}
	username, _ := GetUsernameById(userId, false)
	log := &Log{
		UserId:    userId,
		Username:  username,
		CreatedAt: common.GetTimestamp(),
		Type:      logType,
		Content:   content,
	}
	if len(adminInfo) > 0 {
		other := map[string]interface{}{
			"admin_info": adminInfo,
		}
		log.Other = common.MapToJsonStr(other)
	}
	if err := LOG_DB.Create(log).Error; err != nil {
		common.SysLog("failed to record log: " + err.Error())
	}
}

// buildOpField 构建语言无关的操作描述（写入 Other.op）。
// 前端依据 action(稳定操作标识) + params(结构化参数) 在渲染期用 i18n 本地化展示，
// 因此不在数据库中存储自然语言句子。
func buildOpField(action string, params map[string]interface{}) map[string]interface{} {
	op := map[string]interface{}{
		"action": action,
	}
	if len(params) > 0 {
		op["params"] = params
	}
	return op
}

// RecordLoginLog 记录用户登录成功的审计日志（type=LogTypeLogin）。
// username 由调用方传入（登录流程已持有用户对象），避免额外的数据库查询。
// content 为英文兜底文本（用于导出/经典前端）；action+params 供前端本地化渲染。
// extra 可携带 login_method、user_agent 等附加信息（普通用户可见）。
func RecordLoginLog(userId int, username string, content string, ip string, action string, params map[string]interface{}, extra map[string]interface{}) {
	other := map[string]interface{}{}
	for k, v := range extra {
		other[k] = v
	}
	other["op"] = buildOpField(action, params)
	log := &Log{
		UserId:    userId,
		Username:  username,
		CreatedAt: common.GetTimestamp(),
		Type:      LogTypeLogin,
		Content:   content,
		Ip:        ip,
		Other:     common.MapToJsonStr(other),
	}
	if err := LOG_DB.Create(log).Error; err != nil {
		common.SysLog("failed to record login log: " + err.Error())
	}
}

// RecordOperationAuditLog 记录管理/高危操作审计日志（type=LogTypeManage）。
// logUserId 为日志归属者（面向用户的操作如额度调整归属目标用户，资源类操作如渠道/系统设置归属操作者），
// username 内部按 logUserId 查询。content 为英文兜底文本（导出/经典前端用）。
// action+params 写入 Other.op，供前端本地化渲染（普通用户可见，不含敏感信息）。
// adminInfo 存放操作者身份（写入 Other.admin_info，普通用户查询时剥离）；
// auditInfo 存放路由/方法/结果等中间件兜底信息（写入 Other.audit_info，普通用户查询时剥离）。
func RecordOperationAuditLog(logUserId int, content string, ip string, action string, params map[string]interface{}, adminInfo map[string]interface{}, auditInfo map[string]interface{}) {
	username, _ := GetUsernameById(logUserId, false)
	other := map[string]interface{}{
		"op": buildOpField(action, params),
	}
	if len(adminInfo) > 0 {
		other["admin_info"] = adminInfo
	}
	if len(auditInfo) > 0 {
		other["audit_info"] = auditInfo
	}
	log := &Log{
		UserId:    logUserId,
		Username:  username,
		CreatedAt: common.GetTimestamp(),
		Type:      LogTypeManage,
		Content:   content,
		Ip:        ip,
		Other:     common.MapToJsonStr(other),
	}
	if err := LOG_DB.Create(log).Error; err != nil {
		common.SysLog("failed to record operation audit log: " + err.Error())
	}
}

func RecordTopupLog(userId int, content string, callerIp string, paymentMethod string, callbackPaymentMethod string) {
	username, _ := GetUsernameById(userId, false)
	adminInfo := map[string]interface{}{
		"server_ip":               common.GetIp(),
		"node_name":               common.NodeName,
		"caller_ip":               callerIp,
		"payment_method":          paymentMethod,
		"callback_payment_method": callbackPaymentMethod,
		"version":                 common.Version,
	}
	other := map[string]interface{}{
		"admin_info": adminInfo,
	}
	log := &Log{
		UserId:    userId,
		Username:  username,
		CreatedAt: common.GetTimestamp(),
		Type:      LogTypeTopup,
		Content:   content,
		Ip:        callerIp,
		Other:     common.MapToJsonStr(other),
	}
	err := LOG_DB.Create(log).Error
	if err != nil {
		common.SysLog("failed to record topup log: " + err.Error())
	}
}

func RecordErrorLog(c *gin.Context, userId int, channelId int, modelName string, tokenName string, content string, tokenId int, useTimeSeconds int,
	isStream bool, group string, other map[string]interface{}) {
	logger.LogInfo(c, fmt.Sprintf("record error log: userId=%d, channelId=%d, modelName=%s, tokenName=%s, content=%s", userId, channelId, modelName, tokenName, common.LocalLogPreview(content)))
	username := c.GetString("username")
	requestId := c.GetString(common.RequestIdKey)
	upstreamRequestId := c.GetString(common.UpstreamRequestIdKey)
	otherStr := common.MapToJsonStr(WithRequestClientLog(c, other))
	// 判断是否需要记录 IP
	needRecordIp := false
	if settingMap, err := GetUserSetting(userId, false); err == nil {
		if settingMap.RecordIpLog {
			needRecordIp = true
		}
	}
	log := &Log{
		UserId:           userId,
		Username:         username,
		CreatedAt:        common.GetTimestamp(),
		Type:             LogTypeError,
		Content:          content,
		PromptTokens:     0,
		CompletionTokens: 0,
		TokenName:        tokenName,
		ModelName:        modelName,
		Quota:            0,
		ChannelId:        channelId,
		TokenId:          tokenId,
		UseTime:          useTimeSeconds,
		IsStream:         isStream,
		Group:            group,
		Ip: func() string {
			if needRecordIp {
				return c.ClientIP()
			}
			return ""
		}(),
		RequestId:            requestId,
		UpstreamRequestId:    upstreamRequestId,
		UserConcurrency:      common.GetContextKeyInt(c, constant.ContextKeyUserConcurrency),
		UserConcurrencyLimit: common.GetContextKeyInt(c, constant.ContextKeyUserConcurrencyLimit),
		UserRPM:              common.GetContextKeyInt(c, constant.ContextKeyUserRPM),
		UserRPMLimit:         common.GetContextKeyInt(c, constant.ContextKeyUserRPMLimit),
		Other:                otherStr,
	}
	err := LOG_DB.Create(log).Error
	if err != nil {
		logger.LogError(c, "failed to record log: "+err.Error())
	}
}

type RecordConsumeLogParams struct {
	ChannelId           int                    `json:"channel_id"`
	PromptTokens        int                    `json:"prompt_tokens"`
	CacheTokens         int                    `json:"cache_tokens"` // prompt cache 命中 token
	CompletionTokens    int                    `json:"completion_tokens"`
	ModelName           string                 `json:"model_name"`
	TokenName           string                 `json:"token_name"`
	Quota               int                    `json:"quota"`
	Cost                int                    `json:"cost"`              // 平台成本观测(quota 单位)
	PaidQuota           int                    `json:"paid_quota"`        // 本金分摊
	PaidGiftQuota       int                    `json:"paid_gift_quota"`   // 赠金分摊
	AffAdminIdSnap      int                    `json:"aff_admin_id_snap"` // 树顶管理员快照
	InviterIdSnap       int                    `json:"inviter_id_snap"`   // 直接上级快照
	Inviter2IdSnap      int                    `json:"inviter2_id_snap"`  // 间接上级快照
	Content             string                 `json:"content"`
	TokenId             int                    `json:"token_id"`
	UseTimeSeconds      int                    `json:"use_time_seconds"`
	IsStream            bool                   `json:"is_stream"`
	Group               string                 `json:"group"`
	Other               map[string]interface{} `json:"other"`
	BillingSource       string                 `json:"billing_source"` // wallet/subscription/virtual_membership
	SubscriptionId      int                    `json:"subscription_id"`
	CostRuleVersion     int                    `json:"cost_rule_version"`
	ChannelCostRatioPPM *int64                 `json:"channel_cost_ratio_ppm"`
}

func RecordConsumeLog(c *gin.Context, userId int, params RecordConsumeLogParams) {
	if !common.LogConsumeEnabled {
		return
	}
	// Keep the hot-path INFO record bounded. Full billing/audit fields remain in
	// the structured logs table; serializing the entire request payload here
	// needlessly adds JSON/GC work for every successful relay.
	logger.LogInfo(c, fmt.Sprintf("record consume log: userId=%d model=%s token=%d quota=%d", userId, params.ModelName, params.TokenId, params.Quota))
	username := c.GetString("username")
	requestId := c.GetString(common.RequestIdKey)
	upstreamRequestId := c.GetString(common.UpstreamRequestIdKey)
	jsonStartedAt := time.Now()
	otherStr := common.MapToJsonStr(WithRequestClientLog(c, params.Other))
	jsonDuration := time.Since(jsonStartedAt)
	// 判断是否需要记录 IP
	needRecordIp := false
	if settingMap, err := GetUserSetting(userId, false); err == nil {
		if settingMap.RecordIpLog {
			needRecordIp = true
		}
	}
	log := &Log{
		UserId:           userId,
		Username:         username,
		CreatedAt:        common.GetTimestamp(),
		Type:             LogTypeConsume,
		Content:          params.Content,
		PromptTokens:     params.PromptTokens,
		CacheTokens:      params.CacheTokens,
		CompletionTokens: params.CompletionTokens,
		TokenName:        params.TokenName,
		ModelName:        params.ModelName,
		Quota:            params.Quota,
		PreDiscountQuota: resolvePreDiscountQuota(params.Quota, params.Other),
		Cost:             params.Cost,
		PaidQuota:        params.PaidQuota,
		PaidGiftQuota:    params.PaidGiftQuota,
		AffAdminIdSnap:   params.AffAdminIdSnap,
		InviterIdSnap:    params.InviterIdSnap,
		Inviter2IdSnap:   params.Inviter2IdSnap,
		ChannelId:        params.ChannelId,
		TokenId:          params.TokenId,
		UseTime:          params.UseTimeSeconds,
		IsStream:         params.IsStream,
		Group:            params.Group,
		Ip: func() string {
			if needRecordIp {
				return c.ClientIP()
			}
			return ""
		}(),
		RequestId:            requestId,
		UpstreamRequestId:    upstreamRequestId,
		UserConcurrency:      common.GetContextKeyInt(c, constant.ContextKeyUserConcurrency),
		UserConcurrencyLimit: common.GetContextKeyInt(c, constant.ContextKeyUserConcurrencyLimit),
		UserRPM:              common.GetContextKeyInt(c, constant.ContextKeyUserRPM),
		UserRPMLimit:         common.GetContextKeyInt(c, constant.ContextKeyUserRPMLimit),
		Other:                otherStr,
		BillingSource:        params.BillingSource,
		SubscriptionId:       params.SubscriptionId,
		CostRuleVersion:      params.CostRuleVersion,
		ChannelCostRatioPPM:  params.ChannelCostRatioPPM,
		// Consumption logs are no longer a profit-settlement queue. Mark new
		// rows complete at creation so only pre-cutover rows remain visible in
		// the legacy reconciliation audit.
		Settled:       true,
		SettleBatchId: rechargeCommissionLogBatchV1,
	}
	// 强制从主库单次读取两池余额，避免扣费后的 Redis 异步更新尚未完成时
	// 把扣费前余额写入财务流水。
	balanceStartedAt := time.Now()
	if balanceAfter, balanceErr := GetUserTotalQuotaFromDB(userId); balanceErr == nil {
		log.BalanceAfter = &balanceAfter
	}
	balanceDuration := time.Since(balanceStartedAt)
	logStartedAt := time.Now()
	err := LOG_DB.Create(log).Error
	logWriteDuration := time.Since(logStartedAt)
	common.RecordConsumeLogMetrics(jsonDuration, balanceDuration, logWriteDuration, err != nil)
	if err != nil {
		logger.LogError(c, "failed to record log: "+err.Error())
	}
	if common.DataExportEnabled {
		requestContext := context.Background()
		if c != nil && c.Request != nil {
			requestContext = c.Request.Context()
		}
		if common.BackgroundCtxGo("report", requestContext, func() {
			LogQuotaData(userId, username, params.ModelName, params.Quota, common.GetTimestamp(), params.PromptTokens+params.CompletionTokens)
		}) {
			common.RecordQuotaDataEnqueued()
		} else {
			// Preserve dashboard data when the optional queue is saturated. The
			// fallback is bounded to one aggregation operation and never drops it.
			common.RecordQuotaDataQueueFull()
			common.RecordQuotaDataInline()
			LogQuotaData(userId, username, params.ModelName, params.Quota, common.GetTimestamp(), params.PromptTokens+params.CompletionTokens)
		}
	}
}

type RecordTaskBillingLogParams struct {
	UserId              int
	LogType             int
	Content             string
	ChannelId           int
	ModelName           string
	Quota               int
	Cost                int // 平台成本(quota 单位)
	PaidQuota           int // 本金分摊
	PaidGiftQuota       int // 赠金分摊
	AffAdminIdSnap      int // 树顶管理员快照
	InviterIdSnap       int // 直接上级快照
	Inviter2IdSnap      int // 间接上级快照
	TokenId             int
	Group               string
	Other               map[string]interface{}
	BillingSource       string // wallet/subscription/virtual_membership
	SubscriptionId      int
	CostRuleVersion     int
	ChannelCostRatioPPM *int64
}

func RecordTaskBillingLog(params RecordTaskBillingLogParams) {
	if params.LogType == LogTypeConsume && !common.LogConsumeEnabled {
		return
	}
	username, _ := GetUsernameById(params.UserId, false)
	tokenName := ""
	if params.TokenId > 0 {
		if token, err := GetTokenById(params.TokenId); err == nil {
			tokenName = token.Name
		}
	}
	log := &Log{
		UserId:              params.UserId,
		Username:            username,
		CreatedAt:           common.GetTimestamp(),
		Type:                params.LogType,
		Content:             params.Content,
		TokenName:           tokenName,
		ModelName:           params.ModelName,
		Quota:               params.Quota,
		PreDiscountQuota:    resolvePreDiscountQuota(params.Quota, params.Other),
		Cost:                params.Cost,
		PaidQuota:           params.PaidQuota,
		PaidGiftQuota:       params.PaidGiftQuota,
		AffAdminIdSnap:      params.AffAdminIdSnap,
		InviterIdSnap:       params.InviterIdSnap,
		Inviter2IdSnap:      params.Inviter2IdSnap,
		ChannelId:           params.ChannelId,
		TokenId:             params.TokenId,
		Group:               params.Group,
		Other:               common.MapToJsonStr(params.Other),
		BillingSource:       params.BillingSource,
		SubscriptionId:      params.SubscriptionId,
		CostRuleVersion:     params.CostRuleVersion,
		ChannelCostRatioPPM: params.ChannelCostRatioPPM,
	}
	if balanceAfter, balanceErr := GetUserTotalQuotaFromDB(params.UserId); balanceErr == nil {
		log.BalanceAfter = &balanceAfter
	}
	err := LOG_DB.Create(log).Error
	if err != nil {
		common.SysLog("failed to record task billing log: " + err.Error())
	}
}

type FinancialConsumeDaily struct {
	DayStart     int64  `json:"day_start"`
	Quota        int64  `json:"quota"`
	BalanceAfter *int64 `json:"balance_after"`
}

// GetUserFinancialConsumeDaily streams consume logs in chronological order,
// aggregates them by the server's local calendar day, and keeps the last
// operation's balance snapshot.
// Streaming avoids loading an unbounded 30-day request history into memory.
func GetUserFinancialConsumeDaily(userId int, startTimestamp int64, endTimestamp int64) ([]FinancialConsumeDaily, error) {
	return GetUserFinancialConsumeDailyWithContext(context.Background(), userId, startTimestamp, endTimestamp)
}

func GetUserFinancialConsumeDailyWithContext(ctx context.Context, userId int, startTimestamp int64, endTimestamp int64) ([]FinancialConsumeDaily, error) {
	if items, ready, err := readWalletConsumeDailyAggregatesWithContext(ctx, userId, startTimestamp, endTimestamp); ready && err == nil {
		return items, nil
	}
	return getUserFinancialConsumeDailyLegacyWithContext(ctx, userId, startTimestamp, endTimestamp)
}

// getUserFinancialConsumeDailyLegacyWithContext is the exact compatibility
// path. It remains available when the opt-in wallet projection is disabled,
// missing, incomplete, or unavailable, preserving the historical JSON and
// error behavior while the projection is rolled out gradually.
func getUserFinancialConsumeDailyLegacyWithContext(ctx context.Context, userId int, startTimestamp int64, endTimestamp int64) ([]FinancialConsumeDaily, error) {
	if LOG_DB == nil {
		return nil, errors.New("log database is unavailable")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if startTimestamp <= 0 || endTimestamp <= startTimestamp || userId <= 0 {
		return []FinancialConsumeDaily{}, nil
	}

	// Archive rows are daily segments.  Include a segment when its whole
	// interval is inside the requested range, or when the requested range
	// contains the complete local day.  A segment crossing a partial boundary
	// is deliberately omitted: its exact source rows no longer exist, and
	// counting the whole segment would overstate wallet consumption.
	startBucket := walletConsumeDayStart(startTimestamp)
	endBucket := walletConsumeDayStart(endTimestamp - 1)
	fullArchiveStart := startBucket
	if startTimestamp > startBucket {
		fullArchiveStart = walletConsumeDayEnd(startBucket)
	}
	// endBucket is the last day touched by the half-open query. Complete archive
	// days always stop at the start of the day containing endTimestamp: whether
	// the end is exactly midnight or lies inside that day, that end day is only a
	// partial edge and must not be counted as a whole bucket.
	endDayStart := walletConsumeDayStart(endTimestamp)
	fullArchiveEnd := endDayStart
	archiveScanEnd := walletConsumeDayEnd(endBucket)

	type financialConsumeRow struct {
		UserId       int           `gorm:"column:user_id"`
		DayStart     int64         `gorm:"column:day_start"`
		FirstLogAt   int64         `gorm:"column:first_log_at"`
		CreatedAt    int64         `gorm:"column:created_at"`
		SourceId     int64         `gorm:"column:source_id"`
		SourceKind   int           `gorm:"column:source_kind"`
		RequestCount int64         `gorm:"column:request_count"`
		Quota        int64         `gorm:"column:quota"`
		BalanceAfter sql.NullInt64 `gorm:"column:balance_after"`
	}

	archiveAvailable := LOG_DB.Migrator().HasTable(&UsageLogDailyAggregate{})
	var query *gorm.DB
	if archiveAvailable {
		archiveSourceID := walletConsumeArchiveSourceIDExpression(LOG_DB)
		query = LOG_DB.WithContext(ctx).Raw(fmt.Sprintf(`
			SELECT user_id, 0 AS day_start, created_at AS first_log_at, created_at, id AS source_id,
				0 AS source_kind, 1 AS request_count, quota, balance_after
			FROM logs
			WHERE user_id = ? AND type = ? AND created_at >= ? AND created_at < ?
			UNION ALL
			SELECT user_id, bucket_start AS day_start, first_log_at, last_log_at AS created_at,
				%s AS source_id,
				1 AS source_kind, request_count, quota, balance_after
			FROM usage_log_daily_aggregates
			WHERE user_id = ? AND type = ? AND bucket_start >= ? AND bucket_start < ?
				AND ((bucket_start >= ? AND bucket_start < ?)
					OR (first_log_at >= ? AND last_log_at < ?))
			ORDER BY source_kind ASC, user_id ASC, created_at ASC, source_id ASC`, archiveSourceID),
			userId, LogTypeConsume, startTimestamp, endTimestamp,
			userId, LogTypeConsume, startBucket, archiveScanEnd,
			fullArchiveStart, fullArchiveEnd, startTimestamp, endTimestamp,
		)
	} else {
		query = LOG_DB.WithContext(ctx).Raw(`
			SELECT user_id, 0 AS day_start, created_at AS first_log_at, created_at, id AS source_id,
				0 AS source_kind, 1 AS request_count, quota, balance_after
			FROM logs
			WHERE user_id = ? AND type = ? AND created_at >= ? AND created_at < ?
			ORDER BY created_at ASC, id ASC`,
			userId, LogTypeConsume, startTimestamp, endTimestamp,
		)
	}
	rows, err := query.Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// The UNION is ordered with raw rows first. If an archived segment's
	// timestamp interval intersects the remaining raw rows for the same
	// user/day, source ownership is ambiguous because archive rows do not retain
	// individual source IDs. Skip that segment to prevent duplicate quota. A
	// segment entirely before or after the raw interval remains countable.
	type rawSourceBounds struct {
		First int64
		Last  int64
	}
	rawBoundsByDay := make(map[int64]rawSourceBounds)
	daily := make(map[int64]*walletConsumeAggregateAccumulator)
	for rows.Next() {
		var row financialConsumeRow
		if err := LOG_DB.ScanRows(rows, &row); err != nil {
			return nil, err
		}
		dayStart := row.DayStart
		if dayStart == 0 {
			dayStart = walletConsumeDayStart(row.CreatedAt)
		}
		if row.SourceKind == 0 {
			bounds := rawBoundsByDay[dayStart]
			if bounds.First == 0 || row.FirstLogAt < bounds.First {
				bounds.First = row.FirstLogAt
			}
			if row.CreatedAt > bounds.Last {
				bounds.Last = row.CreatedAt
			}
			rawBoundsByDay[dayStart] = bounds
		} else if bounds, ok := rawBoundsByDay[dayStart]; ok && walletConsumeSourceIntervalsOverlap(row.FirstLogAt, row.CreatedAt, bounds.First, bounds.Last) {
			continue
		}
		acc := daily[dayStart]
		if acc == nil {
			acc = &walletConsumeAggregateAccumulator{UserId: userId}
			daily[dayStart] = acc
		}
		acc.add(walletConsumeAggregateRow{
			UserId: row.UserId, CreatedAt: row.CreatedAt, SourceId: row.SourceId,
			SourceKind: row.SourceKind, RequestCount: row.RequestCount,
			Quota: row.Quota, BalanceAfter: row.BalanceAfter,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	items := make([]FinancialConsumeDaily, 0, len(daily))
	for dayStart, acc := range daily {
		item := FinancialConsumeDaily{DayStart: dayStart, Quota: acc.Quota}
		if acc.BalanceAfter != nil {
			value := *acc.BalanceAfter
			item.BalanceAfter = &value
		}
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].DayStart > items[j].DayStart })
	return items, nil
}

func GetAllLogs(logType int, startTimestamp int64, endTimestamp int64, modelName string, username string, tokenName string, startIdx int, num int, channel int, group string, requestId string, upstreamRequestId string) (logs []*Log, total int64, err error) {
	return GetAllLogsWithContext(context.Background(), logType, startTimestamp, endTimestamp, modelName, username, tokenName, startIdx, num, channel, group, requestId, upstreamRequestId)
}

func GetAllLogsWithContext(ctx context.Context, logType int, startTimestamp int64, endTimestamp int64, modelName string, username string, tokenName string, startIdx int, num int, channel int, group string, requestId string, upstreamRequestId string) (logs []*Log, total int64, err error) {
	return GetAllLogsWithContextCursor(ctx, logType, startTimestamp, endTimestamp, modelName, username, tokenName, startIdx, num, channel, group, requestId, upstreamRequestId, "")
}

// LogPageMetadata opts a caller into count-free pagination. Cursors are
// constructed from database IDs before user-facing log IDs are anonymized.
type LogPageMetadata struct {
	HasMore    bool   `json:"has_more"`
	NextCursor string `json:"next_cursor"`
}

func finishLogPage(logs []*Log, size int, page *LogPageMetadata) []*Log {
	if page == nil {
		return logs
	}
	page.HasMore = len(logs) > size
	if page.HasMore {
		logs = logs[:size]
		last := logs[len(logs)-1]
		page.NextCursor = fmt.Sprintf("%d:%d", last.CreatedAt, last.Id)
	}
	return logs
}

// GetAllLogsWithContextCursor supports stable keyset pagination. The legacy
// page/offset arguments remain for old clients; when cursor is non-empty the
// (created_at,id) cursor takes precedence and no deep OFFSET is executed.
func GetAllLogsWithContextCursor(ctx context.Context, logType int, startTimestamp int64, endTimestamp int64, modelName string, username string, tokenName string, startIdx int, num int, channel int, group string, requestId string, upstreamRequestId string, cursor string, pages ...*LogPageMetadata) (logs []*Log, total int64, err error) {
	var page *LogPageMetadata
	if len(pages) > 0 {
		page = pages[0]
	}
	querySize := num
	if page != nil {
		querySize++
	}
	var tx *gorm.DB
	if logType == LogTypeUnknown {
		tx = LOG_DB.WithContext(ctx)
	} else {
		tx = LOG_DB.WithContext(ctx).Where("logs.type = ?", logType)
	}

	tx = applyClientFamilyFilter(tx, ClientLogFilter(ctx))
	if tx, err = applyExplicitLogTextFilter(tx, "logs.model_name", modelName); err != nil {
		return nil, 0, err
	}
	if tx, err = applyExplicitLogTextFilter(tx, "logs.username", username); err != nil {
		return nil, 0, err
	}
	if tokenName != "" {
		tx = tx.Where("logs.token_name = ?", tokenName)
	}
	if requestId != "" {
		tx = tx.Where("logs.request_id = ?", requestId)
	}
	if upstreamRequestId != "" {
		tx = tx.Where("logs.upstream_request_id = ?", upstreamRequestId)
	}
	if startTimestamp != 0 {
		tx = tx.Where("logs.created_at >= ?", startTimestamp)
	}
	if endTimestamp != 0 {
		tx = tx.Where("logs.created_at <= ?", endTimestamp)
	}
	if channel != 0 {
		tx = tx.Where("logs.channel_id = ?", channel)
	}
	if group != "" {
		tx = tx.Where("logs."+logGroupCol+" = ?", group)
	}
	if cursor != "" {
		createdAt, id, parseErr := parseLogCursor(cursor)
		if parseErr != nil {
			return nil, 0, parseErr
		}
		tx = tx.Where("(logs.created_at < ?) OR (logs.created_at = ? AND logs.id < ?)", createdAt, createdAt, id)
	} else if page == nil {
		// Keep the legacy exact-count contract. A cached total can become
		// observably stale as soon as a new log is written or an old one is
		// deleted, changing pagination metadata while the row query is intact.
		err = tx.Model(&Log{}).Count(&total).Error
		if err != nil {
			return nil, 0, err
		}
	}
	query := tx.Order("logs.created_at desc, logs.id desc").Limit(querySize)
	if cursor == "" && page == nil {
		query = query.Offset(startIdx)
	}
	err = query.Find(&logs).Error
	if err != nil {
		return nil, 0, err
	}

	logs = finishLogPage(logs, num, page)
	channelIds := types.NewSet[int]()
	for _, log := range logs {
		if log.ChannelId != 0 {
			channelIds.Add(log.ChannelId)
		}
	}

	if channelIds.Len() > 0 {
		var channels []struct {
			Id   int    `gorm:"column:id"`
			Name string `gorm:"column:name"`
		}
		if common.MemoryCacheEnabled {
			// Cache get channel
			for _, channelId := range channelIds.Items() {
				if cacheChannel, err := CacheGetChannel(channelId); err == nil {
					channels = append(channels, struct {
						Id   int    `gorm:"column:id"`
						Name string `gorm:"column:name"`
					}{
						Id:   channelId,
						Name: cacheChannel.Name,
					})
				}
			}
		} else {
			// Bulk query channels from DB
			if err = DB.WithContext(ctx).Table("channels").Select("id, name").Where("id IN ?", channelIds.Items()).Find(&channels).Error; err != nil {
				return logs, total, err
			}
		}
		channelMap := make(map[int]string, len(channels))
		for _, channel := range channels {
			channelMap[channel.Id] = channel.Name
		}
		for i := range logs {
			logs[i].ChannelName = channelMap[logs[i].ChannelId]
		}
	}

	return logs, total, err
}

const logSearchCountLimit = 10000

func GetUserLogs(userId int, logType int, startTimestamp int64, endTimestamp int64, modelName string, tokenName string, startIdx int, num int, group string, requestId string, upstreamRequestId string) (logs []*Log, total int64, err error) {
	return GetUserLogsWithContext(context.Background(), userId, logType, startTimestamp, endTimestamp, modelName, tokenName, startIdx, num, group, requestId, upstreamRequestId)
}

func GetUserLogsWithContext(ctx context.Context, userId int, logType int, startTimestamp int64, endTimestamp int64, modelName string, tokenName string, startIdx int, num int, group string, requestId string, upstreamRequestId string) (logs []*Log, total int64, err error) {
	return GetUserLogsWithContextCursor(ctx, userId, logType, startTimestamp, endTimestamp, modelName, tokenName, startIdx, num, group, requestId, upstreamRequestId, "")
}

// GetUserLogsWithContextCursor is the user-scoped keyset pagination variant.
func GetUserLogsWithContextCursor(ctx context.Context, userId int, logType int, startTimestamp int64, endTimestamp int64, modelName string, tokenName string, startIdx int, num int, group string, requestId string, upstreamRequestId string, cursor string, pages ...*LogPageMetadata) (logs []*Log, total int64, err error) {
	var page *LogPageMetadata
	if len(pages) > 0 {
		page = pages[0]
	}
	querySize := num
	if page != nil {
		querySize++
	}
	var tx *gorm.DB
	if logType == LogTypeUnknown {
		tx = LOG_DB.WithContext(ctx).Where("logs.user_id = ?", userId)
	} else {
		tx = LOG_DB.WithContext(ctx).Where("logs.user_id = ? and logs.type = ?", userId, logType)
	}

	tx = applyClientFamilyFilter(tx, ClientLogFilter(ctx))
	if tx, err = applyExplicitLogTextFilter(tx, "logs.model_name", modelName); err != nil {
		return nil, 0, err
	}
	if tokenName != "" {
		tx = tx.Where("logs.token_name = ?", tokenName)
	}
	if requestId != "" {
		tx = tx.Where("logs.request_id = ?", requestId)
	}
	if upstreamRequestId != "" {
		tx = tx.Where("logs.upstream_request_id = ?", upstreamRequestId)
	}
	if startTimestamp != 0 {
		tx = tx.Where("logs.created_at >= ?", startTimestamp)
	}
	if endTimestamp != 0 {
		tx = tx.Where("logs.created_at <= ?", endTimestamp)
	}
	if group != "" {
		tx = tx.Where("logs."+logGroupCol+" = ?", group)
	}
	if cursor != "" {
		createdAt, id, parseErr := parseLogCursor(cursor)
		if parseErr != nil {
			return nil, 0, parseErr
		}
		if page != nil {
			tx = tx.Where("logs.id < ?", id)
		} else {
			tx = tx.Where("(logs.created_at < ?) OR (logs.created_at = ? AND logs.id < ?)", createdAt, createdAt, id)
		}
	} else if page == nil {
		// Keep the legacy bounded COUNT semantics and do not cache pagination
		// metadata; callers expect total to reflect the current log table.
		err = tx.Model(&Log{}).Limit(logSearchCountLimit).Count(&total).Error
		if err != nil {
			common.SysError("failed to count user logs: " + err.Error())
			return nil, 0, errors.New("查询日志失败")
		}
	}
	// Preserve the legacy user-log ordering for offset pagination. The
	// composite timestamp/id order is only needed by the internal cursor path;
	// changing the old wrapper would subtly reorder rows with corrected or
	// imported timestamps.
	order := "logs.id desc"
	if cursor != "" && page == nil {
		order = "logs.created_at desc, logs.id desc"
	}
	query := tx.Order(order).Limit(querySize)
	if cursor == "" && page == nil {
		query = query.Offset(startIdx)
	}
	err = query.Find(&logs).Error
	if err != nil {
		common.SysError("failed to search user logs: " + err.Error())
		return nil, 0, errors.New("查询日志失败")
	}

	logs = finishLogPage(logs, num, page)
	formatUserLogs(logs, startIdx)
	return logs, total, err
}

func parseLogCursor(cursor string) (int64, int, error) {
	parts := strings.Split(cursor, ":")
	if len(parts) != 2 {
		return 0, 0, errors.New("无效的日志分页游标")
	}
	createdAt, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || createdAt <= 0 {
		return 0, 0, errors.New("无效的日志分页游标")
	}
	id64, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil || id64 <= 0 || id64 > int64(^uint(0)>>1) {
		return 0, 0, errors.New("无效的日志分页游标")
	}
	return createdAt, int(id64), nil
}

type Stat struct {
	Quota            int64 `json:"quota"`
	PreDiscountQuota int64 `json:"pre_discount_quota"`
	Rpm              int   `json:"rpm"`
	Tpm              int   `json:"tpm"`
	Tokens           int64 `json:"tokens"`
	// Stale is an internal observability marker. The legacy /api/log/*/stat
	// response is assembled explicitly by the controller and must not gain a
	// new field when stale fallback is used.
	Stale bool `json:"-"`
}

const (
	logStatCacheCapacity = 1024
	logStatCacheTTL      = 5 * time.Second
	logStatStaleTTL      = 5 * time.Minute
)

type cacheRateProjection struct {
	CacheTokens  int64 `json:"cache_tokens"`
	PromptTokens int64 `json:"prompt_tokens"`
}

const (
	userCacheRateTTL      = 30 * time.Second
	userCacheRateStaleTTL = 5 * time.Minute
)

var (
	userCacheRateHotCache = hot.NewHotCache[string, cacheRateProjection](hot.LRU, 256).
				WithTTL(userCacheRateTTL).
				WithJanitor().
				Build()
	userCacheRateStaleCache = hot.NewHotCache[string, cacheRateProjection](hot.LRU, 256).
				WithTTL(userCacheRateStaleTTL).
				WithJanitor().
				Build()
	userCacheRateQueryGroup singleflight.Group
)

var (
	logStatCache = hot.NewHotCache[string, Stat](hot.LRU, logStatCacheCapacity).
			WithTTL(logStatCacheTTL).
			WithJanitor().
			Build()
	logStatStaleCache = hot.NewHotCache[string, Stat](hot.LRU, logStatCacheCapacity).
				WithTTL(logStatStaleTTL).
				WithJanitor().
				Build()
	logStatQueryGroup singleflight.Group
)

func SumUsedQuota(logType int, startTimestamp int64, endTimestamp int64, modelName string, username string, tokenName string, channel int, group string) (stat Stat, err error) {
	return SumUsedQuotaWithContext(context.Background(), logType, startTimestamp, endTimestamp, modelName, username, tokenName, channel, group)
}

func SumUsedQuotaWithContext(ctx context.Context, logType int, startTimestamp int64, endTimestamp int64, modelName string, username string, tokenName string, channel int, group string) (stat Stat, err error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return stat, err
	}
	cacheEndTimestamp := endTimestamp
	if endTimestamp >= time.Now().Unix() {
		// The log UI deliberately sends a future upper bound for its live
		// window. Future bounds are equivalent until the short cache expires,
		// so normalize them to let nearby requests share the same aggregate.
		cacheEndTimestamp = 0
	}
	cacheKey := usageCacheLocalKey(fmt.Sprintf(
		"log-stat|%d|%d|%d|%q|%q|%q|%d|%q",
		logType,
		startTimestamp,
		cacheEndTimestamp,
		modelName,
		username,
		tokenName,
		channel,
		group,
	))
	cacheKey += "|client:" + ClientLogFilter(ctx)
	if cached, found := logStatCache.MustGet(cacheKey); found {
		return cached, nil
	}

	resultCh := logStatQueryGroup.DoChan(cacheKey, func() (any, error) {
		workCtx, workCancel := usageProjectionWorkContext(ctx)
		defer workCancel()
		if cached, found := logStatCache.MustGet(cacheKey); found {
			return cached, nil
		}

		queried, queryErr := queryUsedQuotaWithContext(WithClientLogFilter(workCtx, ClientLogFilter(ctx)),
			logType,
			startTimestamp,
			endTimestamp,
			modelName,
			username,
			tokenName,
			channel,
			group,
		)
		if queryErr != nil {
			if stale, found := logStatStaleCache.MustGet(cacheKey); found {
				stale.Stale = true
				return stale, nil
			}
			return Stat{}, queryErr
		}
		queried.Stale = false
		logStatCache.SetWithTTL(cacheKey, queried, logStatCacheTTL)
		logStatStaleCache.SetWithTTL(cacheKey, queried, logStatStaleTTL)
		return queried, nil
	})
	value, err := waitUsageProjectionResult(ctx, resultCh)
	if err != nil {
		return stat, err
	}
	return value.(Stat), nil
}

func queryUsedQuota(startTimestamp int64, endTimestamp int64, modelName string, username string, tokenName string, channel int, group string) (stat Stat, err error) {
	return queryUsedQuotaWithContext(context.Background(), LogTypeConsume, startTimestamp, endTimestamp, modelName, username, tokenName, channel, group)
}

func queryUsedQuotaWithContext(ctx context.Context, logType int, startTimestamp int64, endTimestamp int64, modelName string, username string, tokenName string, channel int, group string) (stat Stat, err error) {
	// Historical /api/log/*/stat semantics always report consumption usage,
	// even when the UI sends the "all" type value. Keep that contract while
	// allowing the internal query signature to carry the caller's value.
	logType = LogTypeConsume
	minuteCutoff := time.Now().Add(-60 * time.Second).Unix()
	selectedTimePredicate := "1 = 1"
	selectedTimeArgs := make([]interface{}, 0, 2)
	if startTimestamp != 0 {
		selectedTimePredicate += " AND created_at >= ?"
		selectedTimeArgs = append(selectedTimeArgs, startTimestamp)
	}
	if endTimestamp != 0 {
		selectedTimePredicate += " AND created_at <= ?"
		selectedTimeArgs = append(selectedTimeArgs, endTimestamp)
	}
	// Keep the legacy distinction between the selected-range aggregates and
	// RPM/TPM: the latter have always represented the last 60 seconds, even
	// when the log page is browsing an older range. Include the union of both
	// time windows in one scan, then conditionally assign each row to the
	// appropriate aggregate. This preserves the old JSON values without
	// bringing back the previous two full aggregate scans.
	selectArgs := make([]interface{}, 0, len(selectedTimeArgs)*3+2)
	selectArgs = append(selectArgs, selectedTimeArgs...)
	selectArgs = append(selectArgs, selectedTimeArgs...)
	selectArgs = append(selectArgs, selectedTimeArgs...)
	selectArgs = append(selectArgs, minuteCutoff, minuteCutoff)
	tx := LOG_DB.WithContext(ctx).Table("logs").Select(
		"coalesce(sum(CASE WHEN "+selectedTimePredicate+" THEN quota ELSE 0 END), 0) quota, "+
			"coalesce(sum(CASE WHEN "+selectedTimePredicate+" THEN CASE WHEN pre_discount_quota > 0 THEN pre_discount_quota ELSE quota END ELSE 0 END), 0) pre_discount_quota, "+
			"coalesce(sum(CASE WHEN "+selectedTimePredicate+" THEN prompt_tokens + completion_tokens ELSE 0 END), 0) tokens, "+
			"coalesce(sum(CASE WHEN created_at >= ? THEN 1 ELSE 0 END), 0) rpm, "+
			"coalesce(sum(CASE WHEN created_at >= ? THEN prompt_tokens + completion_tokens ELSE 0 END), 0) tpm",
		selectArgs...,
	)

	tx = applyClientFamilyFilter(tx, ClientLogFilter(ctx))
	if tx, err = applyExplicitLogTextFilter(tx, "username", username); err != nil {
		return stat, err
	}
	if tokenName != "" {
		tx = tx.Where("token_name = ?", tokenName)
	}
	if selectedTimePredicate != "1 = 1" {
		whereArgs := append(append([]interface{}{}, selectedTimeArgs...), minuteCutoff)
		tx = tx.Where("("+selectedTimePredicate+") OR created_at >= ?", whereArgs...)
	}
	if tx, err = applyExplicitLogTextFilter(tx, "model_name", modelName); err != nil {
		return stat, err
	}
	if channel != 0 {
		tx = tx.Where("channel_id = ?", channel)
	}
	if group != "" {
		tx = tx.Where(logGroupCol+" = ?", group)
	}

	tx = tx.Where("type = ?", logType)

	// 执行查询
	if err := tx.Scan(&stat).Error; err != nil {
		common.SysError("failed to query log stat: " + err.Error())
		return stat, errors.New("查询统计数据失败")
	}
	return stat, nil
}

func SumUsedToken(logType int, startTimestamp int64, endTimestamp int64, modelName string, username string, tokenName string) (token int) {
	tx := LOG_DB.Table("logs").Select("ifnull(sum(prompt_tokens),0) + ifnull(sum(completion_tokens),0)")
	if username != "" {
		tx = tx.Where("username = ?", username)
	}
	if tokenName != "" {
		tx = tx.Where("token_name = ?", tokenName)
	}
	if startTimestamp != 0 {
		tx = tx.Where("created_at >= ?", startTimestamp)
	}
	if endTimestamp != 0 {
		tx = tx.Where("created_at <= ?", endTimestamp)
	}
	if modelName != "" {
		tx = tx.Where("model_name = ?", modelName)
	}
	tx.Where("type = ?", LogTypeConsume).Scan(&token)
	return token
}

func DeleteOldLog(ctx context.Context, targetTimestamp int64, limit int) (int64, error) {
	var total int64 = 0

	for {
		if nil != ctx.Err() {
			return total, ctx.Err()
		}

		result := LOG_DB.Where("created_at < ?", targetTimestamp).Limit(limit).Delete(&Log{})
		if nil != result.Error {
			return total, result.Error
		}

		total += result.RowsAffected

		if result.RowsAffected < int64(limit) {
			break
		}
	}

	return total, nil
}

// GetUnsettledConsumeLogs 扫描指定时间区间内未结算的消费日志(分批, 按 id 升序翻页)。
// 历史结算兼容查询。新充值分润不读取消费日志。
func GetUnsettledConsumeLogs(dayStart, dayEnd int64, afterLogId, limit int) ([]*Log, error) {
	var logs []*Log
	err := LOG_DB.Where("settled = ? AND type = ? AND created_at >= ? AND created_at < ? AND id > ? AND quota > 0 AND (billing_source = '' OR billing_source = 'wallet')",
		false, LogTypeConsume, dayStart, dayEnd, afterLogId).
		Order("id asc").Limit(limit).Find(&logs).Error
	return logs, err
}

func CountUnresolvedWalletCostLogs(dayStart, dayEnd int64) (int64, error) {
	var count int64
	err := LOG_DB.Model(&Log{}).
		Where("settled = ? AND type = ? AND created_at >= ? AND created_at < ? AND quota > 0 AND (billing_source = '' OR billing_source = 'wallet') AND cost_rule_version >= ? AND channel_cost_ratio_ppm IS NULL",
			false, LogTypeConsume, dayStart, dayEnd, 2).
		Count(&count).Error
	return count, err
}

// GetUserCacheRate 用户个人缓存命中数据(近指定时段): 返回 sum(cache_tokens)/sum(effective_prompt_tokens)。
// 兼容两种上游口径:
// 1. prompt_tokens 已含缓存命中: 分母=prompt_tokens
// 2. prompt_tokens 只含非缓存输入: 当 cache_tokens > prompt_tokens 时, 分母=prompt_tokens+cache_tokens
// prompt_tokens<=0 的不完整日志不参与计算, 避免异常日志把命中率顶成 100%。
func GetUserCacheRate(userId int, startTime, endTime int64) (cacheTokens, promptTokens int64, err error) {
	return GetUserCacheRateWithContext(context.Background(), int64(userId), startTime, endTime)
}

// GetUserCacheRateWithContext shares the unified usage projection with the
// usage query cache. The response still exposes only the historical
// cache_tokens/prompt_tokens pair. Keeping this projection lightweight avoids
// building the model/subscription maps when the dashboard only needs cache
// efficiency. The raw logs query is deliberately the compatibility fallback:
// it does not depend on usage_log_daily_aggregates or the optional bucket
// projection, so a missing/failed archive migration cannot turn this legacy
// endpoint into a new error contract.
func GetUserCacheRateWithContext(ctx context.Context, userId, startTime, endTime int64) (cacheTokens, promptTokens int64, err error) {
	if userId <= 0 {
		return 0, 0, nil
	}
	if LOG_DB == nil {
		return 0, 0, errors.New("log database is unavailable")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return 0, 0, err
	}
	keyStart, keyEnd := normalizeHotUsageWindow(startTime, endTime)
	remoteKey := usageCacheKey("cache-rate:user", int(userId), keyStart, keyEnd, int64(time.Hour/time.Second), usageMetricTimezoneName(), "cache-rate-v1")
	key := usageCacheLocalKey(remoteKey)
	if cached, found := userCacheRateHotCache.MustGet(key); found {
		return cached.CacheTokens, cached.PromptTokens, nil
	}
	var remote cacheRateProjection
	if found, remoteErr := usageCacheGet(ctx, remoteKey, &remote); remoteErr == nil && found {
		userCacheRateHotCache.SetWithTTL(key, remote, userCacheRateTTL)
		return remote.CacheTokens, remote.PromptTokens, nil
	}
	resultCh := userCacheRateQueryGroup.DoChan(key, func() (any, error) {
		workCtx, workCancel := usageProjectionWorkContext(ctx)
		defer workCancel()
		if cached, found := userCacheRateHotCache.MustGet(key); found {
			return cached, nil
		}
		var projection cacheRateProjection
		var err error
		if metricCache, metricPrompt, ready, metricErr := readUserCacheRateFromMetricBuckets(workCtx, userId, startTime, endTime); ready && metricErr == nil {
			projection.CacheTokens = metricCache
			projection.PromptTokens = metricPrompt
		} else {
			// Projection readiness is advisory. During backfill or a partial
			// bucket failure, preserve the exact raw-log calculation.
			err = queryUserCacheRateFromLogsInto(workCtx, userId, startTime, endTime, &projection)
		}
		if err != nil {
			if stale, found := userCacheRateStaleCache.MustGet(key); found {
				return stale, nil
			}
			var remoteStale cacheRateProjection
			if found, remoteErr := usageCacheGet(workCtx, remoteKey+":stale", &remoteStale); remoteErr == nil && found {
				userCacheRateStaleCache.SetWithTTL(key, remoteStale, userCacheRateStaleTTL)
				return remoteStale, nil
			}
			return nil, err
		}
		userCacheRateHotCache.SetWithTTL(key, projection, userCacheRateTTL)
		userCacheRateStaleCache.SetWithTTL(key, projection, userCacheRateStaleTTL)
		_ = usageCacheSet(workCtx, remoteKey, projection, userCacheRateTTL)
		_ = usageCacheSet(workCtx, remoteKey+":stale", projection, userCacheRateStaleTTL)
		return projection, nil
	})
	value, queryErr := waitUsageProjectionResult(ctx, resultCh)
	if queryErr != nil {
		return 0, 0, queryErr
	}
	projection, ok := value.(cacheRateProjection)
	if !ok {
		return 0, 0, errors.New("invalid cache-rate projection")
	}
	return projection.CacheTokens, projection.PromptTokens, nil
}

// queryUserCacheRateFromLogs is the stable pre-projection implementation used
// as the final fallback for cache-rate. Keep the calculation in one place so
// future hourly-bucket reads can fail closed to this exact raw-log contract.
// It intentionally retains the pair-returning shape used by the metric-bucket
// reader for its exact hot-tail fallback.
func queryUserCacheRateFromLogs(ctx context.Context, userId, startTime, endTime int64) (cacheTokens, promptTokens int64, err error) {
	var destination cacheRateProjection
	if err := queryUserCacheRateFromLogsInto(ctx, userId, startTime, endTime, &destination); err != nil {
		return 0, 0, err
	}
	return destination.CacheTokens, destination.PromptTokens, nil
}

func queryUserCacheRateFromLogsInto(ctx context.Context, userId, startTime, endTime int64, destination *cacheRateProjection) error {
	if destination == nil {
		return errors.New("cache-rate destination is nil")
	}
	if LOG_DB == nil {
		return errors.New("log database is unavailable")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	return LOG_DB.WithContext(ctx).Model(&Log{}).
		Where("user_id = ? AND type = ? AND created_at >= ? AND created_at < ?", userId, LogTypeConsume, startTime, endTime).
		Select("COALESCE(SUM(CASE WHEN prompt_tokens <= 0 THEN 0 ELSE cache_tokens END),0) as cache_tokens, COALESCE(SUM(CASE WHEN prompt_tokens <= 0 THEN 0 WHEN cache_tokens > prompt_tokens THEN prompt_tokens + cache_tokens ELSE prompt_tokens END),0) as prompt_tokens").
		Scan(destination).Error
}

// MarkLogsSettled 批量标记日志已结算(原子 WHERE settled=false 防重跑重复算)。走 LOG_DB。
func MarkLogsSettled(ids []int, batchId string) error {
	if len(ids) == 0 {
		return nil
	}
	return LOG_DB.Model(&Log{}).Where("id IN ? AND settled = ?", ids, false).
		Updates(map[string]interface{}{"settled": true, "settle_batch_id": batchId}).Error
}
