package model

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	LuckyCampaignCode = "lucky-wheel"

	LuckyPoolSubscription = "subscription"
	LuckyPoolRecharge     = "recharge"

	LuckyCardAvailable      = "available"
	LuckyCardConsumed       = "consumed"
	LuckyCardExpired        = "expired"
	LuckyCardRevoked        = "revoked"
	LuckyCardReviewRequired = "review_required"

	LuckyPrizeQuota5    = "quota_5"
	LuckyPrizeQuota10   = "quota_10"
	LuckyPrizeQuota20   = "quota_20"
	LuckyPrizeQuota30   = "quota_30"
	LuckyPrizeQuota50   = "quota_50"
	LuckyPrizeQuota100  = "quota_100"
	LuckyPrizeGift5     = "gift_5"
	LuckyPrizeGift10    = "gift_10"
	LuckyPrizeGift20    = "gift_20"
	LuckyPrizeDouble    = "subscription_double"
	LuckyPrizeFullReset = "subscription_full_reset"
	LuckyPrizeCrazy5H   = "crazy_5h"

	LuckyWeightScale int64 = 1_000_000
)

type LuckyCampaign struct {
	Id                 int64  `json:"id"`
	Code               string `json:"code" gorm:"type:varchar(64);uniqueIndex;not null"`
	Name               string `json:"name" gorm:"type:varchar(128);not null"`
	ActiveRuleSetId    int64  `json:"active_rule_set_id" gorm:"index;not null;default:0"`
	IssuancePaused     bool   `json:"issuance_paused" gorm:"not null;default:true"`
	DrawPaused         bool   `json:"draw_paused" gorm:"not null;default:true"`
	DrawPauseStartedAt int64  `json:"draw_pause_started_at" gorm:"not null;default:0"`
	SettingsVersion    int64  `json:"settings_version" gorm:"not null;default:1"`
	CreatedAt          int64  `json:"created_at" gorm:"not null"`
	UpdatedAt          int64  `json:"updated_at" gorm:"not null"`
}

func (m *LuckyCampaign) BeforeCreate(tx *gorm.DB) error {
	now := common.GetTimestamp()
	m.CreatedAt, m.UpdatedAt = now, now
	return nil
}

func (m *LuckyCampaign) BeforeUpdate(tx *gorm.DB) error {
	m.UpdatedAt = common.GetTimestamp()
	return nil
}

type LuckyPrizeConfig struct {
	Code             string `json:"code"`
	DisplayUsdMicros int64  `json:"display_usd_micros"`
	Weight           int64  `json:"weight"`
}

type LuckyRuleSet struct {
	Id                         int64  `json:"id"`
	CampaignId                 int64  `json:"campaign_id" gorm:"uniqueIndex:idx_lucky_rule_version,priority:1;index"`
	Version                    int    `json:"version" gorm:"uniqueIndex:idx_lucky_rule_version,priority:2"`
	Status                     string `json:"status" gorm:"type:varchar(32);index"`
	SubscriptionPool           string `json:"subscription_pool" gorm:"type:text;not null"`
	RechargePool               string `json:"recharge_pool" gorm:"type:text;not null"`
	ThresholdConfig            string `json:"threshold_config" gorm:"type:text;not null"`
	RechargeBonusUsdMicros     int64  `json:"recharge_bonus_usd_micros" gorm:"not null;default:40000000"`
	RechargeCardValidSeconds   int64  `json:"recharge_card_valid_seconds" gorm:"not null;default:2592000"`
	RechargeRewardValidSeconds int64  `json:"recharge_reward_valid_seconds" gorm:"not null;default:2592000"`
	CrazyCardValidSeconds      int64  `json:"crazy_card_valid_seconds" gorm:"not null;default:18000"`
	CrazyCardQuotaUsdMicros    int64  `json:"crazy_card_quota_usd_micros" gorm:"not null;default:600000000"`
	ActivityGroup              string `json:"activity_group" gorm:"type:varchar(64);not null;default:'套餐专用分组'"`
	Checksum                   string `json:"checksum" gorm:"type:char(64);not null"`
	PublishedAt                int64  `json:"published_at"`
	EffectiveAt                int64  `json:"effective_at"`
	CreatedBy                  int    `json:"created_by"`
	CreatedAt                  int64  `json:"created_at"`
}

func (m *LuckyRuleSet) BeforeCreate(tx *gorm.DB) error {
	if m.CreatedAt == 0 {
		m.CreatedAt = common.GetTimestamp()
	}
	return nil
}

type LuckyCard struct {
	Id                     int64  `json:"id"`
	UserId                 int    `json:"user_id" gorm:"index;index:idx_lucky_card_available,priority:1"`
	CampaignId             int64  `json:"campaign_id" gorm:"index"`
	RuleSetId              int64  `json:"rule_set_id" gorm:"index"`
	PoolType               string `json:"pool_type" gorm:"type:varchar(32);not null"`
	SourceType             string `json:"source_type" gorm:"type:varchar(32);not null"`
	SourceRef              string `json:"source_ref" gorm:"type:varchar(255);not null"`
	SourceOrderId          int    `json:"source_order_id" gorm:"index"`
	SourceSubscriptionId   int    `json:"source_subscription_id" gorm:"index"`
	SourceCycleKey         string `json:"source_cycle_key" gorm:"type:varchar(128)"`
	SourceSnapshot         string `json:"source_snapshot" gorm:"type:text"`
	SourceEffectiveEndTime int64  `json:"source_effective_end_time"`
	GrantKey               string `json:"grant_key" gorm:"type:varchar(255);uniqueIndex;not null"`
	Status                 string `json:"status" gorm:"type:varchar(32);index;index:idx_lucky_card_available,priority:2"`
	IssuedAt               int64  `json:"issued_at"`
	ExpiresAt              int64  `json:"expires_at" gorm:"index;index:idx_lucky_card_available,priority:3"`
	PauseExtensionSeconds  int64  `json:"pause_extension_seconds"`
	ConsumedAt             int64  `json:"consumed_at"`
	RevokedAt              int64  `json:"revoked_at"`
	RevokeReason           string `json:"revoke_reason" gorm:"type:varchar(255)"`
	CreatedAt              int64  `json:"created_at"`
	UpdatedAt              int64  `json:"updated_at"`
}

func (m *LuckyCard) BeforeCreate(tx *gorm.DB) error {
	now := common.GetTimestamp()
	m.CreatedAt, m.UpdatedAt = now, now
	if m.IssuedAt == 0 {
		m.IssuedAt = now
	}
	if m.Status == "" {
		m.Status = LuckyCardAvailable
	}
	return nil
}

func (m *LuckyCard) BeforeUpdate(tx *gorm.DB) error {
	m.UpdatedAt = common.GetTimestamp()
	return nil
}

type LuckyDraw struct {
	Id                   int64  `json:"id"`
	UserId               int    `json:"user_id" gorm:"index;uniqueIndex:idx_lucky_draw_idempotency,priority:1"`
	CardId               int64  `json:"card_id" gorm:"uniqueIndex"`
	RuleSetId            int64  `json:"rule_set_id" gorm:"index"`
	IdempotencyKey       string `json:"idempotency_key" gorm:"type:varchar(64);uniqueIndex:idx_lucky_draw_idempotency,priority:2"`
	RandomValue          int64  `json:"random_value"`
	PrizeType            string `json:"prize_type" gorm:"type:varchar(32)"`
	DisplayUsdMicros     int64  `json:"display_usd_micros"`
	ActualUsdMicros      int64  `json:"actual_usd_micros"`
	AwardedQuota         int64  `json:"awarded_quota"`
	RewardSubscriptionId int    `json:"reward_subscription_id" gorm:"index"`
	GiftQuotaAwarded     int64  `json:"gift_quota_awarded"`
	RuleChecksum         string `json:"rule_checksum" gorm:"type:char(64)"`
	Status               string `json:"status" gorm:"type:varchar(32);index"`
	AwardedAt            int64  `json:"awarded_at"`
	CreatedAt            int64  `json:"created_at"`
	UpdatedAt            int64  `json:"updated_at"`
}

func (m *LuckyDraw) BeforeCreate(tx *gorm.DB) error {
	now := common.GetTimestamp()
	m.CreatedAt, m.UpdatedAt = now, now
	return nil
}

type LuckyAdminDrawFilter struct {
	Keyword   string
	UserId    int
	PrizeType string
	Status    string
	StartTime int64
	EndTime   int64
	Page      int
	PageSize  int
}

type LuckyAdminDrawRecord struct {
	Id                     int64  `json:"id"`
	UserId                 int    `json:"user_id"`
	Username               string `json:"username"`
	DisplayName            string `json:"display_name"`
	CardId                 int64  `json:"card_id"`
	RuleSetId              int64  `json:"rule_set_id"`
	PrizeType              string `json:"prize_type"`
	DisplayUsdMicros       int64  `json:"display_usd_micros"`
	ActualUsdMicros        int64  `json:"actual_usd_micros"`
	AwardedQuota           int64  `json:"awarded_quota"`
	RewardSubscriptionId   int    `json:"reward_subscription_id"`
	GiftQuotaAwarded       int64  `json:"gift_quota_awarded"`
	Status                 string `json:"status"`
	AwardedAt              int64  `json:"awarded_at"`
	CreatedAt              int64  `json:"created_at"`
	PoolType               string `json:"pool_type"`
	SourceType             string `json:"source_type"`
	SourceRef              string `json:"source_ref"`
	SourceOrderId          int    `json:"source_order_id"`
	SourceSubscriptionId   int    `json:"source_subscription_id"`
	SourceCycleKey         string `json:"source_cycle_key"`
	SourceEffectiveEndTime int64  `json:"source_effective_end_time"`
	CardIssuedAt           int64  `json:"card_issued_at"`
	CardExpiresAt          int64  `json:"card_expires_at"`
}

func ListLuckyAdminDraws(filter LuckyAdminDrawFilter) ([]LuckyAdminDrawRecord, int64, error) {
	if filter.Page < 1 {
		filter.Page = 1
	}
	if filter.PageSize < 1 || filter.PageSize > 100 {
		filter.PageSize = 20
	}

	buildQuery := func() *gorm.DB {
		query := DB.Table("lucky_draws AS ld").
			Joins("LEFT JOIN users AS u ON u.id = ld.user_id").
			Joins("LEFT JOIN lucky_cards AS lc ON lc.id = ld.card_id")
		if filter.UserId > 0 {
			query = query.Where("ld.user_id = ?", filter.UserId)
		}
		if keyword := strings.TrimSpace(filter.Keyword); keyword != "" {
			like := "%" + keyword + "%"
			if numericUserId, err := strconv.Atoi(keyword); err == nil && numericUserId > 0 {
				query = query.Where(
					"(ld.user_id = ? OR u.username LIKE ? OR u.display_name LIKE ?)",
					numericUserId, like, like,
				)
			} else {
				query = query.Where("(u.username LIKE ? OR u.display_name LIKE ?)", like, like)
			}
		}
		if filter.PrizeType != "" {
			query = query.Where("ld.prize_type = ?", filter.PrizeType)
		}
		if filter.Status != "" {
			query = query.Where("ld.status = ?", filter.Status)
		}
		if filter.StartTime > 0 {
			query = query.Where("ld.awarded_at >= ?", filter.StartTime)
		}
		if filter.EndTime > 0 {
			query = query.Where("ld.awarded_at <= ?", filter.EndTime)
		}
		return query
	}

	var total int64
	if err := buildQuery().Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var records []LuckyAdminDrawRecord
	err := buildQuery().
		Select(`
			ld.id, ld.user_id, u.username, u.display_name,
			ld.card_id, ld.rule_set_id, ld.prize_type,
			ld.display_usd_micros, ld.actual_usd_micros, ld.awarded_quota,
			ld.reward_subscription_id, ld.gift_quota_awarded,
			ld.status, ld.awarded_at, ld.created_at,
			lc.pool_type, lc.source_type, lc.source_ref,
			lc.source_order_id, lc.source_subscription_id, lc.source_cycle_key,
			lc.source_effective_end_time,
			lc.issued_at AS card_issued_at, lc.expires_at AS card_expires_at
		`).
		Order("ld.id DESC").
		Limit(filter.PageSize).
		Offset((filter.Page - 1) * filter.PageSize).
		Scan(&records).Error
	if err != nil {
		return nil, 0, err
	}
	return records, total, nil
}

type LuckyRechargeEvent struct {
	Id          int64  `json:"id"`
	UserId      int    `json:"user_id" gorm:"index"`
	SourceType  string `json:"source_type" gorm:"type:varchar(32);uniqueIndex:idx_lucky_recharge_source,priority:1"`
	SourceRef   string `json:"source_ref" gorm:"type:varchar(255);uniqueIndex:idx_lucky_recharge_source,priority:2"`
	Direction   int    `json:"direction" gorm:"uniqueIndex:idx_lucky_recharge_source,priority:3"`
	AmountCents int64  `json:"amount_cents"`
	RuleSetId   int64  `json:"rule_set_id"`
	OccurredAt  int64  `json:"occurred_at"`
	CreatedAt   int64  `json:"created_at"`
}

type LuckyRechargeProgress struct {
	UserId              int   `json:"user_id" gorm:"primaryKey"`
	EligibleCents       int64 `json:"eligible_cents"`
	HighestAwardedStage int64 `json:"highest_awarded_stage"`
	NextThresholdCents  int64 `json:"next_threshold_cents"`
	UpdatedAt           int64 `json:"updated_at"`
}

type LuckySourceReversalResult struct {
	EventCreated bool  `json:"event_created"`
	RevokedCards int64 `json:"revoked_cards"`
	ReviewCards  int64 `json:"review_cards"`
	ReviewDraws  int64 `json:"review_draws"`
}

type LuckyRewardBucket struct {
	Id                   int64 `json:"id"`
	UserId               int   `json:"user_id" gorm:"uniqueIndex:idx_lucky_reward_bucket,priority:1"`
	SourceSubscriptionId int   `json:"source_subscription_id" gorm:"uniqueIndex:idx_lucky_reward_bucket,priority:2;index"`
	EffectiveEndTime     int64 `json:"effective_end_time" gorm:"uniqueIndex:idx_lucky_reward_bucket,priority:3"`
	RewardSubscriptionId int   `json:"reward_subscription_id" gorm:"uniqueIndex"`
	CreatedAt            int64 `json:"created_at"`
	UpdatedAt            int64 `json:"updated_at"`
}

type LuckyPausePeriod struct {
	Id              int64  `json:"id"`
	CampaignId      int64  `json:"campaign_id" gorm:"index"`
	PauseType       string `json:"pause_type" gorm:"type:varchar(16);index"`
	StartedAt       int64  `json:"started_at"`
	EndedAt         int64  `json:"ended_at"`
	DurationSeconds int64  `json:"duration_seconds"`
	Reason          string `json:"reason" gorm:"type:varchar(255)"`
	OperatorId      int    `json:"operator_id"`
	AffectedCards   int64  `json:"affected_cards"`
	Status          string `json:"status" gorm:"type:varchar(16);index"`
	CreatedAt       int64  `json:"created_at"`
	UpdatedAt       int64  `json:"updated_at"`
}

type SubscriptionConsumptionPriority struct {
	Id             int64  `json:"id"`
	UserId         int    `json:"user_id" gorm:"uniqueIndex:idx_sub_priority_instance,priority:1;uniqueIndex:idx_sub_priority_position,priority:1"`
	GroupName      string `json:"group_name" gorm:"type:varchar(64);uniqueIndex:idx_sub_priority_instance,priority:2;uniqueIndex:idx_sub_priority_position,priority:2"`
	SubscriptionId int    `json:"subscription_id" gorm:"uniqueIndex:idx_sub_priority_instance,priority:3;index"`
	Priority       int    `json:"priority" gorm:"uniqueIndex:idx_sub_priority_position,priority:3"`
	Revision       int64  `json:"revision"`
	UpdatedAt      int64  `json:"updated_at"`
}

func defaultLuckyPools() ([]LuckyPrizeConfig, []LuckyPrizeConfig) {
	subscription := []LuckyPrizeConfig{
		{LuckyPrizeQuota10, 10_000_000, 430_000},
		{LuckyPrizeQuota20, 20_000_000, 260_000},
		{LuckyPrizeQuota30, 30_000_000, 200_000},
		{LuckyPrizeQuota50, 50_000_000, 30_000},
		{LuckyPrizeQuota100, 100_000_000, 10_000},
		{LuckyPrizeGift5, 5_000_000, 47_000},
		{LuckyPrizeGift10, 10_000_000, 20_000},
		{LuckyPrizeDouble, 0, 250},
		{LuckyPrizeFullReset, 0, 1_500},
		{LuckyPrizeCrazy5H, 0, 1_250},
	}
	recharge := []LuckyPrizeConfig{
		{LuckyPrizeQuota5, 5_000_000, 360_000},
		{LuckyPrizeQuota10, 10_000_000, 320_000},
		{LuckyPrizeQuota20, 20_000_000, 200_000},
		{LuckyPrizeQuota50, 50_000_000, 30_000},
		{LuckyPrizeQuota100, 100_000_000, 10_000},
		{LuckyPrizeGift5, 5_000_000, 48_750},
		{LuckyPrizeGift10, 10_000_000, 20_000},
		{LuckyPrizeGift20, 20_000_000, 10_000},
		{LuckyPrizeCrazy5H, 0, 1_250},
	}
	return subscription, recharge
}

func validateLuckyPool(pool []LuckyPrizeConfig, allowSubscriptionOnly bool) error {
	var total int64
	seen := make(map[string]struct{}, len(pool))
	for _, prize := range pool {
		if strings.TrimSpace(prize.Code) == "" || prize.Weight <= 0 {
			return errors.New("invalid lucky prize")
		}
		if _, ok := seen[prize.Code]; ok {
			return fmt.Errorf("duplicate lucky prize: %s", prize.Code)
		}
		if !allowSubscriptionOnly && (prize.Code == LuckyPrizeDouble || prize.Code == LuckyPrizeFullReset) {
			return fmt.Errorf("recharge pool contains subscription-only prize: %s", prize.Code)
		}
		seen[prize.Code] = struct{}{}
		total += prize.Weight
	}
	if total != LuckyWeightScale {
		return fmt.Errorf("lucky prize weights must total %d, got %d", LuckyWeightScale, total)
	}
	return nil
}

func ParseLuckyPool(raw string) ([]LuckyPrizeConfig, error) {
	var pool []LuckyPrizeConfig
	if err := common.UnmarshalJsonStr(raw, &pool); err != nil {
		return nil, err
	}
	return pool, validateLuckyPool(pool, true)
}

func ValidateLuckyRuleSet(rule *LuckyRuleSet) error {
	if rule == nil {
		return errors.New("lucky rule is nil")
	}
	subscription, err := ParseLuckyPool(rule.SubscriptionPool)
	if err != nil {
		return fmt.Errorf("invalid subscription pool: %w", err)
	}
	if err := validateLuckyPool(subscription, true); err != nil {
		return err
	}
	var recharge []LuckyPrizeConfig
	if err := common.UnmarshalJsonStr(rule.RechargePool, &recharge); err != nil {
		return fmt.Errorf("invalid recharge pool: %w", err)
	}
	if err := validateLuckyPool(recharge, false); err != nil {
		return err
	}
	if rule.RechargeBonusUsdMicros < 0 || rule.RechargeCardValidSeconds <= 0 ||
		rule.RechargeRewardValidSeconds <= 0 || rule.CrazyCardValidSeconds <= 0 ||
		rule.CrazyCardQuotaUsdMicros <= 0 || strings.TrimSpace(rule.ActivityGroup) == "" {
		return errors.New("invalid lucky rule amounts, durations, or activity group")
	}
	return nil
}

func RefreshLuckyRuleChecksum(rule *LuckyRuleSet) {
	if rule == nil {
		return
	}
	payload := fmt.Sprintf("%s|%s|%s|%d|%d|%d|%d|%d|%s",
		rule.SubscriptionPool, rule.RechargePool, rule.ThresholdConfig,
		rule.RechargeBonusUsdMicros, rule.RechargeCardValidSeconds,
		rule.RechargeRewardValidSeconds, rule.CrazyCardValidSeconds,
		rule.CrazyCardQuotaUsdMicros, strings.TrimSpace(rule.ActivityGroup))
	sum := sha256.Sum256([]byte(payload))
	rule.Checksum = fmt.Sprintf("%x", sum)
}

func LuckyThresholdCents(stage int64) int64 {
	switch stage {
	case 1:
		return 5_000
	case 2:
		return 10_000
	case 3:
		return 20_000
	case 4:
		return 40_000
	case 5:
		return 60_000
	case 6:
		return 80_000
	default:
		if stage < 1 {
			return 5_000
		}
		return 80_000 + (stage-6)*20_000
	}
}

func EnsureDefaultLuckyCampaign() error {
	return DB.Transaction(func(tx *gorm.DB) error {
		var campaign LuckyCampaign
		err := tx.Where("code = ?", LuckyCampaignCode).First(&campaign).Error
		if err == nil {
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		campaign = LuckyCampaign{
			Code: LuckyCampaignCode, Name: "幸运大转盘",
			IssuancePaused: true, DrawPaused: true, SettingsVersion: 1,
		}
		if err := tx.Create(&campaign).Error; err != nil {
			return err
		}
		subscriptionPool, rechargePool := defaultLuckyPools()
		if err := validateLuckyPool(subscriptionPool, true); err != nil {
			return err
		}
		if err := validateLuckyPool(rechargePool, false); err != nil {
			return err
		}
		subscriptionJSON, err := common.Marshal(subscriptionPool)
		if err != nil {
			return err
		}
		rechargeJSON, err := common.Marshal(rechargePool)
		if err != nil {
			return err
		}
		thresholdJSON, err := common.Marshal([]int64{5_000, 10_000, 20_000, 40_000, 60_000, 80_000})
		if err != nil {
			return err
		}
		checksumBytes := sha256.Sum256([]byte(string(subscriptionJSON) + "|" + string(rechargeJSON)))
		checksum := fmt.Sprintf("%x", checksumBytes)
		rule := LuckyRuleSet{
			CampaignId: campaign.Id, Version: 1, Status: "active",
			SubscriptionPool: string(subscriptionJSON), RechargePool: string(rechargeJSON),
			ThresholdConfig: string(thresholdJSON), RechargeBonusUsdMicros: 40_000_000,
			RechargeCardValidSeconds: 30 * 24 * 3600, RechargeRewardValidSeconds: 30 * 24 * 3600,
			CrazyCardValidSeconds: 5 * 3600, CrazyCardQuotaUsdMicros: 600_000_000,
			ActivityGroup: "套餐专用分组", Checksum: checksum,
			PublishedAt: common.GetTimestamp(), EffectiveAt: common.GetTimestamp(),
		}
		if err := tx.Create(&rule).Error; err != nil {
			return err
		}
		return tx.Model(&campaign).Updates(map[string]interface{}{
			"active_rule_set_id": rule.Id,
			"updated_at":         common.GetTimestamp(),
		}).Error
	})
}

const luckyRechargeBonusFortyMigrationKey = "LuckyRechargeBonusFortyMigratedV1"

// EnsureLuckyRechargeBonusForty 把本次策略调整前仍为 +$60 的历史规则一次性迁移为
// +$40。迁移标记避免管理员未来主动创建其它加成时被启动流程反复覆盖。
func EnsureLuckyRechargeBonusForty() error {
	return DB.Transaction(func(tx *gorm.DB) error {
		var marker Option
		if err := tx.Where(commonKeyCol+" = ?", luckyRechargeBonusFortyMigrationKey).First(&marker).Error; err == nil {
			return nil
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		var rules []LuckyRuleSet
		if err := tx.Where("recharge_bonus_usd_micros = ?", int64(60_000_000)).Find(&rules).Error; err != nil {
			return err
		}
		for i := range rules {
			rules[i].RechargeBonusUsdMicros = 40_000_000
			RefreshLuckyRuleChecksum(&rules[i])
			if err := tx.Model(&LuckyRuleSet{}).Where("id = ?", rules[i].Id).Updates(map[string]interface{}{
				"recharge_bonus_usd_micros": rules[i].RechargeBonusUsdMicros,
				"checksum":                  rules[i].Checksum,
			}).Error; err != nil {
				return err
			}
		}
		return tx.Create(&Option{Key: luckyRechargeBonusFortyMigrationKey, Value: "true"}).Error
	})
}

const luckyPrizeProbability20260811MigrationKey = "LuckyPrizeProbability20260811MigratedV1"

// EnsureLuckyPrizeProbability20260811 publishes the owner-selected
// subscription-card probability table as a new immutable rule version. Cards
// already issued keep their original RuleSetId, while newly eligible orders
// and newly issued cards use the new active rule. The recharge-card pool is
// intentionally copied unchanged because it cannot award subscription-only
// double/reset prizes.
func EnsureLuckyPrizeProbability20260811() error {
	return DB.Transaction(func(tx *gorm.DB) error {
		var marker Option
		if err := tx.Where(commonKeyCol+" = ?", luckyPrizeProbability20260811MigrationKey).First(&marker).Error; err == nil {
			return nil
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		campaign, active, err := GetLuckyCampaignTx(tx, true)
		if err != nil {
			return err
		}
		desiredSubscription, _ := defaultLuckyPools()
		desiredJSON, err := common.Marshal(desiredSubscription)
		if err != nil {
			return err
		}
		if active.SubscriptionPool != string(desiredJSON) {
			var maxVersion int
			if err := tx.Model(&LuckyRuleSet{}).Where("campaign_id = ?", campaign.Id).
				Select("COALESCE(MAX(version), 0)").Scan(&maxVersion).Error; err != nil {
				return err
			}
			now := GetDBTimestamp()
			next := *active
			next.Id = 0
			next.Version = maxVersion + 1
			next.Status = "active"
			next.SubscriptionPool = string(desiredJSON)
			next.PublishedAt = now
			next.EffectiveAt = now
			next.CreatedBy = 0
			next.CreatedAt = now
			RefreshLuckyRuleChecksum(&next)
			if err := ValidateLuckyRuleSet(&next); err != nil {
				return err
			}
			if err := tx.Model(&LuckyRuleSet{}).
				Where("campaign_id = ? AND status = ?", campaign.Id, "active").
				Update("status", "retired").Error; err != nil {
				return err
			}
			if err := tx.Create(&next).Error; err != nil {
				return err
			}
			campaign.ActiveRuleSetId = next.Id
			campaign.SettingsVersion++
			if err := tx.Save(campaign).Error; err != nil {
				return err
			}
		}

		return tx.Create(&Option{Key: luckyPrizeProbability20260811MigrationKey, Value: "true"}).Error
	})
}

const luckyWalletGiftBonus20260817MigrationKey = "LuckyWalletGiftBonus20260817MigratedV1"

func addLuckyWalletGiftBonus(raw string) (string, error) {
	var pool []LuckyPrizeConfig
	if err := common.UnmarshalJsonStr(raw, &pool); err != nil {
		return "", err
	}
	for i := range pool {
		switch pool[i].Code {
		case LuckyPrizeGift5, LuckyPrizeGift10, LuckyPrizeGift20:
			pool[i].DisplayUsdMicros += 10_000_000
		}
	}
	if err := validateLuckyPool(pool, true); err != nil {
		return "", err
	}
	encoded, err := common.Marshal(pool)
	return string(encoded), err
}

// EnsureLuckyWalletGiftBonus20260817 applies the owner-selected +$10 wallet
// gift policy to every rule version, including cards already issued against a
// historical rule. The marker makes the additive migration strictly one-shot.
func EnsureLuckyWalletGiftBonus20260817() error {
	return DB.Transaction(func(tx *gorm.DB) error {
		var marker Option
		if err := tx.Where(commonKeyCol+" = ?", luckyWalletGiftBonus20260817MigrationKey).First(&marker).Error; err == nil {
			return nil
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		var rules []LuckyRuleSet
		if err := tx.Find(&rules).Error; err != nil {
			return err
		}
		for i := range rules {
			subscriptionPool, err := addLuckyWalletGiftBonus(rules[i].SubscriptionPool)
			if err != nil {
				return fmt.Errorf("update lucky subscription gift amounts: %w", err)
			}
			rechargePool, err := addLuckyWalletGiftBonus(rules[i].RechargePool)
			if err != nil {
				return fmt.Errorf("update lucky recharge gift amounts: %w", err)
			}
			rules[i].SubscriptionPool = subscriptionPool
			rules[i].RechargePool = rechargePool
			RefreshLuckyRuleChecksum(&rules[i])
			if err := tx.Model(&LuckyRuleSet{}).Where("id = ?", rules[i].Id).Updates(map[string]interface{}{
				"subscription_pool": rules[i].SubscriptionPool,
				"recharge_pool":     rules[i].RechargePool,
				"checksum":          rules[i].Checksum,
			}).Error; err != nil {
				return err
			}
		}
		return tx.Create(&Option{Key: luckyWalletGiftBonus20260817MigrationKey, Value: "true"}).Error
	})
}

func GetLuckyCampaignTx(tx *gorm.DB, lock bool) (*LuckyCampaign, *LuckyRuleSet, error) {
	if tx == nil {
		tx = DB
	}
	query := tx.Where("code = ?", LuckyCampaignCode)
	if lock {
		query = query.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	var campaign LuckyCampaign
	if err := query.First(&campaign).Error; err != nil {
		return nil, nil, err
	}
	var rule LuckyRuleSet
	if err := tx.First(&rule, campaign.ActiveRuleSetId).Error; err != nil {
		return nil, nil, err
	}
	return &campaign, &rule, nil
}

type LuckyCardGrant struct {
	UserId                   int
	PoolType                 string
	SourceType               string
	SourceRef                string
	SourceOrderId            int
	SourceSubscriptionId     int
	SourceCycleKey           string
	SourceSnapshot           string
	SourceEffectiveEndTime   int64
	ExpiresAt                int64
	GrantKeyPrefix           string
	Count                    int
	HonorEligibilitySnapshot bool
}

func GrantLuckyCardsTx(tx *gorm.DB, campaign *LuckyCampaign, rule *LuckyRuleSet, grant LuckyCardGrant) ([]LuckyCard, error) {
	if tx == nil || campaign == nil || rule == nil || grant.UserId <= 0 || grant.Count <= 0 {
		return nil, errors.New("invalid lucky card grant")
	}
	if campaign.IssuancePaused && !grant.HonorEligibilitySnapshot {
		return nil, nil
	}
	if grant.PoolType != LuckyPoolSubscription && grant.PoolType != LuckyPoolRecharge {
		return nil, errors.New("invalid lucky pool")
	}
	if grant.ExpiresAt <= GetDBTimestamp() {
		return nil, errors.New("lucky card expiry must be in the future")
	}
	cards := make([]LuckyCard, 0, grant.Count)
	for i := 1; i <= grant.Count; i++ {
		card := LuckyCard{
			UserId: grant.UserId, CampaignId: campaign.Id, RuleSetId: rule.Id,
			PoolType: grant.PoolType, SourceType: grant.SourceType, SourceRef: grant.SourceRef,
			SourceOrderId: grant.SourceOrderId, SourceSubscriptionId: grant.SourceSubscriptionId,
			SourceCycleKey: grant.SourceCycleKey, SourceSnapshot: grant.SourceSnapshot,
			SourceEffectiveEndTime: grant.SourceEffectiveEndTime,
			GrantKey:               fmt.Sprintf("%s:slot:%d", grant.GrantKeyPrefix, i),
			Status:                 LuckyCardAvailable, IssuedAt: GetDBTimestamp(), ExpiresAt: grant.ExpiresAt,
		}
		result := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&card)
		if result.Error != nil {
			return nil, result.Error
		}
		if result.RowsAffected > 0 {
			cards = append(cards, card)
		}
	}
	return cards, nil
}

func GrantSubscriptionPurchaseLuckyCardsTx(tx *gorm.DB, order *SubscriptionOrder, sub *UserSubscription) ([]LuckyCard, error) {
	if tx == nil || order == nil || sub == nil {
		return nil, errors.New("invalid subscription lucky grant")
	}
	if !order.LuckyGrantEligible || order.LuckyGrantCount <= 0 || order.LuckyRuleSetId <= 0 ||
		sub.LuckyCardDisabled || order.Money <= 0 {
		return nil, nil
	}
	campaign, _, err := GetLuckyCampaignTx(tx, true)
	if err != nil {
		return nil, err
	}
	var rule LuckyRuleSet
	if err := tx.First(&rule, order.LuckyRuleSetId).Error; err != nil {
		return nil, err
	}
	return GrantLuckyCardsTx(tx, campaign, &rule, LuckyCardGrant{
		UserId: order.UserId, PoolType: LuckyPoolSubscription,
		SourceType: "subscription_purchase", SourceRef: order.TradeNo,
		SourceOrderId: order.Id, SourceSubscriptionId: sub.Id,
		SourceCycleKey: "initial", SourceSnapshot: sub.PlanSnapshot,
		SourceEffectiveEndTime: sub.EndTime, ExpiresAt: sub.EndTime,
		GrantKeyPrefix: fmt.Sprintf("purchase:%d", order.Id), Count: order.LuckyGrantCount,
		HonorEligibilitySnapshot: true,
	})
}

func RecordLuckyRechargeTx(tx *gorm.DB, topUp *TopUp) ([]LuckyCard, error) {
	if tx == nil || topUp == nil || topUp.UserId <= 0 || topUp.TradeNo == "" {
		return nil, errors.New("invalid lucky recharge")
	}
	if !topUp.LuckyRechargeEligible || topUp.LuckyRuleSetId <= 0 {
		return nil, nil
	}
	campaign, _, err := GetLuckyCampaignTx(tx, true)
	if err != nil {
		return nil, err
	}
	var rule LuckyRuleSet
	if err := tx.First(&rule, topUp.LuckyRuleSetId).Error; err != nil {
		return nil, err
	}
	// New payment flows persist the authenticated provider amount/currency.
	// Historical rows fall back to Money for reversal compatibility only.
	amountCents, snapshotErr := RechargeCentsForPayment(PaymentSnapshot{AmountMinor: topUp.ActualPaymentAmountMinor, Currency: topUp.ActualPaymentCurrency})
	if snapshotErr != nil {
		amountCents = MoneyToRechargeCents(topUp.Money)
	}
	if amountCents <= 0 {
		return nil, nil
	}
	event := LuckyRechargeEvent{
		UserId: topUp.UserId, SourceType: "wallet_topup", SourceRef: topUp.TradeNo,
		Direction: 1, AmountCents: amountCents, RuleSetId: rule.Id,
		OccurredAt: topUp.CompleteTime, CreatedAt: common.GetTimestamp(),
	}
	result := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&event)
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		return nil, nil
	}
	var progress LuckyRechargeProgress
	err = tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&progress, "user_id = ?", topUp.UserId).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		progress = LuckyRechargeProgress{UserId: topUp.UserId, NextThresholdCents: LuckyThresholdCents(1)}
		if err = tx.Create(&progress).Error; err != nil {
			return nil, err
		}
	} else if err != nil {
		return nil, err
	}
	progress.EligibleCents += amountCents
	cards := make([]LuckyCard, 0)
	for stage := progress.HighestAwardedStage + 1; LuckyThresholdCents(stage) <= progress.EligibleCents; stage++ {
		issued, grantErr := GrantLuckyCardsTx(tx, campaign, &rule, LuckyCardGrant{
			UserId: topUp.UserId, PoolType: LuckyPoolRecharge, SourceType: "recharge_threshold",
			SourceRef: topUp.TradeNo, SourceCycleKey: fmt.Sprintf("%d", stage),
			ExpiresAt:      event.OccurredAt + rule.RechargeCardValidSeconds,
			GrantKeyPrefix: fmt.Sprintf("recharge:%d:stage:%d", event.Id, stage), Count: 1,
			HonorEligibilitySnapshot: true,
		})
		if grantErr != nil {
			return nil, grantErr
		}
		cards = append(cards, issued...)
		progress.HighestAwardedStage = stage
	}
	progress.NextThresholdCents = LuckyThresholdCents(progress.HighestAwardedStage + 1)
	progress.UpdatedAt = common.GetTimestamp()
	if err := tx.Save(&progress).Error; err != nil {
		return nil, err
	}
	return cards, nil
}

// RecordLuckyPaidOrderRechargeTx adds a non-wallet, externally paid order to
// the same cumulative recharge progress. It deliberately does not create a
// TopUp row or wallet quota: the payment bought a product, not wallet balance.
func RecordLuckyPaidOrderRechargeTx(tx *gorm.DB, userId int, amountCents int64, sourceType, sourceRef string, occurredAt int64) ([]LuckyCard, error) {
	if tx == nil || userId <= 0 || amountCents <= 0 || strings.TrimSpace(sourceType) == "" || strings.TrimSpace(sourceRef) == "" {
		return nil, errors.New("invalid lucky paid-order recharge")
	}
	campaign, rule, err := GetLuckyCampaignTx(tx, true)
	if err != nil {
		return nil, err
	}
	if campaign.IssuancePaused {
		return nil, nil
	}
	return RecordLuckyPaidOrderRechargeSnapshotTx(tx, userId, amountCents, sourceType, sourceRef, occurredAt, rule.Id, true)
}

func RecordLuckyPaidOrderRechargeSnapshotTx(tx *gorm.DB, userId int, amountCents int64, sourceType, sourceRef string, occurredAt, ruleSetId int64, eligible bool) ([]LuckyCard, error) {
	if !eligible || ruleSetId <= 0 {
		return nil, nil
	}
	campaign, _, err := GetLuckyCampaignTx(tx, true)
	if err != nil {
		return nil, err
	}
	var rule LuckyRuleSet
	if err := tx.First(&rule, ruleSetId).Error; err != nil {
		return nil, err
	}
	if occurredAt <= 0 {
		occurredAt = GetDBTimestamp()
	}
	event := LuckyRechargeEvent{
		UserId: userId, SourceType: sourceType, SourceRef: sourceRef,
		Direction: 1, AmountCents: amountCents, RuleSetId: ruleSetId,
		OccurredAt: occurredAt, CreatedAt: common.GetTimestamp(),
	}
	result := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&event)
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		return nil, nil
	}
	var progress LuckyRechargeProgress
	err = tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&progress, "user_id = ?", userId).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		progress = LuckyRechargeProgress{UserId: userId, NextThresholdCents: LuckyThresholdCents(1)}
		if err = tx.Create(&progress).Error; err != nil {
			return nil, err
		}
	} else if err != nil {
		return nil, err
	}
	progress.EligibleCents += amountCents
	cards := make([]LuckyCard, 0)
	for stage := progress.HighestAwardedStage + 1; LuckyThresholdCents(stage) <= progress.EligibleCents; stage++ {
		issued, grantErr := GrantLuckyCardsTx(tx, campaign, &rule, LuckyCardGrant{
			UserId: userId, PoolType: LuckyPoolRecharge, SourceType: "recharge_threshold",
			SourceRef: sourceRef, SourceCycleKey: fmt.Sprintf("%d", stage),
			ExpiresAt:      event.OccurredAt + rule.RechargeCardValidSeconds,
			GrantKeyPrefix: fmt.Sprintf("paid-recharge:%d:stage:%d", event.Id, stage), Count: 1,
			HonorEligibilitySnapshot: true,
		})
		if grantErr != nil {
			return nil, grantErr
		}
		cards = append(cards, issued...)
		progress.HighestAwardedStage = stage
	}
	progress.NextThresholdCents = LuckyThresholdCents(progress.HighestAwardedStage + 1)
	progress.UpdatedAt = common.GetTimestamp()
	if err := tx.Save(&progress).Error; err != nil {
		return nil, err
	}
	return cards, nil
}

func markLuckyCardsForSourceReversalTx(tx *gorm.DB, query *gorm.DB, reason string) (LuckySourceReversalResult, error) {
	var result LuckySourceReversalResult
	now := GetDBTimestamp()
	reason = strings.TrimSpace(reason)
	if reason == "" {
		reason = "source payment reversed"
	}
	available := query.Session(&gorm.Session{}).Where("status = ?", LuckyCardAvailable).
		Updates(map[string]interface{}{
			"status":        LuckyCardRevoked,
			"revoked_at":    now,
			"revoke_reason": reason,
			"updated_at":    now,
		})
	if available.Error != nil {
		return result, available.Error
	}
	result.RevokedCards = available.RowsAffected

	var consumedCardIds []int64
	if err := query.Session(&gorm.Session{}).Where("status = ?", LuckyCardConsumed).
		Pluck("id", &consumedCardIds).Error; err != nil {
		return result, err
	}
	if len(consumedCardIds) == 0 {
		return result, nil
	}
	cardUpdate := tx.Model(&LuckyCard{}).Where("id IN ?", consumedCardIds).
		Updates(map[string]interface{}{
			"status":        LuckyCardReviewRequired,
			"revoke_reason": reason,
			"updated_at":    now,
		})
	if cardUpdate.Error != nil {
		return result, cardUpdate.Error
	}
	result.ReviewCards = cardUpdate.RowsAffected
	drawUpdate := tx.Model(&LuckyDraw{}).
		Where("card_id IN ? AND status = ?", consumedCardIds, "awarded").
		Updates(map[string]interface{}{"status": "review_required", "updated_at": now})
	if drawUpdate.Error != nil {
		return result, drawUpdate.Error
	}
	result.ReviewDraws = drawUpdate.RowsAffected
	return result, nil
}

// ReverseLuckyRechargeTx records a refund/chargeback against the activity
// ledger. Historical awarded stages never decrease, so refund-and-recharge
// cannot earn the same threshold twice.
func ReverseLuckyRechargeTx(tx *gorm.DB, topUp *TopUp, reason string) (LuckySourceReversalResult, error) {
	var result LuckySourceReversalResult
	if tx == nil || topUp == nil || topUp.UserId <= 0 || strings.TrimSpace(topUp.TradeNo) == "" {
		return result, errors.New("invalid lucky recharge reversal")
	}
	if !topUp.LuckyRechargeEligible || topUp.LuckyRuleSetId <= 0 {
		return result, nil
	}
	amountCents, snapshotErr := RechargeCentsForPayment(PaymentSnapshot{AmountMinor: topUp.ActualPaymentAmountMinor, Currency: topUp.ActualPaymentCurrency})
	if snapshotErr != nil {
		amountCents = MoneyToRechargeCents(topUp.Money)
	}
	if amountCents <= 0 {
		return result, nil
	}
	now := GetDBTimestamp()
	event := LuckyRechargeEvent{
		UserId: topUp.UserId, SourceType: "wallet_topup", SourceRef: topUp.TradeNo,
		Direction: -1, AmountCents: amountCents, RuleSetId: topUp.LuckyRuleSetId,
		OccurredAt: now, CreatedAt: now,
	}
	insert := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&event)
	if insert.Error != nil {
		return result, insert.Error
	}
	if insert.RowsAffected == 0 {
		return result, nil
	}
	result.EventCreated = true

	var progress LuckyRechargeProgress
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		First(&progress, "user_id = ?", topUp.UserId).Error
	if err == nil {
		progress.EligibleCents -= amountCents
		if progress.EligibleCents < 0 {
			progress.EligibleCents = 0
		}
		progress.NextThresholdCents = LuckyThresholdCents(progress.HighestAwardedStage + 1)
		progress.UpdatedAt = now
		if err := tx.Save(&progress).Error; err != nil {
			return result, err
		}
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return result, err
	}

	cardResult, err := markLuckyCardsForSourceReversalTx(
		tx,
		tx.Model(&LuckyCard{}).
			Where("user_id = ? AND source_type = ? AND source_ref = ?",
				topUp.UserId, "recharge_threshold", topUp.TradeNo),
		reason,
	)
	if err != nil {
		return result, err
	}
	result.RevokedCards = cardResult.RevokedCards
	result.ReviewCards = cardResult.ReviewCards
	result.ReviewDraws = cardResult.ReviewDraws
	return result, nil
}

// ReverseSubscriptionLuckySourceTx reconciles cards after a paid subscription
// source is refunded or charged back. It does not mutate the payment or
// subscription itself; callers must invoke it from the authoritative refund
// workflow.
func ReverseSubscriptionLuckySourceTx(tx *gorm.DB, order *SubscriptionOrder, reason string) (LuckySourceReversalResult, error) {
	var result LuckySourceReversalResult
	if tx == nil || order == nil || order.Id <= 0 || order.UserId <= 0 {
		return result, errors.New("invalid subscription lucky reversal")
	}
	var sourceSubscriptionIds []int
	if err := tx.Model(&LuckyCard{}).
		Where("user_id = ? AND source_order_id = ?", order.UserId, order.Id).
		Distinct("source_subscription_id").
		Pluck("source_subscription_id", &sourceSubscriptionIds).Error; err != nil {
		return result, err
	}
	query := tx.Model(&LuckyCard{}).Where("user_id = ? AND source_order_id = ?", order.UserId, order.Id)
	validIds := make([]int, 0, len(sourceSubscriptionIds))
	for _, id := range sourceSubscriptionIds {
		if id > 0 {
			validIds = append(validIds, id)
		}
	}
	if len(validIds) > 0 {
		query = tx.Model(&LuckyCard{}).
			Where("user_id = ? AND (source_order_id = ? OR source_subscription_id IN ?)",
				order.UserId, order.Id, validIds)
	}
	return markLuckyCardsForSourceReversalTx(tx, query, reason)
}

func ListLuckyCards(userId int, page, pageSize int) ([]LuckyCard, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	now := GetDBTimestamp()
	_ = DB.Model(&LuckyCard{}).Where("user_id = ? AND status = ? AND expires_at <= ?", userId, LuckyCardAvailable, now).
		Updates(map[string]interface{}{"status": LuckyCardExpired, "updated_at": now}).Error
	var total int64
	if err := DB.Model(&LuckyCard{}).Where("user_id = ?", userId).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var cards []LuckyCard
	err := DB.Where("user_id = ?", userId).Order("id desc").Limit(pageSize).Offset((page - 1) * pageSize).Find(&cards).Error
	return cards, total, err
}

// RevokeUserAvailableLuckyCardsTx removes a user's remaining draw entitlement
// without deleting card or draw history. Consumed cards must stay immutable so
// awarded rewards retain their source audit chain.
func RevokeUserAvailableLuckyCardsTx(tx *gorm.DB, userId int, reason string) (int64, error) {
	if tx == nil || userId <= 0 || strings.TrimSpace(reason) == "" {
		return 0, errors.New("invalid lucky card revocation")
	}
	now := GetDBTimestamp()
	result := tx.Model(&LuckyCard{}).
		Where("user_id = ? AND status = ?", userId, LuckyCardAvailable).
		Updates(map[string]interface{}{
			"status":        LuckyCardRevoked,
			"revoked_at":    now,
			"revoke_reason": strings.TrimSpace(reason),
			"updated_at":    now,
		})
	return result.RowsAffected, result.Error
}

func ListLuckyDraws(userId int, page, pageSize int) ([]LuckyDraw, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	var total int64
	if err := DB.Model(&LuckyDraw{}).Where("user_id = ?", userId).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var draws []LuckyDraw
	err := DB.Where("user_id = ?", userId).Order("id desc").Limit(pageSize).Offset((page - 1) * pageSize).Find(&draws).Error
	return draws, total, err
}

func SortSubscriptionsByPreferenceTx(tx *gorm.DB, userId int, group string, subs []UserSubscription) error {
	if len(subs) < 2 {
		return nil
	}
	// Focused unit tests and rolling upgrades may temporarily run against the
	// pre-feature schema. Preserve the historical end_time/id ordering until
	// the additive table is present.
	if !tx.Migrator().HasTable(&SubscriptionConsumptionPriority{}) {
		return nil
	}
	var priorities []SubscriptionConsumptionPriority
	if err := tx.Where("user_id = ? AND group_name = ?", userId, group).Find(&priorities).Error; err != nil {
		return err
	}
	ranks := make(map[int]int, len(priorities))
	for _, item := range priorities {
		ranks[item.SubscriptionId] = item.Priority
	}
	sort.SliceStable(subs, func(i, j int) bool {
		ri, iok := ranks[subs[i].Id]
		rj, jok := ranks[subs[j].Id]
		if iok != jok {
			return iok
		}
		if iok && ri != rj {
			return ri < rj
		}
		if subs[i].EndTime != subs[j].EndTime {
			return subs[i].EndTime < subs[j].EndTime
		}
		return subs[i].Id < subs[j].Id
	})
	return nil
}

func InvalidateUserTokenCaches(userId int) {
	if userId <= 0 || !common.RedisEnabled {
		return
	}
	var keys []string
	if err := DB.Model(&Token{}).Where("user_id = ?", userId).Pluck("key", &keys).Error; err != nil {
		return
	}
	for _, key := range keys {
		_ = cacheDeleteToken(key)
	}
}
