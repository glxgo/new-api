package controller

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func luckyPagination(c *gin.Context) (int, int) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	return page, pageSize
}

func GetLuckyWheelStatus(c *gin.Context) {
	userId := c.GetInt("id")
	campaign, rule, err := model.GetLuckyCampaignTx(nil, false)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	now := model.GetDBTimestamp()
	var available int64
	if err := model.DB.Model(&model.LuckyCard{}).
		Where("user_id = ? AND status = ? AND expires_at > ?", userId, model.LuckyCardAvailable, now).
		Count(&available).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	var progress model.LuckyRechargeProgress
	if err := model.DB.First(&progress, "user_id = ?", userId).Error; errors.Is(err, gorm.ErrRecordNotFound) {
		progress = model.LuckyRechargeProgress{UserId: userId, NextThresholdCents: model.LuckyThresholdCents(1)}
	} else if err != nil {
		common.ApiError(c, err)
		return
	}
	subscriptionProgress, err := model.GetLuckySubscriptionProgress(userId, now)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{
		"campaign": campaign, "rule_set_id": rule.Id,
		"available_cards": available, "recharge_progress": progress,
		"subscription_progress": subscriptionProgress, "server_time": now,
	})
}

func GetLuckyWheelRules(c *gin.Context) {
	_, active, err := model.GetLuckyCampaignTx(nil, false)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	userId := c.GetInt("id")
	var ruleIds []int64
	if err := model.DB.Model(&model.LuckyCard{}).Where("user_id = ?", userId).
		Distinct("rule_set_id").Pluck("rule_set_id", &ruleIds).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	seen := map[int64]struct{}{active.Id: {}}
	ruleIds = append(ruleIds, active.Id)
	var rules []model.LuckyRuleSet
	if err := model.DB.Where("id IN ?", ruleIds).Order("version desc").Find(&rules).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	filtered := make([]model.LuckyRuleSet, 0, len(rules))
	for _, rule := range rules {
		if _, ok := seen[rule.Id]; ok && rule.Id != active.Id {
			continue
		}
		seen[rule.Id] = struct{}{}
		filtered = append(filtered, rule)
	}
	common.ApiSuccess(c, filtered)
}

func GetMyLuckyCards(c *gin.Context) {
	page, pageSize := luckyPagination(c)
	cards, total, err := model.ListLuckyCards(c.GetInt("id"), page, pageSize)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{"items": cards, "total": total, "page": page, "page_size": pageSize})
}

type luckyDrawRequest struct {
	CardId         int64  `json:"card_id"`
	IdempotencyKey string `json:"idempotency_key"`
}

func CreateLuckyDraw(c *gin.Context) {
	var req luckyDrawRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.CardId <= 0 ||
		strings.TrimSpace(req.IdempotencyKey) == "" {
		common.ApiErrorMsg(c, "参数错误")
		return
	}
	draw, err := service.DrawLuckyCard(c.GetInt("id"), req.CardId, strings.TrimSpace(req.IdempotencyKey))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, draw)
}

func GetMyLuckyDraws(c *gin.Context) {
	page, pageSize := luckyPagination(c)
	draws, total, err := model.ListLuckyDraws(c.GetInt("id"), page, pageSize)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{"items": draws, "total": total, "page": page, "page_size": pageSize})
}

func GetLuckyDraw(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	var draw model.LuckyDraw
	if id <= 0 || model.DB.Where("id = ? AND user_id = ?", id, c.GetInt("id")).First(&draw).Error != nil {
		common.ApiErrorMsg(c, "抽奖记录不存在")
		return
	}
	common.ApiSuccess(c, draw)
}

type luckyPauseRequest struct {
	Reason string `json:"reason"`
}

func AdminPauseLuckyIssuance(c *gin.Context) {
	var req luckyPauseRequest
	_ = c.ShouldBindJSON(&req)
	if err := service.SetLuckyIssuancePaused(true, c.GetInt("id"), req.Reason); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}

func AdminResumeLuckyIssuance(c *gin.Context) {
	var req luckyPauseRequest
	_ = c.ShouldBindJSON(&req)
	if err := service.SetLuckyIssuancePaused(false, c.GetInt("id"), req.Reason); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}

func AdminPauseLuckyDraw(c *gin.Context) {
	var req luckyPauseRequest
	_ = c.ShouldBindJSON(&req)
	if err := service.PauseLuckyDraw(c.GetInt("id"), req.Reason); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}

func AdminResumeLuckyDraw(c *gin.Context) {
	var req luckyPauseRequest
	_ = c.ShouldBindJSON(&req)
	if err := service.ResumeLuckyDraw(c.GetInt("id"), req.Reason); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}

type luckyCompensateRequest struct {
	UserId               int    `json:"user_id"`
	Count                int    `json:"count"`
	PoolType             string `json:"pool_type"`
	SourceSubscriptionId int    `json:"source_subscription_id"`
	Ticket               string `json:"ticket"`
	ExpiresAt            int64  `json:"expires_at"`
}

func AdminCompensateLuckyCards(c *gin.Context) {
	var req luckyCompensateRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.UserId <= 0 || req.Count < 1 || req.Count > 100 ||
		strings.TrimSpace(req.Ticket) == "" {
		common.ApiErrorMsg(c, "参数错误")
		return
	}
	if req.PoolType == "" {
		req.PoolType = model.LuckyPoolRecharge
	}
	var cards []model.LuckyCard
	err := model.DB.Transaction(func(tx *gorm.DB) error {
		campaign, rule, err := model.GetLuckyCampaignTx(tx, true)
		if err != nil {
			return err
		}
		expiresAt := req.ExpiresAt
		sourceSnapshot := ""
		sourceEnd := int64(0)
		if req.PoolType == model.LuckyPoolSubscription {
			var sub model.UserSubscription
			if err := tx.Where("id = ? AND user_id = ?", req.SourceSubscriptionId, req.UserId).First(&sub).Error; err != nil {
				return err
			}
			expiresAt, sourceEnd, sourceSnapshot = sub.EndTime, sub.EndTime, sub.PlanSnapshot
		} else if expiresAt <= model.GetDBTimestamp() {
			expiresAt = model.GetDBTimestamp() + rule.RechargeCardValidSeconds
		}
		campaignCopy := *campaign
		campaignCopy.IssuancePaused = false
		cards, err = model.GrantLuckyCardsTx(tx, &campaignCopy, rule, model.LuckyCardGrant{
			UserId: req.UserId, PoolType: req.PoolType, SourceType: "admin_compensation",
			SourceRef: req.Ticket, SourceSubscriptionId: req.SourceSubscriptionId,
			SourceSnapshot: sourceSnapshot, SourceEffectiveEndTime: sourceEnd,
			ExpiresAt: expiresAt, GrantKeyPrefix: fmt.Sprintf("compensation:%s", req.Ticket),
			Count: req.Count, HonorEligibilitySnapshot: true,
		})
		return err
	})
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, cards)
}

func AdminLuckyOverview(c *gin.Context) {
	campaign, rule, err := model.GetLuckyCampaignTx(nil, false)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	type statusCount struct {
		Status string `json:"status"`
		Count  int64  `json:"count"`
	}
	var cards []statusCount
	var draws int64
	model.DB.Model(&model.LuckyCard{}).Select("status, COUNT(*) AS count").Group("status").Scan(&cards)
	model.DB.Model(&model.LuckyDraw{}).Count(&draws)
	common.ApiSuccess(c, gin.H{"campaign": campaign, "active_rule": rule, "cards": cards, "draws": draws})
}

func AdminListLuckyCards(c *gin.Context) {
	page, pageSize := luckyPagination(c)
	query := model.DB.Model(&model.LuckyCard{})
	if userId, _ := strconv.Atoi(c.Query("user_id")); userId > 0 {
		query = query.Where("user_id = ?", userId)
	}
	if status := strings.TrimSpace(c.Query("status")); status != "" {
		query = query.Where("status = ?", status)
	}
	var total int64
	var cards []model.LuckyCard
	if err := query.Count(&total).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	if err := query.Order("id desc").Limit(pageSize).Offset((page - 1) * pageSize).Find(&cards).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{"items": cards, "total": total, "page": page, "page_size": pageSize})
}

func AdminListLuckyDraws(c *gin.Context) {
	page, pageSize := luckyPagination(c)
	query := model.DB.Model(&model.LuckyDraw{})
	if userId, _ := strconv.Atoi(c.Query("user_id")); userId > 0 {
		query = query.Where("user_id = ?", userId)
	}
	if prize := strings.TrimSpace(c.Query("prize_type")); prize != "" {
		query = query.Where("prize_type = ?", prize)
	}
	var total int64
	var draws []model.LuckyDraw
	if err := query.Count(&total).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	if err := query.Order("id desc").Limit(pageSize).Offset((page - 1) * pageSize).Find(&draws).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{"items": draws, "total": total, "page": page, "page_size": pageSize})
}

type luckySourceReversalRequest struct {
	SourceType string `json:"source_type"`
	TradeNo    string `json:"trade_no"`
	Reason     string `json:"reason"`
}

// AdminReverseLuckySource reconciles activity cards after the authoritative
// payment/subscription workflow has accepted a refund or chargeback. It never
// changes payment state or removes an already-delivered reward automatically.
func AdminReverseLuckySource(c *gin.Context) {
	var req luckySourceReversalRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiErrorMsg(c, "参数错误")
		return
	}
	req.SourceType = strings.TrimSpace(req.SourceType)
	req.TradeNo = strings.TrimSpace(req.TradeNo)
	req.Reason = strings.TrimSpace(req.Reason)
	if req.TradeNo == "" || req.Reason == "" ||
		(req.SourceType != "wallet_topup" && req.SourceType != "subscription_order") {
		common.ApiErrorMsg(c, "来源类型、交易号和退款原因不能为空")
		return
	}
	var result model.LuckySourceReversalResult
	err := model.DB.Transaction(func(tx *gorm.DB) error {
		switch req.SourceType {
		case "wallet_topup":
			var topUp model.TopUp
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
				Where("trade_no = ?", req.TradeNo).First(&topUp).Error; err != nil {
				return err
			}
			var err error
			result, err = model.ReverseLuckyRechargeTx(tx, &topUp, req.Reason)
			return err
		case "subscription_order":
			var order model.SubscriptionOrder
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
				Where("trade_no = ?", req.TradeNo).First(&order).Error; err != nil {
				return err
			}
			var err error
			result, err = model.ReverseSubscriptionLuckySourceTx(tx, &order, req.Reason)
			return err
		default:
			return errors.New("不支持的来源类型")
		}
	})
	if err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAudit(c, "lucky.source_reversal", map[string]interface{}{
		"sourceType": req.SourceType,
		"tradeNo":    req.TradeNo,
		"reason":     req.Reason,
		"result":     result,
	})
	common.ApiSuccess(c, result)
}

func AdminListLuckyRuleSets(c *gin.Context) {
	campaign, _, err := model.GetLuckyCampaignTx(nil, false)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	var rules []model.LuckyRuleSet
	if err := model.DB.Where("campaign_id = ?", campaign.Id).Order("version desc").Find(&rules).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, rules)
}

func AdminCreateLuckyRuleSet(c *gin.Context) {
	var req model.LuckyRuleSet
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiErrorMsg(c, "参数错误")
		return
	}
	var created model.LuckyRuleSet
	err := model.DB.Transaction(func(tx *gorm.DB) error {
		campaign, active, err := model.GetLuckyCampaignTx(tx, true)
		if err != nil {
			return err
		}
		var maxVersion int
		if err := tx.Model(&model.LuckyRuleSet{}).Where("campaign_id = ?", campaign.Id).
			Select("COALESCE(MAX(version), 0)").Scan(&maxVersion).Error; err != nil {
			return err
		}
		created = req
		created.Id = 0
		created.CampaignId = campaign.Id
		created.Version = maxVersion + 1
		created.Status = "draft"
		created.CreatedBy = c.GetInt("id")
		created.CreatedAt = model.GetDBTimestamp()
		if strings.TrimSpace(created.SubscriptionPool) == "" {
			created.SubscriptionPool = active.SubscriptionPool
		}
		if strings.TrimSpace(created.RechargePool) == "" {
			created.RechargePool = active.RechargePool
		}
		if strings.TrimSpace(created.ThresholdConfig) == "" {
			created.ThresholdConfig = active.ThresholdConfig
		}
		if created.RechargeCardValidSeconds <= 0 {
			created.RechargeCardValidSeconds = active.RechargeCardValidSeconds
		}
		if created.RechargeRewardValidSeconds <= 0 {
			created.RechargeRewardValidSeconds = active.RechargeRewardValidSeconds
		}
		if created.RechargeBonusUsdMicros <= 0 {
			created.RechargeBonusUsdMicros = active.RechargeBonusUsdMicros
		}
		if created.CrazyCardValidSeconds <= 0 {
			created.CrazyCardValidSeconds = active.CrazyCardValidSeconds
		}
		if created.CrazyCardQuotaUsdMicros <= 0 {
			created.CrazyCardQuotaUsdMicros = active.CrazyCardQuotaUsdMicros
		}
		if strings.TrimSpace(created.ActivityGroup) == "" {
			created.ActivityGroup = active.ActivityGroup
		}
		if err := model.ValidateLuckyRuleSet(&created); err != nil {
			return err
		}
		model.RefreshLuckyRuleChecksum(&created)
		return tx.Create(&created).Error
	})
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, created)
}

func AdminActivateLuckyRuleSet(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	if id <= 0 {
		common.ApiErrorMsg(c, "规则版本无效")
		return
	}
	err := model.DB.Transaction(func(tx *gorm.DB) error {
		campaign, _, err := model.GetLuckyCampaignTx(tx, true)
		if err != nil {
			return err
		}
		var rule model.LuckyRuleSet
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND campaign_id = ?", id, campaign.Id).First(&rule).Error; err != nil {
			return err
		}
		if rule.Status == "active" {
			return nil
		}
		if rule.Status != "draft" {
			return errors.New("只有草稿规则可以激活")
		}
		if err := model.ValidateLuckyRuleSet(&rule); err != nil {
			return err
		}
		model.RefreshLuckyRuleChecksum(&rule)
		now := model.GetDBTimestamp()
		if err := tx.Model(&model.LuckyRuleSet{}).
			Where("campaign_id = ? AND status = ?", campaign.Id, "active").
			Updates(map[string]interface{}{"status": "retired"}).Error; err != nil {
			return err
		}
		rule.Status = "active"
		rule.PublishedAt = now
		rule.EffectiveAt = now
		if err := tx.Save(&rule).Error; err != nil {
			return err
		}
		campaign.ActiveRuleSetId = rule.Id
		campaign.SettingsVersion++
		return tx.Save(campaign).Error
	})
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}

type consumptionOrderRequest struct {
	Group           string `json:"group"`
	Revision        int64  `json:"revision"`
	SubscriptionIds []int  `json:"subscription_ids"`
}

func GetSubscriptionConsumptionOrder(c *gin.Context) {
	userId := c.GetInt("id")
	group := strings.TrimSpace(c.Query("group"))
	now := model.GetDBTimestamp()
	var subs []model.UserSubscription
	if err := model.DB.Where("user_id = ? AND status = ? AND start_time <= ? AND end_time > ? AND (allowed_group = '' OR allowed_group = ?)",
		userId, "active", now, now, group).Find(&subs).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	var rows []model.SubscriptionConsumptionPriority
	if err := model.DB.Where("user_id = ? AND group_name = ?", userId, group).Order("priority asc").Find(&rows).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	revision := int64(0)
	for _, row := range rows {
		if row.Revision > revision {
			revision = row.Revision
		}
	}
	common.ApiSuccess(c, gin.H{"group": group, "revision": revision, "subscriptions": subs, "order": rows})
}

func UpdateSubscriptionConsumptionOrder(c *gin.Context) {
	userId := c.GetInt("id")
	var req consumptionOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiErrorMsg(c, "参数错误")
		return
	}
	req.Group = strings.TrimSpace(req.Group)
	seen := make(map[int]struct{}, len(req.SubscriptionIds))
	for _, id := range req.SubscriptionIds {
		if id <= 0 {
			common.ApiErrorMsg(c, "订阅实例无效")
			return
		}
		if _, ok := seen[id]; ok {
			common.ApiErrorMsg(c, "订阅实例不能重复")
			return
		}
		seen[id] = struct{}{}
	}
	var newRevision int64
	err := model.DB.Transaction(func(tx *gorm.DB) error {
		var current int64
		if err := tx.Model(&model.SubscriptionConsumptionPriority{}).
			Where("user_id = ? AND group_name = ?", userId, req.Group).
			Select("COALESCE(MAX(revision), 0)").Scan(&current).Error; err != nil {
			return err
		}
		if current != req.Revision {
			return gorm.ErrInvalidTransaction
		}
		if len(req.SubscriptionIds) > 0 {
			var count int64
			if err := tx.Model(&model.UserSubscription{}).
				Where("user_id = ? AND id IN ? AND status = ? AND (allowed_group = '' OR allowed_group = ?)",
					userId, req.SubscriptionIds, "active", req.Group).
				Count(&count).Error; err != nil {
				return err
			}
			if count != int64(len(req.SubscriptionIds)) {
				return errors.New("包含无效或不兼容的订阅实例")
			}
		}
		if err := tx.Where("user_id = ? AND group_name = ?", userId, req.Group).
			Delete(&model.SubscriptionConsumptionPriority{}).Error; err != nil {
			return err
		}
		newRevision = current + 1
		for i, id := range req.SubscriptionIds {
			row := model.SubscriptionConsumptionPriority{
				UserId: userId, GroupName: req.Group, SubscriptionId: id,
				Priority: i + 1, Revision: newRevision, UpdatedAt: model.GetDBTimestamp(),
			}
			if err := tx.Clauses(clause.OnConflict{DoNothing: false}).Create(&row).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if errors.Is(err, gorm.ErrInvalidTransaction) {
		c.JSON(409, gin.H{"success": false, "message": "套餐顺序已在其他页面更新，请刷新后重试"})
		return
	}
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{"revision": newRevision})
}
