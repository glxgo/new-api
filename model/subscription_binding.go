package model

import (
	"errors"
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

const (
	TokenSubscriptionModeAuto     = "auto"
	TokenSubscriptionModeInstance = "instance"

	TokenSubscriptionActionBind             = "bind"
	TokenSubscriptionActionRebind           = "rebind"
	TokenSubscriptionActionUnbind           = "unbind"
	TokenSubscriptionActionAutoRenew        = "auto_renew"
	TokenSubscriptionActionAutoSameGroup    = "auto_same_group"
	TokenSubscriptionActionRenewalScheduled = "renewal_scheduled"
	TokenSubscriptionActionRenewalActivated = "renewal_activated"
	TokenSubscriptionActionRenewalCancelled = "renewal_cancelled"
	TokenSubscriptionActionGroupChanged     = "group_changed"
)

var (
	ErrTokenSubscriptionNotFound       = errors.New("订阅实例不存在")
	ErrTokenSubscriptionNotUsable      = errors.New("订阅实例尚未生效、已结束或额度不足")
	ErrTokenSubscriptionGroupMismatch  = errors.New("订阅实例不支持当前 API Key 分组")
	ErrTokenWalletFallbackDisabled     = errors.New("此 API Key 未启用钱包接续")
	ErrTokenWalletFallbackLimitReached = errors.New("此 API Key 的钱包接续额度已达到上限")
)

// TokenSubscriptionBindingHistory deliberately stores token ids instead of
// token secrets. It remains after a token or subscription is soft-deleted so
// users and administrators can reconcile later billing disputes.
type TokenSubscriptionBindingHistory struct {
	Id                  int    `json:"id"`
	UserId              int    `json:"user_id" gorm:"not null;index"`
	TokenId             int    `json:"token_id" gorm:"not null;index"`
	ActorType           string `json:"actor_type" gorm:"type:varchar(16);not null;default:'user'"`
	Action              string `json:"action" gorm:"type:varchar(32);not null;index"`
	FromSubscriptionId  int    `json:"from_subscription_id" gorm:"not null;default:0;index"`
	ToSubscriptionId    int    `json:"to_subscription_id" gorm:"not null;default:0;index"`
	FromGroup           string `json:"from_group" gorm:"type:varchar(64);not null;default:''"`
	ToGroup             string `json:"to_group" gorm:"type:varchar(64);not null;default:''"`
	ContinuationSummary string `json:"continuation_summary" gorm:"type:varchar(255);not null;default:''"`
	Reason              string `json:"reason" gorm:"type:varchar(255);not null;default:''"`
	CreatedAt           int64  `json:"created_at" gorm:"type:bigint;not null;index"`
}

type TokenSubscriptionBindingInput struct {
	Mode           string `json:"subscription_mode"`
	SubscriptionId int    `json:"subscription_id"`
	AllowRenewal   bool   `json:"subscription_allow_renewal"`
	AllowSameGroup bool   `json:"subscription_allow_same_group"`
	AllowWallet    bool   `json:"subscription_allow_wallet"`
	WalletLimit    int64  `json:"subscription_wallet_limit"`
	CancelPlanned  bool   `json:"cancel_planned_subscription"`
	Reason         string `json:"reason"`
}

type BatchTokenSubscriptionBindingInput struct {
	SubscriptionId      int   `json:"subscription_id"`
	TokenIds            []int `json:"token_ids"`
	AllowRenewal        bool  `json:"subscription_allow_renewal"`
	AllowSameGroup      bool  `json:"subscription_allow_same_group"`
	AllowWallet         bool  `json:"subscription_allow_wallet"`
	WalletLimit         int64 `json:"subscription_wallet_limit"`
	KeepPlannedTokenIds []int `json:"keep_planned_token_ids"`
}

type SubscriptionTokenBindingItem struct {
	Id                           int    `json:"id"`
	Name                         string `json:"name"`
	Group                        string `json:"group"`
	Status                       int    `json:"status"`
	SubscriptionMode             string `json:"subscription_mode"`
	SubscriptionId               int    `json:"subscription_id"`
	SubscriptionAllowRenewal     bool   `json:"subscription_allow_renewal"`
	SubscriptionAllowSameGroup   bool   `json:"subscription_allow_same_group"`
	SubscriptionAllowWallet      bool   `json:"subscription_allow_wallet"`
	SubscriptionWalletLimit      int64  `json:"subscription_wallet_limit"`
	SubscriptionWalletUsed       int64  `json:"subscription_wallet_used"`
	PlannedSubscriptionId        int    `json:"planned_subscription_id"`
	PlannedSubscriptionEffective int64  `json:"planned_subscription_effective"`
	Compatible                   bool   `json:"compatible"`
	IncompatibilityReason        string `json:"incompatibility_reason"`
}

func NormalizeTokenSubscriptionMode(mode string) string {
	if strings.TrimSpace(mode) == TokenSubscriptionModeInstance {
		return TokenSubscriptionModeInstance
	}
	return TokenSubscriptionModeAuto
}

func tokenContinuationSummary(token *Token) string {
	if token == nil || NormalizeTokenSubscriptionMode(token.SubscriptionMode) != TokenSubscriptionModeInstance {
		return TokenSubscriptionModeAuto
	}
	parts := make([]string, 0, 3)
	if token.SubscriptionAllowRenewal {
		parts = append(parts, "renewal")
	}
	if token.SubscriptionAllowSameGroup {
		parts = append(parts, "same_group")
	}
	if token.SubscriptionAllowWallet {
		parts = append(parts, fmt.Sprintf("wallet:%d", token.SubscriptionWalletLimit))
	}
	if len(parts) == 0 {
		return "stop"
	}
	return strings.Join(parts, ",")
}

func validateSubscriptionForTokenTx(tx *gorm.DB, userId int, group string, subscriptionId int, requireUsable bool) (*UserSubscription, error) {
	if tx == nil {
		tx = DB
	}
	if userId <= 0 || subscriptionId <= 0 {
		return nil, ErrTokenSubscriptionNotFound
	}
	var sub UserSubscription
	if err := tx.Where("id = ? AND user_id = ?", subscriptionId, userId).First(&sub).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrTokenSubscriptionNotFound
		}
		return nil, err
	}
	if allowed := strings.TrimSpace(sub.AllowedGroup); allowed != "" && allowed != strings.TrimSpace(group) {
		return nil, ErrTokenSubscriptionGroupMismatch
	}
	if requireUsable {
		now := GetDBTimestamp()
		if sub.Status != "active" || sub.StartTime > now || sub.EndTime <= now {
			return nil, ErrTokenSubscriptionNotUsable
		}
		if sub.AmountTotal > 0 && sub.AmountUsed >= sub.AmountTotal {
			return nil, ErrTokenSubscriptionNotUsable
		}
		if sub.AmountCap > 0 && sub.AmountCapUsed >= sub.AmountCap {
			return nil, ErrTokenSubscriptionNotUsable
		}
	}
	return &sub, nil
}

func ValidateSubscriptionForToken(userId int, group string, subscriptionId int, requireUsable bool) (*UserSubscription, error) {
	return validateSubscriptionForTokenTx(DB, userId, group, subscriptionId, requireUsable)
}

func validateTokenBindingInputTx(tx *gorm.DB, userId int, group string, input TokenSubscriptionBindingInput) (*UserSubscription, error) {
	input.Mode = NormalizeTokenSubscriptionMode(input.Mode)
	if input.Mode == TokenSubscriptionModeAuto {
		return nil, nil
	}
	if input.SubscriptionId <= 0 {
		return nil, ErrTokenSubscriptionNotFound
	}
	if input.AllowWallet && input.WalletLimit <= 0 {
		return nil, errors.New("钱包接续必须设置大于 0 的上限")
	}
	return validateSubscriptionForTokenTx(tx, userId, group, input.SubscriptionId, true)
}

func ValidateTokenSubscriptionBindingInput(userId int, group string, input TokenSubscriptionBindingInput) error {
	_, err := validateTokenBindingInputTx(DB, userId, group, input)
	return err
}

// ResolveTokenFundingBindingForGroup enforces that API Key funding fields
// match the selected group type. Ordinary groups never retain subscription or
// virtual-membership bindings, package groups may use subscription allocation,
// and virtual-membership groups must bind one usable membership explicitly.
func ResolveTokenFundingBindingForGroup(
	userId int,
	group string,
	input TokenSubscriptionBindingInput,
	virtualMembershipId int,
) (TokenSubscriptionBindingInput, int, error) {
	group = strings.TrimSpace(group)
	virtualGroup, err := HasVirtualMembershipPlanByGroup(group)
	if err != nil {
		return input, virtualMembershipId, err
	}
	subscriptionGroup, err := HasSubscriptionPlanByGroup(group)
	if err != nil {
		return input, virtualMembershipId, err
	}
	if virtualGroup && subscriptionGroup {
		return input, virtualMembershipId, errors.New("当前分组同时属于套餐与虚拟会员，请联系管理员修正分组配置")
	}

	if virtualGroup {
		if virtualMembershipId <= 0 {
			return input, 0, errors.New("会员分组必须绑定可用的虚拟会员额度")
		}
		if NormalizeTokenSubscriptionMode(input.Mode) != TokenSubscriptionModeAuto || input.SubscriptionId > 0 {
			return input, 0, errors.New("虚拟会员不能与订阅实例同时绑定")
		}
		if err := ValidateVirtualMembershipForToken(userId, group, virtualMembershipId, true); err != nil {
			return input, 0, err
		}
		return TokenSubscriptionBindingInput{
			Mode:          TokenSubscriptionModeAuto,
			CancelPlanned: true,
			Reason:        input.Reason,
		}, virtualMembershipId, nil
	}

	if subscriptionGroup {
		if virtualMembershipId > 0 {
			return input, 0, errors.New("套餐分组不能绑定虚拟会员额度")
		}
		input.Mode = NormalizeTokenSubscriptionMode(input.Mode)
		if err := ValidateTokenSubscriptionBindingInput(userId, group, input); err != nil {
			return input, 0, err
		}
		return input, 0, nil
	}

	return TokenSubscriptionBindingInput{
		Mode:          TokenSubscriptionModeAuto,
		CancelPlanned: true,
		Reason:        input.Reason,
	}, 0, nil
}

func applyBindingInput(token *Token, input TokenSubscriptionBindingInput) {
	input.Mode = NormalizeTokenSubscriptionMode(input.Mode)
	// Subscription and virtual-membership ledgers are mutually exclusive. The
	// virtual-membership flow sets its id after applying this common mutation.
	token.VirtualMembershipId = 0
	token.SubscriptionMode = input.Mode
	if input.Mode == TokenSubscriptionModeAuto {
		token.SubscriptionId = 0
		token.SubscriptionAllowRenewal = false
		token.SubscriptionAllowSameGroup = false
		token.SubscriptionAllowWallet = false
		token.SubscriptionWalletLimit = 0
		token.SubscriptionWalletUsed = 0
		token.SubscriptionWalletCycleId = 0
		if input.CancelPlanned {
			token.PlannedSubscriptionId = 0
			token.PlannedSubscriptionGroup = ""
			token.PlannedSubscriptionEffective = 0
		}
		return
	}
	rebound := token.SubscriptionId != input.SubscriptionId
	token.SubscriptionId = input.SubscriptionId
	token.SubscriptionAllowRenewal = input.AllowRenewal
	token.SubscriptionAllowSameGroup = input.AllowSameGroup
	token.SubscriptionAllowWallet = input.AllowWallet
	token.SubscriptionWalletLimit = input.WalletLimit
	if rebound || token.SubscriptionWalletCycleId != input.SubscriptionId {
		token.SubscriptionWalletUsed = 0
		token.SubscriptionWalletCycleId = input.SubscriptionId
	}
	if !input.AllowWallet {
		token.SubscriptionWalletUsed = 0
	}
	if input.CancelPlanned {
		token.PlannedSubscriptionId = 0
		token.PlannedSubscriptionGroup = ""
		token.PlannedSubscriptionEffective = 0
	}
}

func ApplyTokenSubscriptionBindingInput(token *Token, input TokenSubscriptionBindingInput) {
	if token == nil {
		return
	}
	applyBindingInput(token, input)
}

func RecordInitialTokenSubscriptionBinding(token *Token, reason string) error {
	if token == nil || token.Id <= 0 ||
		NormalizeTokenSubscriptionMode(token.SubscriptionMode) != TokenSubscriptionModeInstance {
		return nil
	}
	before := *token
	before.SubscriptionMode = TokenSubscriptionModeAuto
	before.SubscriptionId = 0
	history := bindingHistoryFromTokens(
		&before,
		token,
		"user",
		TokenSubscriptionActionBind,
		reason,
	)
	return DB.Create(history).Error
}

func bindingHistoryFromTokens(before, after *Token, actor, action, reason string) *TokenSubscriptionBindingHistory {
	if before == nil || after == nil {
		return nil
	}
	return &TokenSubscriptionBindingHistory{
		UserId:              after.UserId,
		TokenId:             after.Id,
		ActorType:           strings.TrimSpace(actor),
		Action:              strings.TrimSpace(action),
		FromSubscriptionId:  before.SubscriptionId,
		ToSubscriptionId:    after.SubscriptionId,
		FromGroup:           before.Group,
		ToGroup:             after.Group,
		ContinuationSummary: tokenContinuationSummary(after),
		Reason:              strings.TrimSpace(reason),
		CreatedAt:           common.GetTimestamp(),
	}
}

func UpdateTokenSubscriptionBinding(userId, tokenId int, input TokenSubscriptionBindingInput, actor string) (*Token, error) {
	var updated Token
	var tokenKey string
	err := DB.Transaction(func(tx *gorm.DB) error {
		var token Token
		if err := tx.Set("gorm:query_option", "FOR UPDATE").
			Where("id = ? AND user_id = ?", tokenId, userId).First(&token).Error; err != nil {
			return err
		}
		before := token
		if _, err := validateTokenBindingInputTx(tx, userId, token.Group, input); err != nil {
			return err
		}
		applyBindingInput(&token, input)
		if err := tx.Model(&Token{}).Where("id = ? AND user_id = ?", token.Id, userId).
			Updates(map[string]any{
				"subscription_mode":              token.SubscriptionMode,
				"subscription_id":                token.SubscriptionId,
				"subscription_allow_renewal":     token.SubscriptionAllowRenewal,
				"subscription_allow_same_group":  token.SubscriptionAllowSameGroup,
				"subscription_allow_wallet":      token.SubscriptionAllowWallet,
				"subscription_wallet_limit":      token.SubscriptionWalletLimit,
				"subscription_wallet_used":       token.SubscriptionWalletUsed,
				"subscription_wallet_cycle_id":   token.SubscriptionWalletCycleId,
				"planned_subscription_id":        token.PlannedSubscriptionId,
				"planned_subscription_group":     token.PlannedSubscriptionGroup,
				"planned_subscription_effective": token.PlannedSubscriptionEffective,
				"virtual_membership_id":          token.VirtualMembershipId,
			}).Error; err != nil {
			return err
		}
		action := TokenSubscriptionActionRebind
		if NormalizeTokenSubscriptionMode(before.SubscriptionMode) == TokenSubscriptionModeAuto {
			action = TokenSubscriptionActionBind
		}
		if token.SubscriptionMode == TokenSubscriptionModeAuto {
			action = TokenSubscriptionActionUnbind
		}
		history := bindingHistoryFromTokens(&before, &token, actor, action, input.Reason)
		if history != nil {
			if err := tx.Create(history).Error; err != nil {
				return err
			}
		}
		updated = token
		tokenKey = token.Key
		return nil
	})
	if err != nil {
		return nil, err
	}
	if common.RedisEnabled && tokenKey != "" {
		_ = cacheDeleteToken(tokenKey)
	}
	return &updated, nil
}

// UpdateTokenWithSubscriptionBinding atomically applies the ordinary editable
// API Key fields and its subscription ownership. This prevents a failed
// binding write from leaving the Key group/quota changed without the user's
// selected ownership strategy.
func UpdateTokenWithSubscriptionBinding(userId int, desired *Token, input TokenSubscriptionBindingInput, actor string) (*Token, error) {
	if desired == nil || desired.Id <= 0 || userId <= 0 {
		return nil, errors.New("invalid token update args")
	}
	var updated Token
	var tokenKey string
	err := DB.Transaction(func(tx *gorm.DB) error {
		var current Token
		if err := tx.Set("gorm:query_option", "FOR UPDATE").
			Where("id = ? AND user_id = ?", desired.Id, userId).
			First(&current).Error; err != nil {
			return err
		}
		if _, err := validateTokenBindingInputTx(tx, userId, desired.Group, input); err != nil {
			return err
		}
		before := current
		current.Name = desired.Name
		current.Status = desired.Status
		current.ExpiredTime = desired.ExpiredTime
		current.RemainQuota = desired.RemainQuota
		current.UnlimitedQuota = desired.UnlimitedQuota
		current.ModelLimitsEnabled = desired.ModelLimitsEnabled
		current.ModelLimits = desired.ModelLimits
		current.AllowIps = desired.AllowIps
		current.Group = strings.TrimSpace(desired.Group)
		current.CrossGroupRetry = desired.CrossGroupRetry
		applyBindingInput(&current, input)
		current.VirtualMembershipId = desired.VirtualMembershipId
		if err := tx.Model(&Token{}).Where("id = ? AND user_id = ?", current.Id, userId).
			Updates(map[string]any{
				"name":                           current.Name,
				"status":                         current.Status,
				"expired_time":                   current.ExpiredTime,
				"remain_quota":                   current.RemainQuota,
				"unlimited_quota":                current.UnlimitedQuota,
				"model_limits_enabled":           current.ModelLimitsEnabled,
				"model_limits":                   current.ModelLimits,
				"allow_ips":                      current.AllowIps,
				"group":                          current.Group,
				"cross_group_retry":              current.CrossGroupRetry,
				"subscription_mode":              current.SubscriptionMode,
				"subscription_id":                current.SubscriptionId,
				"subscription_allow_renewal":     current.SubscriptionAllowRenewal,
				"subscription_allow_same_group":  current.SubscriptionAllowSameGroup,
				"subscription_allow_wallet":      current.SubscriptionAllowWallet,
				"subscription_wallet_limit":      current.SubscriptionWalletLimit,
				"subscription_wallet_used":       current.SubscriptionWalletUsed,
				"subscription_wallet_cycle_id":   current.SubscriptionWalletCycleId,
				"planned_subscription_id":        current.PlannedSubscriptionId,
				"planned_subscription_group":     current.PlannedSubscriptionGroup,
				"planned_subscription_effective": current.PlannedSubscriptionEffective,
				"virtual_membership_id":          current.VirtualMembershipId,
			}).Error; err != nil {
			return err
		}
		action := TokenSubscriptionActionRebind
		if NormalizeTokenSubscriptionMode(before.SubscriptionMode) == TokenSubscriptionModeAuto {
			action = TokenSubscriptionActionBind
		}
		if current.SubscriptionMode == TokenSubscriptionModeAuto {
			action = TokenSubscriptionActionUnbind
		}
		if history := bindingHistoryFromTokens(&before, &current, actor, action, input.Reason); history != nil {
			if err := tx.Create(history).Error; err != nil {
				return err
			}
		}
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

func ListTokenSubscriptionBindingHistory(userId, tokenId int) ([]TokenSubscriptionBindingHistory, error) {
	var history []TokenSubscriptionBindingHistory
	query := DB.Where("user_id = ?", userId)
	if tokenId > 0 {
		query = query.Where("token_id = ?", tokenId)
	}
	err := query.Order("id desc").Find(&history).Error
	return history, err
}

func ListUserTokensForSubscriptionBinding(userId, subscriptionId int) ([]SubscriptionTokenBindingItem, error) {
	var sub UserSubscription
	if err := DB.Where("id = ? AND user_id = ?", subscriptionId, userId).First(&sub).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrTokenSubscriptionNotFound
		}
		return nil, err
	}
	var tokens []Token
	if err := DB.Where("user_id = ?", userId).Order("id desc").Find(&tokens).Error; err != nil {
		return nil, err
	}
	items := make([]SubscriptionTokenBindingItem, 0, len(tokens))
	for _, token := range tokens {
		compatible := strings.TrimSpace(sub.AllowedGroup) == "" ||
			strings.TrimSpace(sub.AllowedGroup) == strings.TrimSpace(token.Group)
		reason := ""
		if !compatible {
			reason = fmt.Sprintf("当前 Key 分组为 %s，订阅实例仅支持 %s", token.Group, sub.AllowedGroup)
		}
		items = append(items, SubscriptionTokenBindingItem{
			Id:                           token.Id,
			Name:                         token.Name,
			Group:                        token.Group,
			Status:                       token.Status,
			SubscriptionMode:             NormalizeTokenSubscriptionMode(token.SubscriptionMode),
			SubscriptionId:               token.SubscriptionId,
			SubscriptionAllowRenewal:     token.SubscriptionAllowRenewal,
			SubscriptionAllowSameGroup:   token.SubscriptionAllowSameGroup,
			SubscriptionAllowWallet:      token.SubscriptionAllowWallet,
			SubscriptionWalletLimit:      token.SubscriptionWalletLimit,
			SubscriptionWalletUsed:       token.SubscriptionWalletUsed,
			PlannedSubscriptionId:        token.PlannedSubscriptionId,
			PlannedSubscriptionEffective: token.PlannedSubscriptionEffective,
			Compatible:                   compatible,
			IncompatibilityReason:        reason,
		})
	}
	return items, nil
}

func ReplaceSubscriptionTokenBindings(userId int, input BatchTokenSubscriptionBindingInput) error {
	if userId <= 0 || input.SubscriptionId <= 0 {
		return errors.New("invalid binding args")
	}
	selected := make(map[int]struct{}, len(input.TokenIds))
	for _, tokenId := range input.TokenIds {
		if tokenId > 0 {
			selected[tokenId] = struct{}{}
		}
	}
	keepPlanned := make(map[int]struct{}, len(input.KeepPlannedTokenIds))
	for _, tokenId := range input.KeepPlannedTokenIds {
		if tokenId > 0 {
			keepPlanned[tokenId] = struct{}{}
		}
	}
	var tokenKeys []string
	err := DB.Transaction(func(tx *gorm.DB) error {
		var sub UserSubscription
		if err := tx.Set("gorm:query_option", "FOR UPDATE").
			Where("id = ? AND user_id = ?", input.SubscriptionId, userId).
			First(&sub).Error; err != nil {
			return err
		}
		now := GetDBTimestamp()
		if sub.Status != "active" || sub.StartTime > now || sub.EndTime <= now {
			return ErrTokenSubscriptionNotUsable
		}
		var affected []Token
		query := tx.Set("gorm:query_option", "FOR UPDATE").
			Where("user_id = ? AND subscription_id = ?", userId, input.SubscriptionId)
		if len(selected) > 0 {
			ids := make([]int, 0, len(selected))
			for tokenId := range selected {
				ids = append(ids, tokenId)
			}
			query = query.Or("user_id = ? AND id IN ?", userId, ids)
		}
		if err := query.Find(&affected).Error; err != nil {
			return err
		}
		foundSelected := 0
		for _, token := range affected {
			_, shouldBind := selected[token.Id]
			if shouldBind {
				foundSelected++
				if allowed := strings.TrimSpace(sub.AllowedGroup); allowed != "" && allowed != strings.TrimSpace(token.Group) {
					return ErrTokenSubscriptionGroupMismatch
				}
				if input.AllowWallet && input.WalletLimit <= 0 {
					return errors.New("钱包接续必须设置大于 0 的上限")
				}
			}
		}
		if foundSelected != len(selected) {
			return errors.New("one or more API keys were not found")
		}
		for i := range affected {
			token := affected[i]
			before := token
			_, shouldBind := selected[token.Id]
			action := TokenSubscriptionActionRebind
			if shouldBind {
				applyBindingInput(&token, TokenSubscriptionBindingInput{
					Mode:           TokenSubscriptionModeInstance,
					SubscriptionId: input.SubscriptionId,
					AllowRenewal:   input.AllowRenewal,
					AllowSameGroup: input.AllowSameGroup,
					AllowWallet:    input.AllowWallet,
					WalletLimit:    input.WalletLimit,
				})
				if NormalizeTokenSubscriptionMode(before.SubscriptionMode) == TokenSubscriptionModeAuto {
					action = TokenSubscriptionActionBind
				}
			} else {
				_, keep := keepPlanned[token.Id]
				applyBindingInput(&token, TokenSubscriptionBindingInput{
					Mode:          TokenSubscriptionModeAuto,
					CancelPlanned: !keep,
				})
				action = TokenSubscriptionActionUnbind
			}
			if err := tx.Model(&Token{}).Where("id = ? AND user_id = ?", token.Id, userId).
				Updates(map[string]any{
					"subscription_mode":              token.SubscriptionMode,
					"subscription_id":                token.SubscriptionId,
					"subscription_allow_renewal":     token.SubscriptionAllowRenewal,
					"subscription_allow_same_group":  token.SubscriptionAllowSameGroup,
					"subscription_allow_wallet":      token.SubscriptionAllowWallet,
					"subscription_wallet_limit":      token.SubscriptionWalletLimit,
					"subscription_wallet_used":       token.SubscriptionWalletUsed,
					"subscription_wallet_cycle_id":   token.SubscriptionWalletCycleId,
					"planned_subscription_id":        token.PlannedSubscriptionId,
					"planned_subscription_group":     token.PlannedSubscriptionGroup,
					"planned_subscription_effective": token.PlannedSubscriptionEffective,
				}).Error; err != nil {
				return err
			}
			history := bindingHistoryFromTokens(
				&before,
				&token,
				"user",
				action,
				"subscription card batch management",
			)
			if err := tx.Create(history).Error; err != nil {
				return err
			}
			if token.Key != "" {
				tokenKeys = append(tokenKeys, token.Key)
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	if common.RedisEnabled {
		for _, tokenKey := range tokenKeys {
			_ = cacheDeleteToken(tokenKey)
		}
	}
	return nil
}

func UpdateUserSubscriptionRemark(userId, subscriptionId int, remark string) (*UserSubscription, error) {
	remark = strings.TrimSpace(remark)
	if len([]rune(remark)) > 128 {
		return nil, errors.New("订阅实例备注不能超过 128 个字符")
	}
	var sub UserSubscription
	err := DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Set("gorm:query_option", "FOR UPDATE").
			Where("id = ? AND user_id = ?", subscriptionId, userId).
			First(&sub).Error; err != nil {
			return err
		}
		sub.Remark = remark
		return tx.Model(&UserSubscription{}).Where("id = ?", sub.Id).
			Update("remark", remark).Error
	})
	if err != nil {
		return nil, err
	}
	return &sub, nil
}

// ApplyDueTokenSubscriptionTransition switches group and binding together when
// a paid renewal successor becomes effective. It is safe to call on every
// authenticated request and is a no-op until the scheduled time.
func ApplyDueTokenSubscriptionTransition(token *Token) (bool, error) {
	if token == nil || token.Id <= 0 || token.PlannedSubscriptionId <= 0 ||
		token.PlannedSubscriptionEffective <= 0 || token.PlannedSubscriptionEffective > GetDBTimestamp() {
		return false, nil
	}
	var updated Token
	err := DB.Transaction(func(tx *gorm.DB) error {
		var current Token
		if err := tx.Set("gorm:query_option", "FOR UPDATE").Where("id = ?", token.Id).First(&current).Error; err != nil {
			return err
		}
		now := GetDBTimestamp()
		if current.PlannedSubscriptionId <= 0 || current.PlannedSubscriptionEffective > now {
			updated = current
			return nil
		}
		targetGroup := strings.TrimSpace(current.PlannedSubscriptionGroup)
		if _, err := validateSubscriptionForTokenTx(tx, current.UserId, targetGroup, current.PlannedSubscriptionId, true); err != nil {
			return err
		}
		before := current
		current.Group = targetGroup
		current.SubscriptionMode = TokenSubscriptionModeInstance
		current.SubscriptionId = current.PlannedSubscriptionId
		current.SubscriptionWalletCycleId = current.SubscriptionId
		current.SubscriptionWalletUsed = 0
		current.PlannedSubscriptionId = 0
		current.PlannedSubscriptionGroup = ""
		current.PlannedSubscriptionEffective = 0
		if err := tx.Model(&Token{}).Where("id = ?", current.Id).Updates(map[string]any{
			"group":                          current.Group,
			"subscription_mode":              current.SubscriptionMode,
			"subscription_id":                current.SubscriptionId,
			"subscription_wallet_cycle_id":   current.SubscriptionWalletCycleId,
			"subscription_wallet_used":       current.SubscriptionWalletUsed,
			"planned_subscription_id":        0,
			"planned_subscription_group":     "",
			"planned_subscription_effective": 0,
		}).Error; err != nil {
			return err
		}
		history := bindingHistoryFromTokens(&before, &current, "system", TokenSubscriptionActionRenewalActivated, "renewal successor became active")
		if err := tx.Create(history).Error; err != nil {
			return err
		}
		updated = current
		return nil
	})
	if err != nil {
		return false, err
	}
	*token = updated
	if common.RedisEnabled && token.Key != "" {
		_ = cacheSetToken(*token)
	}
	return true, nil
}

func adjustTokenWalletFallbackUsed(tokenId, userId int, delta int64) (int64, error) {
	if tokenId <= 0 || userId <= 0 || delta == 0 {
		return 0, nil
	}
	var used int64
	err := DB.Transaction(func(tx *gorm.DB) error {
		var token Token
		if err := tx.Set("gorm:query_option", "FOR UPDATE").
			Where("id = ? AND user_id = ?", tokenId, userId).First(&token).Error; err != nil {
			return err
		}
		if NormalizeTokenSubscriptionMode(token.SubscriptionMode) != TokenSubscriptionModeInstance ||
			!token.SubscriptionAllowWallet || token.SubscriptionWalletLimit <= 0 {
			return ErrTokenWalletFallbackDisabled
		}
		next := token.SubscriptionWalletUsed + delta
		if next < 0 {
			next = 0
		}
		if next > token.SubscriptionWalletLimit {
			return ErrTokenWalletFallbackLimitReached
		}
		if err := tx.Model(&Token{}).Where("id = ?", token.Id).
			Update("subscription_wallet_used", next).Error; err != nil {
			return err
		}
		used = next
		return nil
	})
	if err != nil {
		return 0, err
	}
	if common.RedisEnabled {
		if token, tokenErr := GetTokenById(tokenId); tokenErr == nil && token.Key != "" {
			_ = cacheSetToken(*token)
		}
	}
	return used, nil
}

func ReserveTokenWalletFallback(tokenId, userId int, amount int64) (int64, error) {
	if amount <= 0 {
		return 0, nil
	}
	return adjustTokenWalletFallbackUsed(tokenId, userId, amount)
}

func ReleaseTokenWalletFallback(tokenId, userId int, amount int64) (int64, error) {
	if amount <= 0 {
		return 0, nil
	}
	return adjustTokenWalletFallbackUsed(tokenId, userId, -amount)
}
