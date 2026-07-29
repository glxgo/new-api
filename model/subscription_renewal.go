package model

import (
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

var (
	ErrSubscriptionRenewalNotAvailable  = errors.New("当前订阅实例无法续费")
	ErrSubscriptionRenewalAlreadyExists = errors.New("当前订阅实例已经存在续费后继实例")
	ErrSubscriptionRenewalOrderPending  = errors.New("当前订阅实例已经存在待支付的续费订单")
)

type RenewalBindingSnapshotEntry struct {
	TokenId            int    `json:"token_id"`
	TokenName          string `json:"token_name"`
	FromGroup          string `json:"from_group"`
	ToGroup            string `json:"to_group"`
	EffectiveAt        int64  `json:"effective_at"`
	AppliedImmediately bool   `json:"applied_immediately"`
}

type SubscriptionRenewalPreview struct {
	FromSubscription *UserSubscription             `json:"from_subscription"`
	Plan             *SubscriptionPlan             `json:"plan"`
	IsReplacement    bool                          `json:"is_replacement"`
	BindingChanges   []RenewalBindingSnapshotEntry `json:"binding_changes"`
	StartTime        int64                         `json:"start_time"`
	EndTime          int64                         `json:"end_time"`
}

func ResolveSubscriptionRenewalPlan(userId, fromSubscriptionId int) (*UserSubscription, *SubscriptionPlan, bool, error) {
	if userId <= 0 || fromSubscriptionId <= 0 {
		return nil, nil, false, ErrSubscriptionRenewalNotAvailable
	}
	var source UserSubscription
	if err := DB.Where("id = ? AND user_id = ?", fromSubscriptionId, userId).First(&source).Error; err != nil {
		return nil, nil, false, err
	}
	var successorCount int64
	if err := DB.Model(&UserSubscription{}).Where("renewed_from_id = ?", source.Id).Count(&successorCount).Error; err != nil {
		return nil, nil, false, err
	}
	if successorCount > 0 {
		return nil, nil, false, ErrSubscriptionRenewalAlreadyExists
	}
	var pendingCount int64
	if err := DB.Model(&SubscriptionOrder{}).
		Where("renew_from_subscription_id = ? AND status = ?", source.Id, common.TopUpStatusPending).
		Count(&pendingCount).Error; err != nil {
		return nil, nil, false, err
	}
	if pendingCount > 0 {
		return nil, nil, false, ErrSubscriptionRenewalOrderPending
	}

	plan, err := GetSubscriptionPlanById(source.PlanId)
	if err != nil {
		return nil, nil, false, err
	}
	isReplacement := false
	if !plan.Enabled {
		if plan.RenewalPlanId == nil || *plan.RenewalPlanId <= 0 {
			return nil, nil, false, ErrSubscriptionRenewalNotAvailable
		}
		plan, err = GetSubscriptionPlanById(*plan.RenewalPlanId)
		if err != nil {
			return nil, nil, false, err
		}
		if !plan.Enabled {
			return nil, nil, false, ErrSubscriptionRenewalNotAvailable
		}
		isReplacement = true
	}
	return &source, plan, isReplacement, nil
}

func renewalTargetGroup(tokenGroup string, plan *SubscriptionPlan) string {
	if plan == nil {
		return strings.TrimSpace(tokenGroup)
	}
	if group := strings.TrimSpace(plan.AllowedGroup); group != "" {
		return group
	}
	return strings.TrimSpace(tokenGroup)
}

func renewalStartAndEnd(source *UserSubscription, plan *SubscriptionPlan, now int64) (int64, int64, error) {
	if source == nil || plan == nil {
		return 0, 0, errors.New("invalid renewal args")
	}
	start := source.EndTime
	if start < now {
		start = now
	}
	end, err := calcPlanEndTime(timeFromUnix(start), plan)
	return start, end, err
}

func timeFromUnix(value int64) time.Time {
	return time.Unix(value, 0)
}

func GetSubscriptionRenewalPreview(userId, fromSubscriptionId int) (*SubscriptionRenewalPreview, error) {
	source, plan, replacement, err := ResolveSubscriptionRenewalPlan(userId, fromSubscriptionId)
	if err != nil {
		return nil, err
	}
	now := GetDBTimestamp()
	start, end, err := renewalStartAndEnd(source, plan, now)
	if err != nil {
		return nil, err
	}
	var tokens []Token
	if err := DB.Where(
		"user_id = ? AND subscription_mode = ? AND subscription_id = ?",
		userId,
		TokenSubscriptionModeInstance,
		source.Id,
	).Order("id asc").Find(&tokens).Error; err != nil {
		return nil, err
	}
	changes := make([]RenewalBindingSnapshotEntry, 0, len(tokens))
	for _, token := range tokens {
		changes = append(changes, RenewalBindingSnapshotEntry{
			TokenId:     token.Id,
			TokenName:   token.Name,
			FromGroup:   token.Group,
			ToGroup:     renewalTargetGroup(token.Group, plan),
			EffectiveAt: start,
		})
	}
	return &SubscriptionRenewalPreview{
		FromSubscription: source,
		Plan:             plan,
		IsReplacement:    replacement,
		BindingChanges:   changes,
		StartTime:        start,
		EndTime:          end,
	}, nil
}

func ConfigureSubscriptionOrderRenewal(order *SubscriptionOrder, userId, fromSubscriptionId int) error {
	if order == nil {
		return errors.New("order is nil")
	}
	source, plan, _, err := ResolveSubscriptionRenewalPlan(userId, fromSubscriptionId)
	if err != nil {
		return err
	}
	if order.UserId != userId || order.PlanId != plan.Id {
		return errors.New("renewal plan does not match order")
	}
	order.RenewFromSubscriptionId = &source.Id
	return nil
}

func renewalRemark(current string) string {
	current = strings.TrimSpace(current)
	if current == "" {
		return ""
	}
	result := current + "（续费）"
	const maxRunes = 128
	if utf8.RuneCountInString(result) <= maxRunes {
		return result
	}
	runes := []rune(result)
	return string(runes[:maxRunes])
}

func scheduleRenewalBindingsTx(tx *gorm.DB, source, successor *UserSubscription, plan *SubscriptionPlan, now int64) ([]RenewalBindingSnapshotEntry, []string, error) {
	if tx == nil || source == nil || successor == nil || plan == nil {
		return nil, nil, errors.New("invalid renewal binding args")
	}
	var tokens []Token
	if err := tx.Set("gorm:query_option", "FOR UPDATE").
		Where(
			"user_id = ? AND subscription_mode = ? AND subscription_id = ?",
			source.UserId,
			TokenSubscriptionModeInstance,
			source.Id,
		).
		Order("id asc").
		Find(&tokens).Error; err != nil {
		return nil, nil, err
	}
	entries := make([]RenewalBindingSnapshotEntry, 0, len(tokens))
	tokenKeys := make([]string, 0, len(tokens))
	for _, token := range tokens {
		before := token
		targetGroup := renewalTargetGroup(token.Group, plan)
		immediate := successor.StartTime <= now
		updates := map[string]any{}
		action := TokenSubscriptionActionRenewalScheduled
		if immediate {
			token.Group = targetGroup
			token.SubscriptionId = successor.Id
			token.SubscriptionWalletCycleId = successor.Id
			token.SubscriptionWalletUsed = 0
			token.PlannedSubscriptionId = 0
			token.PlannedSubscriptionGroup = ""
			token.PlannedSubscriptionEffective = 0
			updates["group"] = targetGroup
			updates["subscription_id"] = successor.Id
			updates["subscription_wallet_cycle_id"] = successor.Id
			updates["subscription_wallet_used"] = 0
			updates["planned_subscription_id"] = 0
			updates["planned_subscription_group"] = ""
			updates["planned_subscription_effective"] = 0
			action = TokenSubscriptionActionRenewalActivated
		} else {
			token.PlannedSubscriptionId = successor.Id
			token.PlannedSubscriptionGroup = targetGroup
			token.PlannedSubscriptionEffective = successor.StartTime
			updates["planned_subscription_id"] = successor.Id
			updates["planned_subscription_group"] = targetGroup
			updates["planned_subscription_effective"] = successor.StartTime
		}
		if err := tx.Model(&Token{}).Where("id = ? AND user_id = ?", token.Id, token.UserId).
			Updates(updates).Error; err != nil {
			return nil, nil, err
		}
		history := bindingHistoryFromTokens(
			&before,
			&token,
			"system",
			action,
			fmt.Sprintf("renewal successor #%d", successor.Id),
		)
		history.ToSubscriptionId = successor.Id
		history.ToGroup = targetGroup
		if err := tx.Create(history).Error; err != nil {
			return nil, nil, err
		}
		entries = append(entries, RenewalBindingSnapshotEntry{
			TokenId:            token.Id,
			TokenName:          token.Name,
			FromGroup:          before.Group,
			ToGroup:            targetGroup,
			EffectiveAt:        successor.StartTime,
			AppliedImmediately: immediate,
		})
		if token.Key != "" {
			tokenKeys = append(tokenKeys, token.Key)
		}
	}
	return entries, tokenKeys, nil
}

func createRenewalSubscriptionFromOrderTx(tx *gorm.DB, order *SubscriptionOrder, plan *SubscriptionPlan, paidRevenueQuota int64) (*UserSubscription, []string, error) {
	if tx == nil || order == nil || order.RenewFromSubscriptionId == nil || *order.RenewFromSubscriptionId <= 0 {
		return nil, nil, errors.New("invalid renewal order")
	}
	var source UserSubscription
	if err := tx.Set("gorm:query_option", "FOR UPDATE").
		Where("id = ? AND user_id = ?", *order.RenewFromSubscriptionId, order.UserId).
		First(&source).Error; err != nil {
		return nil, nil, err
	}
	var successorCount int64
	if err := tx.Model(&UserSubscription{}).Where("renewed_from_id = ?", source.Id).Count(&successorCount).Error; err != nil {
		return nil, nil, err
	}
	if successorCount > 0 {
		return nil, nil, ErrSubscriptionRenewalAlreadyExists
	}
	now := GetDBTimestamp()
	start, _, err := renewalStartAndEnd(&source, plan, now)
	if err != nil {
		return nil, nil, err
	}
	successor, err := CreateUserSubscriptionFromPlanWithOptionsTx(
		tx,
		order.UserId,
		plan,
		"renewal",
		CreateUserSubscriptionOptions{
			StartTime:         start,
			RenewedFromId:     &source.Id,
			Remark:            renewalRemark(source.Remark),
			PlanSnapshot:      order.PlanSnapshot,
			SkipPurchaseLimit: true,
		},
		paidRevenueQuota,
	)
	if err != nil {
		return nil, nil, err
	}
	entries, tokenKeys, err := scheduleRenewalBindingsTx(tx, &source, successor, plan, now)
	if err != nil {
		return nil, nil, err
	}
	snapshot, err := common.Marshal(entries)
	if err != nil {
		return nil, nil, err
	}
	order.RenewalBindingSnapshot = string(snapshot)
	return successor, tokenKeys, nil
}
