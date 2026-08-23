package model

import (
	"errors"
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"gorm.io/gorm"
)

const (
	TokenRoutingModeSingle = "single"
	TokenRoutingModeCustom = "custom"

	TokenRouteSourceWallet            = "wallet"
	TokenRouteSourceSubscription      = "subscription"
	TokenRouteSourceVirtualMembership = "virtual_membership"

	TokenRouteSelectionAuto     = "auto"
	TokenRouteSelectionInstance = "instance"

	VirtualMembershipModeAuto     = "auto"
	VirtualMembershipModeInstance = "instance"
)

// TokenRouteStep is one explicit, ordered funding-and-routing boundary for an
// API Key. GroupName is always a real channel group; there is deliberately no
// implicit/default wallet group.
type TokenRouteStep struct {
	Id            int    `json:"id"`
	TokenId       int    `json:"token_id" gorm:"not null;index;uniqueIndex:idx_token_route_position,priority:1"`
	UserId        int    `json:"user_id" gorm:"not null;index"`
	Position      int    `json:"position" gorm:"not null;uniqueIndex:idx_token_route_position,priority:2"`
	GroupName     string `json:"group" gorm:"column:group_name;type:varchar(64);not null;index"`
	FundingSource string `json:"funding_source" gorm:"type:varchar(32);not null;index"`
	SelectionMode string `json:"selection_mode" gorm:"type:varchar(16);not null;default:'auto'"`
	SourceId      int    `json:"source_id" gorm:"not null;default:0;index"`
	CreatedAt     int64  `json:"created_at" gorm:"type:bigint;not null"`
	UpdatedAt     int64  `json:"updated_at" gorm:"type:bigint;not null"`
}

type TokenRouteQuotaAvailability struct {
	Usable  bool
	ResetAt int64
}

func routeQuotaRequiredAmount(requiredAmount int64) int64 {
	if requiredAmount <= 0 {
		return 1
	}
	return requiredAmount
}

func subscriptionRouteQuotaAvailability(userId int, step TokenRouteStep, requiredAmount int64) (TokenRouteQuotaAvailability, error) {
	var result TokenRouteQuotaAvailability
	now := GetDBTimestamp()
	amount := routeQuotaRequiredAmount(requiredAmount)
	var subscriptions []UserSubscription
	query := DB.Where("user_id = ? AND status = ? AND start_time <= ? AND end_time > ? AND allowed_group = ?",
		userId, "active", now, now, strings.TrimSpace(step.GroupName))
	if step.SelectionMode == TokenRouteSelectionInstance {
		query = query.Where("id = ?", step.SourceId)
	}
	if err := query.Order("end_time asc, id asc").Find(&subscriptions).Error; err != nil {
		return result, err
	}
	for i := range subscriptions {
		sub := &subscriptions[i]
		capUsable := sub.AmountCap <= 0 || sub.AmountCapUsed+amount <= sub.AmountCap
		plan, _ := planForUserSubscriptionTx(DB, sub)
		cycleResetDue := subscriptionResetBoundaryDueFor(sub, plan, now)
		cycleUsed := sub.AmountUsed
		if cycleResetDue {
			cycleUsed = 0
		}
		cycleUsable := sub.AmountTotal <= 0 || cycleUsed+amount <= sub.AmountTotal
		if capUsable && cycleUsable {
			return TokenRouteQuotaAvailability{Usable: true}, nil
		}
		readyAt := sub.EndTime
		if capUsable && !cycleUsable {
			candidateReset := sub.NextResetTime
			if subscriptionUsesPurchaseResetAnchorFor(sub, plan) && sub.StartTime > 0 {
				_, projected := purchaseAnchoredResetSchedule(plan, sub.StartTime, sub.EndTime, now)
				candidateReset = projected
			}
			if candidateReset > now {
				readyAt = candidateReset
			}
		}
		if readyAt > now && (result.ResetAt == 0 || readyAt < result.ResetAt) {
			result.ResetAt = readyAt
		}
	}
	return result, nil
}

func virtualMembershipReadyAt(membership *UserVirtualMembership, amount, now int64) (bool, int64) {
	if membership == nil || membership.Status != VirtualMembershipStatusActive || membership.StartTime > now || membership.EndTime <= now {
		return false, 0
	}
	weeklyBlocked := membership.WeeklyQuota > 0 && membership.WeeklyUsed+amount > membership.WeeklyQuota
	fiveHourBlocked := membership.FiveHourActive && membership.FiveHourQuota > 0 && membership.FiveHourUsed+amount > membership.FiveHourQuota
	if !weeklyBlocked && !fiveHourBlocked {
		return true, 0
	}
	readyAt := int64(0)
	if weeklyBlocked {
		readyAt = membership.WeeklyResetAt
	}
	if fiveHourBlocked && membership.FiveHourResetAt > readyAt {
		readyAt = membership.FiveHourResetAt
	}
	if readyAt <= now {
		readyAt = 0
	}
	return false, readyAt
}

func virtualMembershipRouteQuotaAvailability(userId int, modelName string, step TokenRouteStep, requiredAmount int64) (TokenRouteQuotaAvailability, error) {
	var result TokenRouteQuotaAvailability
	amount := routeQuotaRequiredAmount(requiredAmount)
	err := DB.Transaction(func(tx *gorm.DB) error {
		now := GetDBTimestamp()
		var memberships []UserVirtualMembership
		query := tx.Set("gorm:query_option", "FOR UPDATE").
			Where("user_id = ? AND status = ? AND start_time <= ? AND end_time > ?",
				userId, VirtualMembershipStatusActive, now, now)
		group := strings.TrimSpace(step.GroupName)
		if group == VirtualMembershipDefaultAllowedGroup {
			query = query.Where("(allowed_group = ? OR allowed_group = '')", group)
		} else {
			query = query.Where("allowed_group = ?", group)
		}
		if step.SelectionMode == TokenRouteSelectionInstance {
			query = query.Where("id = ?", step.SourceId)
		}
		if err := query.Order("weekly_reset_at asc, end_time asc, id asc").Find(&memberships).Error; err != nil {
			return err
		}
		for i := range memberships {
			membership := &memberships[i]
			if err := virtualMembershipResetIfDue(tx, membership, now); err != nil {
				return err
			}
			if modelName != "" && !virtualMembershipAllowsModel(membership, modelName) {
				continue
			}
			usable, readyAt := virtualMembershipReadyAt(membership, amount, now)
			if usable {
				result.Usable = true
				result.ResetAt = 0
				return nil
			}
			if readyAt > 0 && (result.ResetAt == 0 || readyAt < result.ResetAt) {
				result.ResetAt = readyAt
			}
		}
		return nil
	})
	return result, err
}

// GetTokenRouteQuotaAvailability returns both current usability and the first
// ledger reset that can make an entitlement route usable again. Wallet routes
// intentionally have no quota freeze; their runtime policy is ordering-only.
func GetTokenRouteQuotaAvailability(userId int, modelName string, step TokenRouteStep, requiredAmount int64) (TokenRouteQuotaAvailability, error) {
	switch step.FundingSource {
	case TokenRouteSourceSubscription:
		return subscriptionRouteQuotaAvailability(userId, step, requiredAmount)
	case TokenRouteSourceVirtualMembership:
		return virtualMembershipRouteQuotaAvailability(userId, modelName, step, requiredAmount)
	case TokenRouteSourceWallet:
		return TokenRouteQuotaAvailability{Usable: true}, nil
	default:
		return TokenRouteQuotaAvailability{}, errors.New("API Key 消耗路由资金来源无效")
	}
}

func NormalizeTokenRoutingMode(mode string) string {
	if strings.TrimSpace(mode) == TokenRoutingModeCustom {
		return TokenRoutingModeCustom
	}
	return TokenRoutingModeSingle
}

func NormalizeVirtualMembershipMode(mode string) string {
	if strings.TrimSpace(mode) == VirtualMembershipModeAuto {
		return VirtualMembershipModeAuto
	}
	return VirtualMembershipModeInstance
}

func inferTokenRouteFundingSource(group string) (string, error) {
	virtual, err := HasVirtualMembershipPlanByGroup(group)
	if err != nil {
		return "", err
	}
	subscription, err := HasSubscriptionPlanByGroup(group)
	if err != nil {
		return "", err
	}
	if virtual && subscription {
		return "", errors.New("当前分组同时属于套餐与虚拟会员，请联系管理员修正分组配置")
	}
	if virtual {
		return TokenRouteSourceVirtualMembership, nil
	}
	if subscription {
		return TokenRouteSourceSubscription, nil
	}
	return TokenRouteSourceWallet, nil
}

// PrepareTokenRouteSteps validates ownership and derives funding_source on the
// server. The client may choose a group and an allocation mode, but cannot turn
// a protected entitlement group into wallet billing.
func PrepareTokenRouteSteps(userId int, steps []TokenRouteStep) ([]TokenRouteStep, error) {
	if userId <= 0 {
		return nil, errors.New("用户无效")
	}
	if len(steps) == 0 || len(steps) > 16 {
		return nil, errors.New("消耗路由必须包含 1 到 16 个分组")
	}
	now := common.GetTimestamp()
	prepared := make([]TokenRouteStep, 0, len(steps))
	seenGroups := make(map[string]struct{}, len(steps))
	for i := range steps {
		step := steps[i]
		step.GroupName = strings.TrimSpace(step.GroupName)
		if step.GroupName == "" || step.GroupName == "auto" {
			return nil, fmt.Errorf("第 %d 条路线必须选择明确的实际分组", i+1)
		}
		if _, exists := seenGroups[step.GroupName]; exists {
			return nil, fmt.Errorf("分组 %s 在消耗路由中重复", step.GroupName)
		}
		seenGroups[step.GroupName] = struct{}{}
		if !ratio_setting.ContainsGroupRatio(step.GroupName) {
			return nil, fmt.Errorf("分组 %s 已被弃用或未配置倍率", step.GroupName)
		}
		funding, err := inferTokenRouteFundingSource(step.GroupName)
		if err != nil {
			return nil, err
		}
		step.Position = i + 1
		step.UserId = userId
		step.FundingSource = funding
		if step.SelectionMode != TokenRouteSelectionInstance {
			step.SelectionMode = TokenRouteSelectionAuto
			step.SourceId = 0
		}
		switch funding {
		case TokenRouteSourceWallet:
			step.SelectionMode = TokenRouteSelectionAuto
			step.SourceId = 0
		case TokenRouteSourceSubscription:
			if step.SelectionMode == TokenRouteSelectionInstance {
				if _, err := ValidateActiveSubscriptionEntitlementForToken(userId, step.GroupName, step.SourceId); err != nil {
					return nil, err
				}
			} else {
				has, err := HasActiveUserSubscriptionByGroup(userId, step.GroupName)
				if err != nil || !has {
					return nil, fmt.Errorf("当前没有可用于 %s 的有效套餐", step.GroupName)
				}
			}
		case TokenRouteSourceVirtualMembership:
			if step.SelectionMode == TokenRouteSelectionInstance {
				if err := ValidateActiveVirtualMembershipEntitlementForToken(userId, step.GroupName, step.SourceId); err != nil {
					return nil, err
				}
			} else {
				has, err := HasActiveUserVirtualMembershipByGroup(userId, step.GroupName)
				if err != nil || !has {
					return nil, fmt.Errorf("当前没有可用于 %s 的有效会员额度", step.GroupName)
				}
			}
		}
		step.CreatedAt = now
		step.UpdatedAt = now
		prepared = append(prepared, step)
	}
	return prepared, nil
}

func GetTokenRouteSteps(userId, tokenId int) ([]TokenRouteStep, error) {
	var steps []TokenRouteStep
	err := DB.Where("user_id = ? AND token_id = ?", userId, tokenId).
		Order("position asc, id asc").Find(&steps).Error
	return steps, err
}

func AttachTokenRouteSteps(token *Token) error {
	if token == nil || token.Id <= 0 || NormalizeTokenRoutingMode(token.RoutingMode) != TokenRoutingModeCustom {
		return nil
	}
	steps, err := GetTokenRouteSteps(token.UserId, token.Id)
	if err == nil {
		token.RouteSteps = steps
	}
	return err
}

// InsertTokenWithRoute creates a key and its first route revision in one
// transaction, so a custom key can never exist without its ordered steps.
func InsertTokenWithRoute(token *Token, steps []TokenRouteStep) error {
	if token == nil || token.UserId <= 0 {
		return errors.New("API Key 参数无效")
	}
	prepared, err := PrepareTokenRouteSteps(token.UserId, steps)
	if err != nil {
		return err
	}
	now := common.GetTimestamp()
	token.RoutingMode = TokenRoutingModeCustom
	token.RoutingRevision = 1
	token.Group = prepared[0].GroupName
	token.CrossGroupRetry = false
	token.SubscriptionMode = TokenSubscriptionModeAuto
	token.SubscriptionId = 0
	token.VirtualMembershipId = 0
	token.VirtualMembershipMode = VirtualMembershipModeInstance
	return DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(token).Error; err != nil {
			return err
		}
		for i := range prepared {
			prepared[i].Id = 0
			prepared[i].TokenId = token.Id
			prepared[i].CreatedAt = now
			prepared[i].UpdatedAt = now
			if err := tx.Create(&prepared[i]).Error; err != nil {
				return err
			}
		}
		token.RouteSteps = prepared
		return nil
	})
}

// UpdateTokenWithRoute atomically updates editable key fields and replaces the
// ordered route under an optimistic revision check.
func UpdateTokenWithRoute(userId int, desired *Token, steps []TokenRouteStep, revision int64) (*Token, error) {
	if desired == nil || desired.Id <= 0 || userId <= 0 {
		return nil, errors.New("API Key 参数无效")
	}
	prepared, err := PrepareTokenRouteSteps(userId, steps)
	if err != nil {
		return nil, err
	}
	var updated Token
	var tokenKey string
	now := common.GetTimestamp()
	err = DB.Transaction(func(tx *gorm.DB) error {
		var current Token
		if err := tx.Set("gorm:query_option", "FOR UPDATE").
			Where("id = ? AND user_id = ?", desired.Id, userId).
			First(&current).Error; err != nil {
			return err
		}
		if current.RoutingRevision != revision {
			return errors.New("API Key 消耗路由策略已被其他窗口修改，请刷新后重试")
		}
		current.Name = desired.Name
		current.Status = desired.Status
		current.ExpiredTime = desired.ExpiredTime
		current.RemainQuota = desired.RemainQuota
		current.UnlimitedQuota = desired.UnlimitedQuota
		current.ModelLimitsEnabled = desired.ModelLimitsEnabled
		current.ModelLimits = desired.ModelLimits
		current.AllowIps = desired.AllowIps
		current.Group = prepared[0].GroupName
		current.CrossGroupRetry = false
		current.RoutingMode = TokenRoutingModeCustom
		current.RoutingRevision = revision + 1
		applyBindingInput(&current, TokenSubscriptionBindingInput{Mode: TokenSubscriptionModeAuto, CancelPlanned: true})
		current.VirtualMembershipId = 0
		current.VirtualMembershipMode = VirtualMembershipModeInstance
		if err := tx.Model(&Token{}).Where("id = ? AND user_id = ?", current.Id, userId).Select(
			"name", "status", "expired_time", "remain_quota", "unlimited_quota",
			"model_limits_enabled", "model_limits", "allow_ips", "group", "cross_group_retry",
			"routing_mode", "routing_revision", "subscription_mode", "subscription_id",
			"subscription_allow_renewal", "subscription_allow_same_group", "subscription_allow_wallet",
			"subscription_wallet_limit", "subscription_wallet_used", "subscription_wallet_cycle_id",
			"planned_subscription_id", "planned_subscription_group", "planned_subscription_effective",
			"virtual_membership_id", "virtual_membership_mode",
		).Updates(&current).Error; err != nil {
			return err
		}
		if err := tx.Where("token_id = ? AND user_id = ?", current.Id, userId).Delete(&TokenRouteStep{}).Error; err != nil {
			return err
		}
		for i := range prepared {
			prepared[i].Id = 0
			prepared[i].TokenId = current.Id
			prepared[i].CreatedAt = now
			prepared[i].UpdatedAt = now
			if err := tx.Create(&prepared[i]).Error; err != nil {
				return err
			}
		}
		current.RouteSteps = prepared
		updated = current
		tokenKey = current.Key
		return nil
	})
	if err != nil {
		return nil, err
	}
	if common.RedisEnabled && tokenKey != "" {
		_ = cacheSetToken(updated)
	}
	return &updated, nil
}
