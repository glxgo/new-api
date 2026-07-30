package model

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/pkg/cachex"
	"github.com/samber/hot"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// Subscription duration units
const (
	SubscriptionDurationYear   = "year"
	SubscriptionDurationMonth  = "month"
	SubscriptionDurationDay    = "day"
	SubscriptionDurationWeek   = "week"
	SubscriptionDurationHour   = "hour"
	SubscriptionDurationCustom = "custom"
)

// Subscription quota reset period
const (
	SubscriptionResetNever   = "never"
	SubscriptionResetDaily   = "daily"
	SubscriptionResetWeekly  = "weekly"
	SubscriptionResetMonthly = "monthly"
	SubscriptionResetCustom  = "custom"
)

// Subscription plan version (drives card border color & badge: starter=铜/advanced=银/pro=金/enterprise=黑金)
const (
	PlanVersionStarter    = "starter"
	PlanVersionAdvanced   = "advanced"
	PlanVersionPro        = "pro"
	PlanVersionEnterprise = "enterprise"
)

var (
	ErrSubscriptionOrderNotFound      = errors.New("subscription order not found")
	ErrSubscriptionOrderStatusInvalid = errors.New("subscription order status invalid")
	ErrChannelCostRatioMissing        = errors.New("channel cost ratio is not configured")
)

const (
	subscriptionPlanCacheNamespace           = "new-api:subscription_plan:v1"
	subscriptionPlanInfoCacheNamespace       = "new-api:subscription_plan_info:v1"
	ChannelCostRatioScale              int64 = 1_000_000
	MaxChannelCostRatioPPM             int64 = 1_000_000_000

	SubscriptionCostStatusReserved    = "reserved"
	SubscriptionCostStatusProvisional = "provisional"
	SubscriptionCostStatusFinal       = "final"
	SubscriptionCostStatusRefunded    = "refunded"

	SubscriptionDividendPending         = "pending"
	SubscriptionDividendProcessing      = "processing"
	SubscriptionDividendDone            = "done"
	SubscriptionDividendSkippedNoProfit = "skipped_no_profit"
	SubscriptionDividendSkippedSource   = "skipped_source"
)

var (
	subscriptionPlanCacheOnce     sync.Once
	subscriptionPlanInfoCacheOnce sync.Once

	subscriptionPlanCache     *cachex.HybridCache[SubscriptionPlan]
	subscriptionPlanInfoCache *cachex.HybridCache[SubscriptionPlanInfo]
)

func subscriptionPlanCacheTTL() time.Duration {
	ttlSeconds := common.GetEnvOrDefault("SUBSCRIPTION_PLAN_CACHE_TTL", 300)
	if ttlSeconds <= 0 {
		ttlSeconds = 300
	}
	return time.Duration(ttlSeconds) * time.Second
}

func subscriptionPlanInfoCacheTTL() time.Duration {
	ttlSeconds := common.GetEnvOrDefault("SUBSCRIPTION_PLAN_INFO_CACHE_TTL", 120)
	if ttlSeconds <= 0 {
		ttlSeconds = 120
	}
	return time.Duration(ttlSeconds) * time.Second
}

func subscriptionPlanCacheCapacity() int {
	capacity := common.GetEnvOrDefault("SUBSCRIPTION_PLAN_CACHE_CAP", 5000)
	if capacity <= 0 {
		capacity = 5000
	}
	return capacity
}

func subscriptionPlanInfoCacheCapacity() int {
	capacity := common.GetEnvOrDefault("SUBSCRIPTION_PLAN_INFO_CACHE_CAP", 10000)
	if capacity <= 0 {
		capacity = 10000
	}
	return capacity
}

func getSubscriptionPlanCache() *cachex.HybridCache[SubscriptionPlan] {
	subscriptionPlanCacheOnce.Do(func() {
		ttl := subscriptionPlanCacheTTL()
		subscriptionPlanCache = cachex.NewHybridCache[SubscriptionPlan](cachex.HybridCacheConfig[SubscriptionPlan]{
			Namespace: cachex.Namespace(subscriptionPlanCacheNamespace),
			Redis:     common.RDB,
			RedisEnabled: func() bool {
				return common.RedisEnabled && common.RDB != nil
			},
			RedisCodec: cachex.JSONCodec[SubscriptionPlan]{},
			Memory: func() *hot.HotCache[string, SubscriptionPlan] {
				return hot.NewHotCache[string, SubscriptionPlan](hot.LRU, subscriptionPlanCacheCapacity()).
					WithTTL(ttl).
					WithJanitor().
					Build()
			},
		})
	})
	return subscriptionPlanCache
}

func getSubscriptionPlanInfoCache() *cachex.HybridCache[SubscriptionPlanInfo] {
	subscriptionPlanInfoCacheOnce.Do(func() {
		ttl := subscriptionPlanInfoCacheTTL()
		subscriptionPlanInfoCache = cachex.NewHybridCache[SubscriptionPlanInfo](cachex.HybridCacheConfig[SubscriptionPlanInfo]{
			Namespace: cachex.Namespace(subscriptionPlanInfoCacheNamespace),
			Redis:     common.RDB,
			RedisEnabled: func() bool {
				return common.RedisEnabled && common.RDB != nil
			},
			RedisCodec: cachex.JSONCodec[SubscriptionPlanInfo]{},
			Memory: func() *hot.HotCache[string, SubscriptionPlanInfo] {
				return hot.NewHotCache[string, SubscriptionPlanInfo](hot.LRU, subscriptionPlanInfoCacheCapacity()).
					WithTTL(ttl).
					WithJanitor().
					Build()
			},
		})
	})
	return subscriptionPlanInfoCache
}

func subscriptionPlanCacheKey(id int) string {
	if id <= 0 {
		return ""
	}
	return strconv.Itoa(id)
}

func InvalidateSubscriptionPlanCache(planId int) {
	if planId <= 0 {
		return
	}
	cache := getSubscriptionPlanCache()
	_, _ = cache.DeleteMany([]string{subscriptionPlanCacheKey(planId)})
	infoCache := getSubscriptionPlanInfoCache()
	_ = infoCache.Purge()
}

// Subscription plan
type SubscriptionPlan struct {
	Id int `json:"id"`

	Title    string `json:"title" gorm:"type:varchar(128);not null"`
	Subtitle string `json:"subtitle" gorm:"type:varchar(255);default:''"`
	// SuitableFor: 适合人群(套餐名下方展示"适合 XXX")
	SuitableFor string `json:"suitable_for" gorm:"type:varchar(255);default:''"`
	// Description: 套餐详细介绍(用户订阅时弹出展示, "已阅读"关闭/"永不展示"localStorage 记住)
	Description string `json:"description" gorm:"type:text"`

	// Display money amount (follow existing code style: float64 for money)
	PriceAmount float64 `json:"price_amount" gorm:"type:decimal(10,6);not null;default:0"`
	Currency    string  `json:"currency" gorm:"type:varchar(8);not null;default:'USD'"`

	DurationUnit  string `json:"duration_unit" gorm:"type:varchar(16);not null;default:'month'"`
	DurationValue int    `json:"duration_value" gorm:"type:int;not null;default:1"`
	CustomSeconds int64  `json:"custom_seconds" gorm:"type:bigint;not null;default:0"`

	Enabled   bool `json:"enabled" gorm:"default:true"`
	SortOrder int  `json:"sort_order" gorm:"type:int;default:0"`

	AllowBalancePay *bool `json:"allow_balance_pay" gorm:"default:true"`
	// LuckyCardGrantCount is snapshotted into a paid order. Reward-created
	// subscriptions always disable card generation at the instance level.
	LuckyCardGrantCount int  `json:"lucky_card_grant_count" gorm:"type:int;not null;default:0"`
	LuckyCardOnReset    bool `json:"lucky_card_on_reset" gorm:"not null;default:false"`

	StripePriceId         string `json:"stripe_price_id" gorm:"type:varchar(128);default:''"`
	CreemProductId        string `json:"creem_product_id" gorm:"type:varchar(128);default:''"`
	WaffoPancakeProductId string `json:"waffo_pancake_product_id" gorm:"type:varchar(128);default:''"`

	// Max purchases per user (0 = unlimited)
	MaxPurchasePerUser int `json:"max_purchase_per_user" gorm:"type:int;default:0"`

	// Upgrade user group after purchase (empty = no change)
	UpgradeGroup string `json:"upgrade_group" gorm:"type:varchar(64);default:''"`

	// AllowedGroup: 订阅级分组限制。非空时订阅额度只能用于该分组(不改 user.group, 与 UpgradeGroup 互斥)
	AllowedGroup string `json:"allowed_group" gorm:"type:varchar(64);default:''"`
	// RenewalPlanId explicitly points to the currently sellable replacement
	// used when this plan itself can no longer be renewed. Nil means no
	// replacement; never infer one from title/group similarity.
	RenewalPlanId *int `json:"renewal_plan_id" gorm:"default:null;index"`

	// NumberPool: 号池(权益列表展示, 如"旗舰池/高质量池")
	NumberPool string `json:"number_pool" gorm:"type:varchar(255);default:''"`

	// ModelLimit: 模型限制(权益列表展示"模型限制: XXX", 管理员自由填写)
	ModelLimit string `json:"model_limit" gorm:"type:varchar(255);default:''"`
	// PlanVersion: 套餐版本(入门版/进阶版/专业版/企业版), 驱动用户卡片边框配色与右上徽章
	PlanVersion string `json:"plan_version" gorm:"type:varchar(32);default:''"`

	// Recommended: 前台展示「推荐」标记
	Recommended bool `json:"recommended" gorm:"default:false"`
	// MinRatio: 最低倍率(展示用, 如 0.35 表示该套餐分组模型最低 0.35 折)
	MinRatio float64 `json:"min_ratio" gorm:"default:0"`

	// Total quota (amount in quota units, 0 = unlimited)
	TotalAmount int64 `json:"total_amount" gorm:"type:bigint;not null;default:0"`

	// AmountCap: 套餐总额度上限(整个套餐周期, 不随 reset 重置, 0=不限)。达到则套餐到期。
	AmountCap int64 `json:"amount_cap" gorm:"type:bigint;not null;default:0"`

	// Quota reset period for plan
	QuotaResetPeriod        string `json:"quota_reset_period" gorm:"type:varchar(16);default:'never'"`
	QuotaResetCustomSeconds int64  `json:"quota_reset_custom_seconds" gorm:"type:bigint;default:0"`

	CreatedAt int64 `json:"created_at" gorm:"bigint"`
	UpdatedAt int64 `json:"updated_at" gorm:"bigint"`
}

func (p *SubscriptionPlan) BeforeCreate(tx *gorm.DB) error {
	now := common.GetTimestamp()
	p.CreatedAt = now
	p.UpdatedAt = now
	return nil
}

func (p *SubscriptionPlan) BeforeUpdate(tx *gorm.DB) error {
	p.UpdatedAt = common.GetTimestamp()
	return nil
}

func (p *SubscriptionPlan) NormalizeDefaults() {
	if p.AllowBalancePay == nil {
		p.AllowBalancePay = common.GetPointer(true)
	}
}

func BuildSubscriptionPlanSnapshot(plan *SubscriptionPlan) (string, error) {
	if plan == nil || plan.Id <= 0 {
		return "", errors.New("invalid plan")
	}
	snapshot := *plan
	snapshot.NormalizeDefaults()
	data, err := common.Marshal(snapshot)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func ParseSubscriptionPlanSnapshot(snapshot string) (*SubscriptionPlan, error) {
	if strings.TrimSpace(snapshot) == "" {
		return nil, errors.New("subscription plan snapshot is empty")
	}
	var plan SubscriptionPlan
	if err := common.UnmarshalJsonStr(snapshot, &plan); err != nil {
		return nil, err
	}
	if plan.Id <= 0 {
		return nil, errors.New("subscription plan snapshot has invalid plan id")
	}
	plan.NormalizeDefaults()
	return &plan, nil
}

func planForUserSubscriptionTx(tx *gorm.DB, sub *UserSubscription) (*SubscriptionPlan, error) {
	if sub == nil {
		return nil, errors.New("subscription is nil")
	}
	if strings.TrimSpace(sub.PlanSnapshot) != "" {
		if plan, err := ParseSubscriptionPlanSnapshot(sub.PlanSnapshot); err == nil {
			return plan, nil
		}
	}
	return getSubscriptionPlanByIdTx(tx, sub.PlanId)
}

// Subscription order (payment -> webhook -> create UserSubscription)
type SubscriptionOrder struct {
	Id     int     `json:"id"`
	UserId int     `json:"user_id" gorm:"index"`
	PlanId int     `json:"plan_id" gorm:"index"`
	Money  float64 `json:"money"`

	TradeNo         string `json:"trade_no" gorm:"unique;type:varchar(255);index"`
	PaymentMethod   string `json:"payment_method" gorm:"type:varchar(50)"`
	PaymentProvider string `json:"payment_provider" gorm:"type:varchar(50);default:''"`
	Status          string `json:"status"`
	CreateTime      int64  `json:"create_time"`
	CompleteTime    int64  `json:"complete_time"`

	ProviderPayload         string `json:"provider_payload" gorm:"type:text"`
	PlanSnapshot            string `json:"plan_snapshot" gorm:"type:text"`
	RenewFromSubscriptionId *int   `json:"renew_from_subscription_id" gorm:"default:null;index"`
	RenewalBindingSnapshot  string `json:"renewal_binding_snapshot" gorm:"type:text"`
	LuckyRuleSetId          int64  `json:"lucky_rule_set_id" gorm:"index;not null;default:0"`
	LuckyGrantEligible      bool   `json:"lucky_grant_eligible" gorm:"not null;default:false"`
	LuckyGrantCount         int    `json:"lucky_grant_count" gorm:"not null;default:0"`
	LuckyGrantOnReset       bool   `json:"lucky_grant_on_reset" gorm:"not null;default:false"`
}

func (o *SubscriptionOrder) Insert() error {
	if o.CreateTime == 0 {
		o.CreateTime = common.GetTimestamp()
	}
	if o.RenewFromSubscriptionId == nil || *o.RenewFromSubscriptionId <= 0 {
		return DB.Create(o).Error
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		var source UserSubscription
		if err := tx.Set("gorm:query_option", "FOR UPDATE").
			Where("id = ? AND user_id = ?", *o.RenewFromSubscriptionId, o.UserId).
			First(&source).Error; err != nil {
			return err
		}
		var successorCount int64
		if err := tx.Model(&UserSubscription{}).
			Where("renewed_from_id = ?", source.Id).
			Count(&successorCount).Error; err != nil {
			return err
		}
		if successorCount > 0 {
			return ErrSubscriptionRenewalAlreadyExists
		}
		var pendingCount int64
		if err := tx.Model(&SubscriptionOrder{}).
			Where("renew_from_subscription_id = ? AND status = ?", source.Id, common.TopUpStatusPending).
			Count(&pendingCount).Error; err != nil {
			return err
		}
		if pendingCount > 0 {
			return ErrSubscriptionRenewalOrderPending
		}
		return tx.Create(o).Error
	})
}

func (o *SubscriptionOrder) Update() error {
	return DB.Save(o).Error
}

func GetSubscriptionOrderByTradeNo(tradeNo string) *SubscriptionOrder {
	if tradeNo == "" {
		return nil
	}
	var order SubscriptionOrder
	if err := DB.Where("trade_no = ?", tradeNo).First(&order).Error; err != nil {
		return nil
	}
	return &order
}

func NewSubscriptionOrderFromPlan(userId int, plan *SubscriptionPlan, tradeNo, paymentMethod, paymentProvider string) (*SubscriptionOrder, error) {
	if userId <= 0 || plan == nil || plan.Id <= 0 || strings.TrimSpace(tradeNo) == "" {
		return nil, errors.New("invalid subscription order args")
	}
	snapshot, err := BuildSubscriptionPlanSnapshot(plan)
	if err != nil {
		return nil, err
	}
	order := &SubscriptionOrder{
		UserId:          userId,
		PlanId:          plan.Id,
		Money:           plan.PriceAmount,
		TradeNo:         tradeNo,
		PaymentMethod:   paymentMethod,
		PaymentProvider: paymentProvider,
		Status:          common.TopUpStatusPending,
		CreateTime:      common.GetTimestamp(),
		PlanSnapshot:    snapshot,
	}
	if campaign, rule, lookupErr := GetLuckyCampaignTx(nil, false); lookupErr == nil && !campaign.IssuancePaused {
		order.LuckyRuleSetId = rule.Id
		order.LuckyGrantCount = plan.LuckyCardGrantCount
		order.LuckyGrantOnReset = plan.LuckyCardOnReset
		order.LuckyGrantEligible = plan.PriceAmount > 0 &&
			(order.LuckyGrantCount > 0 || order.LuckyGrantOnReset)
	}
	return order, nil
}

// User subscription instance
type UserSubscription struct {
	Id     int `json:"id"`
	UserId int `json:"user_id" gorm:"index;index:idx_user_sub_active,priority:1"`
	PlanId int `json:"plan_id" gorm:"index"`
	// PlanSnapshot freezes the purchased entitlements for reset/runtime logic.
	// Existing rows may be empty and continue to use the historical fallback.
	PlanSnapshot  string `json:"-" gorm:"type:text"`
	PlanTitle     string `json:"plan_title" gorm:"type:varchar(128);not null;default:''"`
	Remark        string `json:"remark" gorm:"type:varchar(128);not null;default:''"`
	RenewedFromId *int   `json:"renewed_from_id" gorm:"default:null;uniqueIndex"`

	AmountTotal int64 `json:"amount_total" gorm:"type:bigint;not null;default:0"`
	AmountUsed  int64 `json:"amount_used" gorm:"type:bigint;not null;default:0"`

	// AmountCap: 套餐总额度上限(冗余自 plan, 不随 reset 重置, 0=不限)
	AmountCap int64 `json:"amount_cap" gorm:"type:bigint;not null;default:0"`
	// AmountCapUsed: 已用总额度(不随 reset 重置, 达到 AmountCap 则套餐到期)
	AmountCapUsed int64 `json:"amount_cap_used" gorm:"type:bigint;not null;default:0"`

	StartTime int64  `json:"start_time" gorm:"bigint"`
	EndTime   int64  `json:"end_time" gorm:"bigint;index;index:idx_user_sub_active,priority:3"`
	Status    string `json:"status" gorm:"type:varchar(32);index;index:idx_user_sub_active,priority:2"` // active/expired/cancelled

	Source string `json:"source" gorm:"type:varchar(32);default:'order'"` // order/admin

	LastResetTime int64 `json:"last_reset_time" gorm:"type:bigint;default:0"`
	NextResetTime int64 `json:"next_reset_time" gorm:"type:bigint;default:0;index"`

	UpgradeGroup  string `json:"upgrade_group" gorm:"type:varchar(64);default:''"`
	PrevUserGroup string `json:"prev_user_group" gorm:"type:varchar(64);default:''"`

	// AllowedGroup: 冗余自 plan, 扣费时按 relayInfo.UsingGroup 过滤(订阅级分组限制)
	AllowedGroup string `json:"allowed_group" gorm:"type:varchar(64);default:''"`

	// PaidRevenueQuota is the immutable actual paid amount snapshot in quota units.
	// CostAccumulator stores the exact sum of sale_quota*channel_ratio_ppm and is
	// rounded only once when the subscription ends.
	PaidRevenueQuota              int64  `json:"paid_revenue_quota" gorm:"type:bigint;not null;default:0"`
	CostAccumulator               int64  `json:"cost_accumulator" gorm:"type:bigint;not null;default:0"`
	DividendState                 string `json:"dividend_state" gorm:"type:varchar(32);not null;default:'pending';index"`
	DividendReadyAt               int64  `json:"dividend_ready_at" gorm:"type:bigint;not null;default:0;index"`
	LuckyCardDisabled             bool   `json:"lucky_card_disabled" gorm:"not null;default:false"`
	PromotionOriginDrawId         int64  `json:"promotion_origin_draw_id" gorm:"index;not null;default:0"`
	PromotionSourceSubscriptionId int    `json:"promotion_source_subscription_id" gorm:"index;not null;default:0"`
	SupersededById                *int   `json:"superseded_by_id" gorm:"default:null;index"`

	CreatedAt int64 `json:"created_at" gorm:"bigint"`
	UpdatedAt int64 `json:"updated_at" gorm:"bigint"`
}

func (s *UserSubscription) BeforeCreate(tx *gorm.DB) error {
	now := common.GetTimestamp()
	s.CreatedAt = now
	s.UpdatedAt = now
	return nil
}

func (s *UserSubscription) BeforeUpdate(tx *gorm.DB) error {
	s.UpdatedAt = common.GetTimestamp()
	return nil
}

type SubscriptionSummary struct {
	Subscription *UserSubscription `json:"subscription"`
}

type LuckySubscriptionProgress struct {
	Subscribed bool  `json:"subscribed"`
	Eligible   bool  `json:"eligible"`
	NextCardAt int64 `json:"next_card_at"`
}

// GetLuckySubscriptionProgress returns the earliest real subscription reset
// that can issue a lucky card. Reward-created instances stay visible as active
// subscriptions, but LuckyCardDisabled prevents them from advertising another
// card here.
func GetLuckySubscriptionProgress(userId int, now int64) (LuckySubscriptionProgress, error) {
	progress := LuckySubscriptionProgress{}
	if userId <= 0 {
		return progress, errors.New("invalid userId")
	}
	if now <= 0 {
		now = GetDBTimestamp()
	}
	var subscriptions []UserSubscription
	if err := DB.Where(
		"user_id = ? AND status = ? AND start_time <= ? AND end_time > ?",
		userId, "active", now, now,
	).Order("next_reset_time asc, end_time asc, id asc").Find(&subscriptions).Error; err != nil {
		return progress, err
	}
	progress.Subscribed = len(subscriptions) > 0
	for i := range subscriptions {
		subscription := &subscriptions[i]
		if subscription.LuckyCardDisabled {
			continue
		}
		plan, err := planForUserSubscriptionTx(DB, subscription)
		if errors.Is(err, gorm.ErrRecordNotFound) {
			continue
		}
		if err != nil {
			return progress, err
		}
		if !plan.LuckyCardOnReset ||
			NormalizeResetPeriod(plan.QuotaResetPeriod) == SubscriptionResetNever {
			continue
		}
		nextCardAt := subscription.NextResetTime
		if nextCardAt <= 0 {
			baseUnix := subscription.LastResetTime
			if baseUnix <= 0 {
				baseUnix = subscription.StartTime
			}
			nextCardAt = calcNextResetTime(
				time.Unix(baseUnix, 0),
				plan,
				subscription.EndTime,
				subscription.StartTime,
			)
		}
		if nextCardAt <= 0 {
			continue
		}
		progress.Eligible = true
		if progress.NextCardAt == 0 || nextCardAt < progress.NextCardAt {
			progress.NextCardAt = nextCardAt
		}
	}
	return progress, nil
}

func calcPlanEndTime(start time.Time, plan *SubscriptionPlan) (int64, error) {
	if plan == nil {
		return 0, errors.New("plan is nil")
	}
	if plan.DurationValue <= 0 && plan.DurationUnit != SubscriptionDurationCustom {
		return 0, errors.New("duration_value must be > 0")
	}
	switch plan.DurationUnit {
	case SubscriptionDurationYear:
		return start.AddDate(plan.DurationValue, 0, 0).Unix(), nil
	case SubscriptionDurationMonth:
		return start.AddDate(0, plan.DurationValue, 0).Unix(), nil
	case SubscriptionDurationDay:
		return start.Add(time.Duration(plan.DurationValue) * 24 * time.Hour).Unix(), nil
	case SubscriptionDurationWeek:
		return start.Add(time.Duration(plan.DurationValue) * 7 * 24 * time.Hour).Unix(), nil
	case SubscriptionDurationHour:
		return start.Add(time.Duration(plan.DurationValue) * time.Hour).Unix(), nil
	case SubscriptionDurationCustom:
		if plan.CustomSeconds <= 0 {
			return 0, errors.New("custom_seconds must be > 0")
		}
		return start.Add(time.Duration(plan.CustomSeconds) * time.Second).Unix(), nil
	default:
		return 0, fmt.Errorf("invalid duration_unit: %s", plan.DurationUnit)
	}
}

func NormalizeResetPeriod(period string) string {
	switch strings.TrimSpace(period) {
	case SubscriptionResetDaily, SubscriptionResetWeekly, SubscriptionResetMonthly, SubscriptionResetCustom:
		return strings.TrimSpace(period)
	default:
		return SubscriptionResetNever
	}
}

// NormalizePlanVersion 非法/空值回落到 ""(不设版本, 卡片保持默认外观)
func NormalizePlanVersion(v string) string {
	switch strings.TrimSpace(v) {
	case PlanVersionStarter, PlanVersionAdvanced, PlanVersionPro, PlanVersionEnterprise:
		return strings.TrimSpace(v)
	default:
		return ""
	}
}

func calcNextResetTime(base time.Time, plan *SubscriptionPlan, endUnix int64, startUnix int64) int64 {
	if plan == nil {
		return 0
	}
	period := NormalizeResetPeriod(plan.QuotaResetPeriod)
	if period == SubscriptionResetNever {
		return 0
	}
	var next time.Time
	switch period {
	case SubscriptionResetDaily:
		next = time.Date(base.Year(), base.Month(), base.Day(), 0, 0, 0, 0, base.Location()).
			AddDate(0, 0, 1)
	case SubscriptionResetWeekly:
		// 所有卡(周卡/月卡/季卡/年卡)都从订阅(付款)日起每 7 天一周期, 不对齐周一。
		// 单月卡(duration_value=1)特殊: 前 3 周各重置一次(start+7/+14/+21),
		// 第 4 段(start+21 → 月底约 9-10 天)不再重置, 额度用到月底到期。
		// 解决月卡 30 天按 4×7=28 天算多出 2-3 天、最后一周额度浪费的问题。
		if plan.DurationUnit == SubscriptionDurationMonth && plan.DurationValue == 1 && startUnix > 0 {
			weeksFromStart := int(base.Sub(time.Unix(startUnix, 0)).Hours() / 24 / 7)
			if weeksFromStart >= 3 {
				return 0
			}
		}
		next = base.AddDate(0, 0, 7)
	case SubscriptionResetMonthly:
		// Align to first day of next month 00:00
		next = time.Date(base.Year(), base.Month(), 1, 0, 0, 0, 0, base.Location()).
			AddDate(0, 1, 0)
	case SubscriptionResetCustom:
		if plan.QuotaResetCustomSeconds <= 0 {
			return 0
		}
		next = base.Add(time.Duration(plan.QuotaResetCustomSeconds) * time.Second)
	default:
		return 0
	}
	if endUnix > 0 && next.Unix() > endUnix {
		return 0
	}
	return next.Unix()
}

func GetSubscriptionPlanById(id int) (*SubscriptionPlan, error) {
	return getSubscriptionPlanByIdTx(nil, id)
}

func getSubscriptionPlanByIdTx(tx *gorm.DB, id int) (*SubscriptionPlan, error) {
	if id <= 0 {
		return nil, errors.New("invalid plan id")
	}
	key := subscriptionPlanCacheKey(id)
	if key != "" {
		if cached, found, err := getSubscriptionPlanCache().Get(key); err == nil && found {
			cached.NormalizeDefaults()
			return &cached, nil
		}
	}
	var plan SubscriptionPlan
	query := DB
	if tx != nil {
		query = tx
	}
	if err := query.Where("id = ?", id).First(&plan).Error; err != nil {
		return nil, err
	}
	plan.NormalizeDefaults()
	_ = getSubscriptionPlanCache().SetWithTTL(key, plan, subscriptionPlanCacheTTL())
	return &plan, nil
}

func CountUserSubscriptionsByPlan(userId int, planId int) (int64, error) {
	if userId <= 0 || planId <= 0 {
		return 0, errors.New("invalid userId or planId")
	}
	var count int64
	if err := DB.Model(&UserSubscription{}).
		Where("user_id = ? AND plan_id = ?", userId, planId).
		Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

func getUserGroupByIdTx(tx *gorm.DB, userId int) (string, error) {
	if userId <= 0 {
		return "", errors.New("invalid userId")
	}
	if tx == nil {
		tx = DB
	}
	var group string
	if err := tx.Model(&User{}).Where("id = ?", userId).Select(commonGroupCol).Find(&group).Error; err != nil {
		return "", err
	}
	return group, nil
}

func downgradeUserGroupForSubscriptionTx(tx *gorm.DB, sub *UserSubscription, now int64) (string, error) {
	if tx == nil || sub == nil {
		return "", errors.New("invalid downgrade args")
	}
	upgradeGroup := strings.TrimSpace(sub.UpgradeGroup)
	if upgradeGroup == "" {
		return "", nil
	}
	currentGroup, err := getUserGroupByIdTx(tx, sub.UserId)
	if err != nil {
		return "", err
	}
	if currentGroup != upgradeGroup {
		return "", nil
	}
	var activeSub UserSubscription
	activeQuery := tx.Where("user_id = ? AND status = ? AND start_time <= ? AND end_time > ? AND id <> ? AND upgrade_group <> ''",
		sub.UserId, "active", now, now, sub.Id).
		Order("end_time desc, id desc").
		Limit(1).
		Find(&activeSub)
	if activeQuery.Error == nil && activeQuery.RowsAffected > 0 {
		return "", nil
	}
	prevGroup := strings.TrimSpace(sub.PrevUserGroup)
	if prevGroup == "" || prevGroup == currentGroup {
		return "", nil
	}
	if err := tx.Model(&User{}).Where("id = ?", sub.UserId).
		Update("group", prevGroup).Error; err != nil {
		return "", err
	}
	return prevGroup, nil
}

func CreateUserSubscriptionFromPlanTx(tx *gorm.DB, userId int, plan *SubscriptionPlan, source string, paidRevenueQuota ...int64) (*UserSubscription, error) {
	return CreateUserSubscriptionFromPlanWithOptionsTx(tx, userId, plan, source, CreateUserSubscriptionOptions{}, paidRevenueQuota...)
}

type CreateUserSubscriptionOptions struct {
	StartTime         int64
	RenewedFromId     *int
	Remark            string
	PlanSnapshot      string
	SkipPurchaseLimit bool
}

func CreateUserSubscriptionFromPlanWithOptionsTx(tx *gorm.DB, userId int, plan *SubscriptionPlan, source string, options CreateUserSubscriptionOptions, paidRevenueQuota ...int64) (*UserSubscription, error) {
	if tx == nil {
		return nil, errors.New("tx is nil")
	}
	if plan == nil || plan.Id == 0 {
		return nil, errors.New("invalid plan")
	}
	if userId <= 0 {
		return nil, errors.New("invalid user id")
	}
	if plan.MaxPurchasePerUser > 0 && !options.SkipPurchaseLimit {
		var count int64
		if err := tx.Model(&UserSubscription{}).
			Where("user_id = ? AND plan_id = ?", userId, plan.Id).
			Count(&count).Error; err != nil {
			return nil, err
		}
		if count >= int64(plan.MaxPurchasePerUser) {
			return nil, errors.New("已达到该套餐购买上限")
		}
	}
	nowUnix := options.StartTime
	if nowUnix <= 0 {
		nowUnix = GetDBTimestamp()
	}
	now := time.Unix(nowUnix, 0)
	endUnix, err := calcPlanEndTime(now, plan)
	if err != nil {
		return nil, err
	}
	resetBase := now
	nextReset := calcNextResetTime(resetBase, plan, endUnix, now.Unix())
	lastReset := int64(0)
	if nextReset > 0 {
		lastReset = now.Unix()
	}
	upgradeGroup := strings.TrimSpace(plan.UpgradeGroup)
	prevGroup := ""
	if upgradeGroup != "" && strings.TrimSpace(plan.AllowedGroup) == "" {
		currentGroup, err := getUserGroupByIdTx(tx, userId)
		if err != nil {
			return nil, err
		}
		if currentGroup != upgradeGroup {
			prevGroup = currentGroup
			if err := tx.Model(&User{}).Where("id = ?", userId).
				Update("group", upgradeGroup).Error; err != nil {
				return nil, err
			}
		}
	}
	revenueQuota := int64(0)
	if len(paidRevenueQuota) > 0 && paidRevenueQuota[0] > 0 {
		revenueQuota = paidRevenueQuota[0]
	}
	planSnapshot := strings.TrimSpace(options.PlanSnapshot)
	if planSnapshot == "" {
		var snapshotErr error
		planSnapshot, snapshotErr = BuildSubscriptionPlanSnapshot(plan)
		if snapshotErr != nil {
			return nil, snapshotErr
		}
	}
	sub := &UserSubscription{
		UserId:           userId,
		PlanId:           plan.Id,
		PlanSnapshot:     planSnapshot,
		PlanTitle:        plan.Title,
		Remark:           strings.TrimSpace(options.Remark),
		RenewedFromId:    options.RenewedFromId,
		AmountTotal:      plan.TotalAmount,
		AmountUsed:       0,
		AmountCap:        plan.AmountCap,
		AmountCapUsed:    0,
		StartTime:        now.Unix(),
		EndTime:          endUnix,
		Status:           "active",
		Source:           source,
		LastResetTime:    lastReset,
		NextResetTime:    nextReset,
		UpgradeGroup:     upgradeGroup,
		PrevUserGroup:    prevGroup,
		AllowedGroup:     strings.TrimSpace(plan.AllowedGroup),
		PaidRevenueQuota: revenueQuota,
		DividendState:    SubscriptionDividendPending,
		CreatedAt:        common.GetTimestamp(),
		UpdatedAt:        common.GetTimestamp(),
	}
	if err := tx.Create(sub).Error; err != nil {
		return nil, err
	}
	return sub, nil
}

// Complete a subscription order (idempotent). Creates a UserSubscription snapshot from the plan.
// expectedPaymentProvider guards against cross-gateway callback attacks (empty skips the check).
// actualPaymentMethod updates the order's PaymentMethod to reflect the real payment type used (empty skips update).
func CompleteSubscriptionOrder(tradeNo string, providerPayload string, expectedPaymentProvider string, actualPaymentMethod string) error {
	if tradeNo == "" {
		return errors.New("tradeNo is empty")
	}
	refCol := "`trade_no`"
	if common.UsingPostgreSQL {
		refCol = `"trade_no"`
	}
	var logUserId int
	var logPlanTitle string
	var logMoney float64
	var logPaymentMethod string
	var upgradeGroup string
	var renewalTokenKeys []string
	err := DB.Transaction(func(tx *gorm.DB) error {
		var order SubscriptionOrder
		if err := tx.Set("gorm:query_option", "FOR UPDATE").Where(refCol+" = ?", tradeNo).First(&order).Error; err != nil {
			return ErrSubscriptionOrderNotFound
		}
		if expectedPaymentProvider != "" && order.PaymentProvider != expectedPaymentProvider {
			return ErrPaymentMethodMismatch
		}
		if order.Status == common.TopUpStatusSuccess {
			return nil
		}
		if order.Status != common.TopUpStatusPending {
			return ErrSubscriptionOrderStatusInvalid
		}
		plan, err := ParseSubscriptionPlanSnapshot(order.PlanSnapshot)
		if err != nil {
			// Backward compatibility for orders created before full snapshots
			// were introduced.
			plan, err = GetSubscriptionPlanById(order.PlanId)
			if err != nil {
				return err
			}
			order.PlanSnapshot, err = BuildSubscriptionPlanSnapshot(plan)
			if err != nil {
				return err
			}
		}
		if !plan.Enabled {
			// still allow completion for already purchased orders
		}
		upgradeGroup = strings.TrimSpace(plan.UpgradeGroup)
		revenueQuota, quotaErr := calcSubscriptionBalanceQuota(order.Money)
		if quotaErr != nil {
			return quotaErr
		}
		var createdSub *UserSubscription
		if order.RenewFromSubscriptionId != nil && *order.RenewFromSubscriptionId > 0 {
			createdSub, renewalTokenKeys, err = createRenewalSubscriptionFromOrderTx(tx, &order, plan, int64(revenueQuota))
			if err != nil {
				return err
			}
		} else {
			createdSub, err = CreateUserSubscriptionFromPlanWithOptionsTx(
				tx,
				order.UserId,
				plan,
				"order",
				CreateUserSubscriptionOptions{PlanSnapshot: order.PlanSnapshot},
				int64(revenueQuota),
			)
			if err != nil {
				return err
			}
		}
		if _, err := GrantSubscriptionPurchaseLuckyCardsTx(tx, &order, createdSub); err != nil {
			return err
		}
		if err := upsertSubscriptionTopUpTx(tx, &order); err != nil {
			return err
		}
		if _, err := RecordRechargeCreditTx(
			tx,
			order.UserId,
			MoneyToRechargeCents(order.Money),
			"topup",
			order.TradeNo,
			common.GetTimestamp(),
		); err != nil {
			return err
		}
		order.Status = common.TopUpStatusSuccess
		order.CompleteTime = common.GetTimestamp()
		if providerPayload != "" {
			order.ProviderPayload = providerPayload
		}
		if actualPaymentMethod != "" && order.PaymentMethod != actualPaymentMethod {
			order.PaymentMethod = actualPaymentMethod
		}
		if err := tx.Save(&order).Error; err != nil {
			return err
		}
		logUserId = order.UserId
		logPlanTitle = plan.Title
		logMoney = order.Money
		logPaymentMethod = order.PaymentMethod
		return nil
	})
	if err != nil {
		return err
	}
	if upgradeGroup != "" && logUserId > 0 {
		_ = UpdateUserGroupCache(logUserId, upgradeGroup)
	}
	if logUserId > 0 {
		_ = InvalidateUserCache(logUserId)
		if common.RedisEnabled {
			for _, tokenKey := range renewalTokenKeys {
				_ = cacheDeleteToken(tokenKey)
			}
		}
		msg := fmt.Sprintf("订阅购买成功，套餐: %s，支付金额: %.2f，支付方式: %s", logPlanTitle, logMoney, logPaymentMethod)
		RecordLog(logUserId, LogTypeTopup, msg)
	}
	return nil
}

func upsertSubscriptionTopUpTx(tx *gorm.DB, order *SubscriptionOrder) error {
	if tx == nil || order == nil {
		return errors.New("invalid subscription order")
	}
	now := common.GetTimestamp()
	var topup TopUp
	if err := tx.Where("trade_no = ?", order.TradeNo).First(&topup).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			topup = TopUp{
				UserId:        order.UserId,
				Amount:        0,
				Money:         order.Money,
				TradeNo:       order.TradeNo,
				PaymentMethod: order.PaymentMethod,
				CreateTime:    order.CreateTime,
				CompleteTime:  now,
				Status:        common.TopUpStatusSuccess,
			}
			// 订阅购买不改 user.quota（额度走订阅系统），故快照 quotaToAdd=0，
			// 仅记录完成时刻的 (quota+gift_quota) 余额，保持充值记录字段一致。
			topup.BalanceAfter = snapshotBalanceAfterRecharge(tx, order.UserId, 0)
			return tx.Create(&topup).Error
		}
		return err
	}
	topup.Money = order.Money
	if topup.PaymentMethod == "" {
		topup.PaymentMethod = order.PaymentMethod
	} else if topup.PaymentMethod != order.PaymentMethod {
		return ErrPaymentMethodMismatch
	}
	if topup.CreateTime == 0 {
		topup.CreateTime = order.CreateTime
	}
	topup.CompleteTime = now
	topup.Status = common.TopUpStatusSuccess
	// 订阅购买不改 user.quota，快照 quotaToAdd=0，记录完成时刻余额。
	topup.BalanceAfter = snapshotBalanceAfterRecharge(tx, order.UserId, 0)
	return tx.Save(&topup).Error
}

func ExpireSubscriptionOrder(tradeNo string, expectedPaymentProvider string) error {
	if tradeNo == "" {
		return errors.New("tradeNo is empty")
	}
	refCol := "`trade_no`"
	if common.UsingPostgreSQL {
		refCol = `"trade_no"`
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		var order SubscriptionOrder
		if err := tx.Set("gorm:query_option", "FOR UPDATE").Where(refCol+" = ?", tradeNo).First(&order).Error; err != nil {
			return ErrSubscriptionOrderNotFound
		}
		if expectedPaymentProvider != "" && order.PaymentProvider != expectedPaymentProvider {
			return ErrPaymentMethodMismatch
		}
		if order.Status != common.TopUpStatusPending {
			return nil
		}
		order.Status = common.TopUpStatusExpired
		order.CompleteTime = common.GetTimestamp()
		return tx.Save(&order).Error
	})
}

// Admin bind (no payment). Creates a UserSubscription from a plan.
func AdminBindSubscription(userId int, planId int, sourceNote string) (string, error) {
	if userId <= 0 || planId <= 0 {
		return "", errors.New("invalid userId or planId")
	}
	plan, err := GetSubscriptionPlanById(planId)
	if err != nil {
		return "", err
	}
	err = DB.Transaction(func(tx *gorm.DB) error {
		_, err := CreateUserSubscriptionFromPlanTx(tx, userId, plan, "admin")
		return err
	})
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(plan.UpgradeGroup) != "" {
		_ = UpdateUserGroupCache(userId, plan.UpgradeGroup)
		return fmt.Sprintf("用户分组将升级到 %s", plan.UpgradeGroup), nil
	}
	return "", nil
}

func calcSubscriptionBalanceQuota(priceAmount float64) (int, error) {
	if priceAmount <= 0 {
		return 0, nil
	}
	if common.QuotaPerUnit <= 0 {
		return 0, errors.New("额度单位配置错误")
	}
	quota := decimal.NewFromFloat(priceAmount).
		Mul(decimal.NewFromFloat(common.QuotaPerUnit)).
		Ceil().
		IntPart()
	return int(quota), nil
}

// PurchaseSubscriptionWithBalance creates a subscription by deducting the user's wallet quota.
func PurchaseSubscriptionWithBalance(userId int, planId int) error {
	return PurchaseSubscriptionWithBalanceRenewal(userId, planId, 0)
}

func PurchaseSubscriptionWithBalanceRenewal(userId int, planId int, renewFromSubscriptionId int) error {
	if userId <= 0 || planId <= 0 {
		return errors.New("invalid userId or planId")
	}

	var logPlanTitle string
	var logMoney float64
	var chargedQuota int
	var upgradeGroup string
	var tradeNo string
	var renewalTokenKeys []string
	err := DB.Transaction(func(tx *gorm.DB) error {
		var plan *SubscriptionPlan
		var err error
		if renewFromSubscriptionId > 0 {
			var source UserSubscription
			if err := tx.Set("gorm:query_option", "FOR UPDATE").
				Where("id = ? AND user_id = ?", renewFromSubscriptionId, userId).
				First(&source).Error; err != nil {
				return err
			}
			plan, err = getSubscriptionPlanByIdTx(tx, source.PlanId)
			if err != nil {
				return err
			}
			if !plan.Enabled {
				if plan.RenewalPlanId == nil || *plan.RenewalPlanId <= 0 {
					return ErrSubscriptionRenewalNotAvailable
				}
				plan, err = getSubscriptionPlanByIdTx(tx, *plan.RenewalPlanId)
				if err != nil {
					return err
				}
			}
			if plan.Id != planId {
				return errors.New("续费套餐已变化，请刷新后重新确认")
			}
		} else {
			plan, err = getSubscriptionPlanByIdTx(tx, planId)
			if err != nil {
				return err
			}
		}
		if !plan.Enabled {
			return errors.New("套餐未启用")
		}
		if plan.PriceAmount < 0 {
			return errors.New("套餐价格不能为负数")
		}
		if plan.AllowBalancePay != nil && !*plan.AllowBalancePay {
			return errors.New("该套餐不允许使用余额兑换")
		}

		requiredQuota, err := calcSubscriptionBalanceQuota(plan.PriceAmount)
		if err != nil {
			return err
		}

		var user User
		if err := tx.Set("gorm:query_option", "FOR UPDATE").Where("id = ?", userId).First(&user).Error; err != nil {
			return err
		}
		if requiredQuota > 0 && user.Quota < requiredQuota {
			return errors.New("余额不足")
		}
		if requiredQuota > 0 {
			if err := tx.Model(&User{}).Where("id = ?", userId).
				Update("quota", gorm.Expr("quota - ?", requiredQuota)).Error; err != nil {
				return err
			}
		}

		now := common.GetTimestamp()
		tradeNo = fmt.Sprintf("SUBBALUSR%dNO%s%d", userId, common.GetRandomString(6), time.Now().UnixNano())
		order, err := NewSubscriptionOrderFromPlan(
			userId, plan, tradeNo, PaymentMethodBalance, PaymentProviderBalance,
		)
		if err != nil {
			return err
		}
		order.Status = common.TopUpStatusSuccess
		order.CreateTime = now
		order.CompleteTime = now
		order.ProviderPayload = fmt.Sprintf("charged_quota=%d", requiredQuota)
		if renewFromSubscriptionId > 0 {
			order.RenewFromSubscriptionId = &renewFromSubscriptionId
		}
		if err := tx.Create(order).Error; err != nil {
			return err
		}
		var createdSub *UserSubscription
		if renewFromSubscriptionId > 0 {
			if createdSub, renewalTokenKeys, err = createRenewalSubscriptionFromOrderTx(
				tx,
				order,
				plan,
				int64(requiredQuota),
			); err != nil {
				return err
			}
		} else {
			if createdSub, err = CreateUserSubscriptionFromPlanWithOptionsTx(
				tx,
				userId,
				plan,
				PaymentMethodBalance,
				CreateUserSubscriptionOptions{PlanSnapshot: order.PlanSnapshot},
				int64(requiredQuota),
			); err != nil {
				return err
			}
		}
		if _, err := GrantSubscriptionPurchaseLuckyCardsTx(tx, order, createdSub); err != nil {
			return err
		}

		logPlanTitle = plan.Title
		logMoney = plan.PriceAmount
		chargedQuota = requiredQuota
		upgradeGroup = strings.TrimSpace(plan.UpgradeGroup)
		return nil
	})
	if err != nil {
		return err
	}

	if chargedQuota > 0 {
		if err := cacheDecrUserQuota(userId, int64(chargedQuota)); err != nil {
			common.SysLog("failed to decrease user quota cache after subscription balance purchase: " + err.Error())
		}
	}
	if upgradeGroup != "" {
		_ = UpdateUserGroupCache(userId, upgradeGroup)
	}
	if common.RedisEnabled {
		for _, tokenKey := range renewalTokenKeys {
			_ = cacheDeleteToken(tokenKey)
		}
	}
	msg := fmt.Sprintf("使用余额购买订阅成功，套餐: %s，支付金额: %.2f，扣除额度: %d", logPlanTitle, logMoney, chargedQuota)
	RecordLog(userId, LogTypeTopup, msg)
	return nil
}

// GetAllActiveUserSubscriptions returns all active subscriptions for a user.
func GetAllActiveUserSubscriptions(userId int) ([]SubscriptionSummary, error) {
	if userId <= 0 {
		return nil, errors.New("invalid userId")
	}
	now := common.GetTimestamp()
	var subs []UserSubscription
	err := DB.Where("user_id = ? AND status = ? AND start_time <= ? AND end_time > ?", userId, "active", now, now).
		Order("end_time desc, id desc").
		Find(&subs).Error
	if err != nil {
		return nil, err
	}
	return buildSubscriptionSummaries(subs), nil
}

// HasActiveUserSubscription returns whether the user has any active subscription.
// This is a lightweight existence check to avoid heavy pre-consume transactions.
func HasActiveUserSubscription(userId int) (bool, error) {
	if userId <= 0 {
		return false, errors.New("invalid userId")
	}
	now := common.GetTimestamp()
	var count int64
	if err := DB.Model(&UserSubscription{}).
		Where("user_id = ? AND status = ? AND start_time <= ? AND end_time > ?", userId, "active", now, now).
		Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

// HasActiveUserSubscriptionByGroup 检查用户是否有绑定指定分组的有效订阅(订阅即凭证: auth 旁路放行该分组)。
func HasActiveUserSubscriptionByGroup(userId int, group string) (bool, error) {
	if userId <= 0 || strings.TrimSpace(group) == "" {
		return false, nil
	}
	now := common.GetTimestamp()
	var count int64
	if err := DB.Model(&UserSubscription{}).
		Where("user_id = ? AND status = ? AND start_time <= ? AND end_time > ? AND allowed_group = ?", userId, "active", now, now, group).
		Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

// GetActiveUserSubscriptionAllowedGroups 返回用户有效订阅绑定的 AllowedGroup 列表(去重, 供分组下拉/凭证用)。
func GetActiveUserSubscriptionAllowedGroups(userId int) ([]string, error) {
	if userId <= 0 {
		return nil, nil
	}
	now := common.GetTimestamp()
	var groups []string
	err := DB.Model(&UserSubscription{}).
		Where("user_id = ? AND status = ? AND start_time <= ? AND end_time > ? AND allowed_group != ''", userId, "active", now, now).
		Distinct("allowed_group").
		Pluck("allowed_group", &groups).Error
	if err != nil {
		return nil, err
	}
	return groups, nil
}

// GetAllUserSubscriptions returns all subscriptions (active and expired) for a user.
func GetAllUserSubscriptions(userId int) ([]SubscriptionSummary, error) {
	if userId <= 0 {
		return nil, errors.New("invalid userId")
	}
	var subs []UserSubscription
	err := DB.Where("user_id = ?", userId).
		Order("end_time desc, id desc").
		Find(&subs).Error
	if err != nil {
		return nil, err
	}
	return buildSubscriptionSummaries(subs), nil
}

// SubscriberSummary 订阅用户汇总(超管「订阅用户列表」用)
type SubscriberSummary struct {
	UserId      int    `json:"user_id" gorm:"column:user_id"`
	Username    string `json:"username" gorm:"column:username"`
	TotalCount  int    `json:"total_count" gorm:"column:total_count"`
	ActiveCount int    `json:"active_count" gorm:"column:active_count"`
}

// GetSubscriptionSubscribers 返回所有买过套餐的用户(含总订阅数 + 未到期数)。
func GetSubscriptionSubscribers() ([]SubscriberSummary, error) {
	var results []SubscriberSummary
	now := GetDBTimestamp()
	err := DB.Model(&UserSubscription{}).
		Select("user_subscriptions.user_id as user_id, users.username as username, COUNT(*) as total_count, SUM(CASE WHEN user_subscriptions.status = 'active' AND user_subscriptions.end_time > ? THEN 1 ELSE 0 END) as active_count", now).
		Joins("JOIN users ON users.id = user_subscriptions.user_id").
		Group("user_subscriptions.user_id").
		Order("total_count desc").
		Find(&results).Error
	return results, err
}

func buildSubscriptionSummaries(subs []UserSubscription) []SubscriptionSummary {
	if len(subs) == 0 {
		return []SubscriptionSummary{}
	}
	result := make([]SubscriptionSummary, 0, len(subs))
	for _, sub := range subs {
		subCopy := sub
		result = append(result, SubscriptionSummary{
			Subscription: &subCopy,
		})
	}
	return result
}

// AdminInvalidateUserSubscription marks a user subscription as cancelled and ends it immediately.
func AdminInvalidateUserSubscription(userSubscriptionId int) (string, error) {
	if userSubscriptionId <= 0 {
		return "", errors.New("invalid userSubscriptionId")
	}
	now := common.GetTimestamp()
	cacheGroup := ""
	downgradeGroup := ""
	var userId int
	var sub UserSubscription
	err := DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Set("gorm:query_option", "FOR UPDATE").
			Where("id = ?", userSubscriptionId).First(&sub).Error; err != nil {
			return err
		}
		userId = sub.UserId
		if err := tx.Model(&sub).Updates(map[string]interface{}{
			"status":            "cancelled",
			"end_time":          now,
			"dividend_ready_at": now,
			"updated_at":        now,
		}).Error; err != nil {
			return err
		}
		target, err := downgradeUserGroupForSubscriptionTx(tx, &sub, now)
		if err != nil {
			return err
		}
		if target != "" {
			cacheGroup = target
			downgradeGroup = target
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	if cacheGroup != "" && userId > 0 {
		_ = UpdateUserGroupCache(userId, cacheGroup)
	}
	// 失效分润(post-commit): 按实际利润结算(利润≤0不分润)
	settleSubscriptionEndForSub(&sub)
	if downgradeGroup != "" {
		return fmt.Sprintf("用户分组将回退到 %s", downgradeGroup), nil
	}
	return "", nil
}

// AdminDeleteUserSubscription hard-deletes a user subscription.
func AdminDeleteUserSubscription(userSubscriptionId int) (string, error) {
	if userSubscriptionId <= 0 {
		return "", errors.New("invalid userSubscriptionId")
	}
	now := common.GetTimestamp()
	cacheGroup := ""
	downgradeGroup := ""
	var userId int
	var sub UserSubscription
	err := DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Set("gorm:query_option", "FOR UPDATE").
			Where("id = ?", userSubscriptionId).First(&sub).Error; err != nil {
			return err
		}
		var relationCount int64
		relationQueries := []*gorm.DB{
			tx.Model(&Token{}).Where("subscription_id = ? OR planned_subscription_id = ?", userSubscriptionId, userSubscriptionId),
			tx.Model(&SubscriptionPreConsumeRecord{}).Where("user_subscription_id = ?", userSubscriptionId),
			tx.Model(&TokenSubscriptionBindingHistory{}).Where("from_subscription_id = ? OR to_subscription_id = ?", userSubscriptionId, userSubscriptionId),
			tx.Model(&UserSubscription{}).Where("renewed_from_id = ?", userSubscriptionId),
			tx.Model(&SubscriptionOrder{}).Where("renew_from_subscription_id = ?", userSubscriptionId),
		}
		for _, query := range relationQueries {
			var count int64
			if err := query.Count(&count).Error; err != nil {
				return err
			}
			relationCount += count
		}
		if sub.RenewedFromId != nil {
			relationCount++
		}
		if relationCount > 0 {
			return errors.New("该订阅实例存在 API Key 绑定、续费、计费或审计记录，只能作废，不能彻底删除")
		}
		userId = sub.UserId
		target, err := downgradeUserGroupForSubscriptionTx(tx, &sub, now)
		if err != nil {
			return err
		}
		if target != "" {
			cacheGroup = target
			downgradeGroup = target
		}
		if err := tx.Where("id = ?", userSubscriptionId).Delete(&UserSubscription{}).Error; err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	if cacheGroup != "" && userId > 0 {
		_ = UpdateUserGroupCache(userId, cacheGroup)
	}
	if downgradeGroup != "" {
		return fmt.Sprintf("用户分组将回退到 %s", downgradeGroup), nil
	}
	return "", nil
}

// settleSubscriptionEndForSub 套餐结束时按实际利润分润。
// 管理员硬删除表示订阅作废，删除路径不得调用本函数。
func settleSubscriptionEndForSub(sub *UserSubscription) {
	if sub == nil || sub.UserId <= 0 {
		return
	}
	SettleSubscriptionEndDividend(sub.UserId, sub.Id)
}

type SubscriptionPreConsumeResult struct {
	UserSubscriptionId int
	PreConsumed        int64
	AmountTotal        int64
	AmountUsedBefore   int64
	AmountUsedAfter    int64
}

func subscriptionCandidatesTx(tx *gorm.DB, userId int, usingGroup string, now int64, binding *Token) ([]UserSubscription, error) {
	if tx == nil {
		return nil, errors.New("tx is nil")
	}
	if binding == nil || NormalizeTokenSubscriptionMode(binding.SubscriptionMode) != TokenSubscriptionModeInstance ||
		binding.SubscriptionId <= 0 {
		var subs []UserSubscription
		if err := tx.Set("gorm:query_option", "FOR UPDATE").
			Where("user_id = ? AND status = ? AND start_time <= ? AND end_time > ? AND (allowed_group = '' OR allowed_group = ?)", userId, "active", now, now, usingGroup).
			Order("end_time asc, id asc").
			Find(&subs).Error; err != nil {
			return nil, err
		}
		if err := SortSubscriptionsByPreferenceTx(tx, userId, usingGroup, subs); err != nil {
			return nil, err
		}
		return subs, nil
	}

	candidates := make([]UserSubscription, 0, 8)
	seen := map[int]struct{}{}
	appendById := func(id int) error {
		if id <= 0 {
			return nil
		}
		if _, ok := seen[id]; ok {
			return nil
		}
		var sub UserSubscription
		if err := tx.Set("gorm:query_option", "FOR UPDATE").
			Where("id = ? AND user_id = ?", id, userId).
			First(&sub).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil
			}
			return err
		}
		seen[id] = struct{}{}
		candidates = append(candidates, sub)
		return nil
	}
	if err := appendById(binding.SubscriptionId); err != nil {
		return nil, err
	}

	if binding.SubscriptionAllowRenewal {
		cursor := binding.SubscriptionId
		for hop := 0; hop < 64; hop++ {
			var successor UserSubscription
			result := tx.Set("gorm:query_option", "FOR UPDATE").
				Where("renewed_from_id = ? AND user_id = ?", cursor, userId).
				Limit(1).
				Find(&successor)
			if result.Error != nil {
				return nil, result.Error
			}
			if result.RowsAffected == 0 {
				break
			}
			if err := appendById(successor.Id); err != nil {
				return nil, err
			}
			cursor = successor.Id
		}
	}

	if binding.SubscriptionAllowSameGroup {
		var sameGroup []UserSubscription
		if err := tx.Set("gorm:query_option", "FOR UPDATE").
			Where("user_id = ? AND status = ? AND start_time <= ? AND end_time > ? AND (allowed_group = '' OR allowed_group = ?)", userId, "active", now, now, usingGroup).
			Order("end_time asc, id asc").
			Find(&sameGroup).Error; err != nil {
			return nil, err
		}
		for _, sub := range sameGroup {
			if _, ok := seen[sub.Id]; ok {
				continue
			}
			seen[sub.Id] = struct{}{}
			candidates = append(candidates, sub)
		}
	}
	return candidates, nil
}

// ExpireDueSubscriptions marks expired subscriptions and handles group downgrade.
func ExpireDueSubscriptions(limit int) (int, error) {
	if limit <= 0 {
		limit = 200
	}
	now := GetDBTimestamp()
	var subs []UserSubscription
	if err := DB.Where("status = ? AND end_time > 0 AND end_time <= ?", "active", now).
		Order("end_time asc, id asc").
		Limit(limit).
		Find(&subs).Error; err != nil {
		return 0, err
	}
	if len(subs) == 0 {
		return 0, nil
	}
	expiredCount := 0
	userIds := make(map[int]struct{}, len(subs))
	for _, sub := range subs {
		if sub.UserId > 0 {
			userIds[sub.UserId] = struct{}{}
		}
	}
	for userId := range userIds {
		cacheGroup := ""
		err := DB.Transaction(func(tx *gorm.DB) error {
			res := tx.Model(&UserSubscription{}).
				Where("user_id = ? AND status = ? AND end_time > 0 AND end_time <= ?", userId, "active", now).
				Updates(map[string]interface{}{
					"status":            "expired",
					"dividend_ready_at": now,
					"updated_at":        common.GetTimestamp(),
				})
			if res.Error != nil {
				return res.Error
			}
			expiredCount += int(res.RowsAffected)

			// If there's an active upgraded subscription, keep current group.
			var activeSub UserSubscription
			activeQuery := tx.Where("user_id = ? AND status = ? AND start_time <= ? AND end_time > ? AND upgrade_group <> ''",
				userId, "active", now, now).
				Order("end_time desc, id desc").
				Limit(1).
				Find(&activeSub)
			if activeQuery.Error == nil && activeQuery.RowsAffected > 0 {
				return nil
			}

			// No active upgraded subscription, downgrade to previous group if needed.
			var lastExpired UserSubscription
			expiredQuery := tx.Where("user_id = ? AND status = ? AND upgrade_group <> ''",
				userId, "expired").
				Order("end_time desc, id desc").
				Limit(1).
				Find(&lastExpired)
			if expiredQuery.Error != nil || expiredQuery.RowsAffected == 0 {
				return nil
			}
			upgradeGroup := strings.TrimSpace(lastExpired.UpgradeGroup)
			prevGroup := strings.TrimSpace(lastExpired.PrevUserGroup)
			if upgradeGroup == "" || prevGroup == "" {
				return nil
			}
			currentGroup, err := getUserGroupByIdTx(tx, userId)
			if err != nil {
				return err
			}
			if currentGroup != upgradeGroup || currentGroup == prevGroup {
				return nil
			}
			if err := tx.Model(&User{}).Where("id = ?", userId).
				Update("group", prevGroup).Error; err != nil {
				return err
			}
			cacheGroup = prevGroup
			return nil
		})
		if err != nil {
			return expiredCount, err
		}
		if cacheGroup != "" {
			_ = UpdateUserGroupCache(userId, cacheGroup)
		}
	}

	// 到期不立即分润: 交给 SettleDelayedSubscriptionDividend 在到期 24h 后扫描
	// (等 Codex 异步任务结算完写 log, 避免 log 不完整→成本漏算→利润虚高)
	return expiredCount, nil
}

// SettleDelayedSubscriptionDividend 扫描已到期超过 delaySeconds(默认24h) 的订阅, 触发结束分润。
// 延迟目的: 等 Codex 异步任务结算完(写 log), 避免 log 不完整→成本漏算→利润虚高。
// 幂等: SettleSubscriptionEndDividend 内部 sourceRef 去重, 已分的秒跳过。
// Order end_time desc 优先扫新到期的, 由 subscription quota reset task 每分钟调用。
func SettleDelayedSubscriptionDividend(delaySeconds int64, limit int) (int, error) {
	if delaySeconds <= 0 {
		delaySeconds = 24 * 3600
	}
	if limit <= 0 {
		limit = 200
	}
	now := GetDBTimestamp()
	cutoff := now - delaySeconds
	query := DB.Where("status IN ? AND ((dividend_ready_at > 0 AND dividend_ready_at <= ?) OR (dividend_ready_at = 0 AND end_time > 0 AND end_time <= ?))",
		[]string{"expired", "cancelled"}, cutoff, cutoff).
		Where("dividend_state = ? OR dividend_state = ''", SubscriptionDividendPending)
	// 部署时间分界: 只扫部署后到期的, 排除历史(历史由人工/追回处理, 不走新机制扫描)
	// 避免给旧机制漏分的套餐补发分润
	if deployCutoff := getSubEndDividendCutoff(); deployCutoff > 0 {
		query = query.Where("end_time > ?", deployCutoff)
	}
	var subs []UserSubscription
	if err := query.Order("end_time desc, id desc").Limit(limit).Find(&subs).Error; err != nil {
		return 0, err
	}
	for _, sub := range subs {
		if sub.UserId <= 0 {
			continue
		}
		SettleSubscriptionEndDividend(sub.UserId, sub.Id)
	}
	return len(subs), nil
}

// getSubEndDividendCutoff 读取 option 'SubEndDividendCutoff'(部署时间戳, 秒, 0=不限)。
// 用于排除部署前的历史套餐, 避免新机制扫描给旧漏分套餐补发分润。
// 部署后由超管设此 option = 部署时刻, 即可把所有历史套餐挡在扫描之外。
func getSubEndDividendCutoff() int64 {
	var opt Option
	if err := DB.Where("`key` = ?", "SubEndDividendCutoff").First(&opt).Error; err != nil {
		return 0
	}
	var n int64
	fmt.Sscanf(opt.Value, "%d", &n)
	return n
}

// SubscriptionPreConsumeRecord stores idempotent pre-consume operations per request.
type SubscriptionPreConsumeRecord struct {
	Id                  int    `json:"id"`
	RequestId           string `json:"request_id" gorm:"type:varchar(64);uniqueIndex"`
	UserId              int    `json:"user_id" gorm:"index"`
	UserSubscriptionId  int    `json:"user_subscription_id" gorm:"index"`
	PreConsumed         int64  `json:"pre_consumed" gorm:"type:bigint;not null;default:0"`
	FinalSaleQuota      int64  `json:"final_sale_quota" gorm:"type:bigint;not null;default:0"`
	ChannelId           int    `json:"channel_id" gorm:"index;not null;default:0"`
	ChannelCostRatioPPM *int64 `json:"channel_cost_ratio_ppm" gorm:"column:channel_cost_ratio_ppm;default:null"`
	CostNumerator       int64  `json:"cost_numerator" gorm:"type:bigint;not null;default:0"`
	Status              string `json:"status" gorm:"type:varchar(32);index"` // reserved/provisional/final/refunded
	CreatedAt           int64  `json:"created_at" gorm:"bigint"`
	UpdatedAt           int64  `json:"updated_at" gorm:"bigint;index"`
}

func (r *SubscriptionPreConsumeRecord) BeforeCreate(tx *gorm.DB) error {
	now := common.GetTimestamp()
	r.CreatedAt = now
	r.UpdatedAt = now
	return nil
}

func (r *SubscriptionPreConsumeRecord) BeforeUpdate(tx *gorm.DB) error {
	r.UpdatedAt = common.GetTimestamp()
	return nil
}

func maybeResetUserSubscriptionWithPlanTx(tx *gorm.DB, sub *UserSubscription, plan *SubscriptionPlan, now int64) error {
	if tx == nil || sub == nil || plan == nil {
		return errors.New("invalid reset args")
	}
	if sub.NextResetTime > 0 && sub.NextResetTime > now {
		return nil
	}
	if NormalizeResetPeriod(plan.QuotaResetPeriod) == SubscriptionResetNever {
		return nil
	}
	baseUnix := sub.LastResetTime
	if baseUnix <= 0 {
		baseUnix = sub.StartTime
	}
	base := time.Unix(baseUnix, 0)
	next := calcNextResetTime(base, plan, sub.EndTime, sub.StartTime)
	advanced := false
	resetEpochs := make([]int64, 0, 4)
	for next > 0 && next <= now {
		advanced = true
		resetEpochs = append(resetEpochs, next)
		base = time.Unix(next, 0)
		next = calcNextResetTime(base, plan, sub.EndTime, sub.StartTime)
	}
	if !advanced {
		if sub.NextResetTime == 0 && next > 0 {
			sub.NextResetTime = next
			sub.LastResetTime = base.Unix()
			return tx.Save(sub).Error
		}
		return nil
	}
	sub.AmountUsed = 0
	sub.LastResetTime = base.Unix()
	sub.NextResetTime = next
	if err := tx.Save(sub).Error; err != nil {
		return err
	}
	if !plan.LuckyCardOnReset || sub.LuckyCardDisabled || sub.EndTime <= now {
		return nil
	}
	campaign, rule, err := GetLuckyCampaignTx(tx, true)
	if err != nil {
		return err
	}
	for _, epoch := range resetEpochs {
		if _, err := GrantLuckyCardsTx(tx, campaign, rule, LuckyCardGrant{
			UserId: sub.UserId, PoolType: LuckyPoolSubscription,
			SourceType: "subscription_reset", SourceRef: fmt.Sprintf("%d", sub.Id),
			SourceSubscriptionId: sub.Id, SourceCycleKey: fmt.Sprintf("%d", epoch),
			SourceSnapshot: sub.PlanSnapshot, SourceEffectiveEndTime: sub.EndTime,
			ExpiresAt: sub.EndTime, GrantKeyPrefix: fmt.Sprintf("reset:%d:%d", sub.Id, epoch), Count: 1,
		}); err != nil {
			return err
		}
	}
	return nil
}

// PreConsumeUserSubscription pre-consumes from any active subscription total quota.
func PreConsumeUserSubscription(requestId string, userId int, modelName string, quotaType int, amount int64, usingGroup string) (*SubscriptionPreConsumeResult, error) {
	return PreConsumeUserSubscriptionForToken(requestId, userId, modelName, quotaType, amount, usingGroup, nil)
}

// PreConsumeUserSubscriptionForToken respects a concrete API Key binding. Its
// candidate order is the configured instance, explicit renewal successors,
// then other same-group instances. Wallet fallback remains a separate funding
// source and is never mixed into this transaction.
func PreConsumeUserSubscriptionForToken(requestId string, userId int, modelName string, quotaType int, amount int64, usingGroup string, binding *Token) (*SubscriptionPreConsumeResult, error) {
	if userId <= 0 {
		return nil, errors.New("invalid userId")
	}
	if strings.TrimSpace(requestId) == "" {
		return nil, errors.New("requestId is empty")
	}
	if amount <= 0 {
		return nil, errors.New("amount must be > 0")
	}
	now := GetDBTimestamp()

	returnValue := &SubscriptionPreConsumeResult{}

	tokenKeyToInvalidate := ""
	err := DB.Transaction(func(tx *gorm.DB) error {
		var existing SubscriptionPreConsumeRecord
		query := tx.Where("request_id = ?", requestId).Limit(1).Find(&existing)
		if query.Error != nil {
			return query.Error
		}
		if query.RowsAffected > 0 {
			if existing.Status == SubscriptionCostStatusRefunded {
				return errors.New("subscription pre-consume already refunded")
			}
			var sub UserSubscription
			if err := tx.Where("id = ?", existing.UserSubscriptionId).First(&sub).Error; err != nil {
				return err
			}
			returnValue.UserSubscriptionId = sub.Id
			returnValue.PreConsumed = existing.PreConsumed
			returnValue.AmountTotal = sub.AmountTotal
			returnValue.AmountUsedBefore = sub.AmountUsed
			returnValue.AmountUsedAfter = sub.AmountUsed
			return nil
		}

		subs, err := subscriptionCandidatesTx(tx, userId, usingGroup, now, binding)
		if err != nil {
			return errors.New("no active subscription")
		}
		if len(subs) == 0 {
			return errors.New("no active subscription")
		}
		for _, candidate := range subs {
			sub := candidate
			if sub.Status != "active" || sub.StartTime > now || sub.EndTime <= now {
				continue
			}
			if allowed := strings.TrimSpace(sub.AllowedGroup); allowed != "" && allowed != strings.TrimSpace(usingGroup) {
				continue
			}
			plan, err := planForUserSubscriptionTx(tx, &sub)
			if err != nil {
				return err
			}
			if err := maybeResetUserSubscriptionWithPlanTx(tx, &sub, plan, now); err != nil {
				return err
			}
			usedBefore := sub.AmountUsed
			if sub.AmountTotal > 0 {
				remain := sub.AmountTotal - usedBefore
				if remain < amount {
					continue
				}
			}
			// 月限额(套餐总额度上限, 不随 reset 重置)
			if sub.AmountCap > 0 {
				capRemain := sub.AmountCap - sub.AmountCapUsed
				if capRemain < amount {
					continue
				}
			}
			record := &SubscriptionPreConsumeRecord{
				RequestId:          requestId,
				UserId:             userId,
				UserSubscriptionId: sub.Id,
				PreConsumed:        amount,
				Status:             SubscriptionCostStatusReserved,
			}
			if err := tx.Create(record).Error; err != nil {
				var dup SubscriptionPreConsumeRecord
				if err2 := tx.Where("request_id = ?", requestId).First(&dup).Error; err2 == nil {
					if dup.Status == SubscriptionCostStatusRefunded {
						return errors.New("subscription pre-consume already refunded")
					}
					returnValue.UserSubscriptionId = sub.Id
					returnValue.PreConsumed = dup.PreConsumed
					returnValue.AmountTotal = sub.AmountTotal
					returnValue.AmountUsedBefore = sub.AmountUsed
					returnValue.AmountUsedAfter = sub.AmountUsed
					return nil
				}
				return err
			}
			sub.AmountUsed += amount
			sub.AmountCapUsed += amount
			// 月限额用完 → 套餐到期
			if sub.AmountCap > 0 && sub.AmountCapUsed >= sub.AmountCap {
				sub.Status = "expired"
				sub.EndTime = now
				sub.DividendReadyAt = now
			}
			if err := tx.Save(&sub).Error; err != nil {
				return err
			}
			if binding != nil &&
				NormalizeTokenSubscriptionMode(binding.SubscriptionMode) == TokenSubscriptionModeInstance &&
				binding.Id > 0 &&
				sub.Id != binding.SubscriptionId {
				var lockedToken Token
				if err := tx.Set("gorm:query_option", "FOR UPDATE").
					Where("id = ? AND user_id = ?", binding.Id, userId).
					First(&lockedToken).Error; err != nil {
					return err
				}
				before := lockedToken
				lockedToken.SubscriptionId = sub.Id
				lockedToken.SubscriptionWalletCycleId = sub.Id
				lockedToken.SubscriptionWalletUsed = 0
				if err := tx.Model(&Token{}).Where("id = ?", lockedToken.Id).
					Updates(map[string]any{
						"subscription_id":              sub.Id,
						"subscription_wallet_cycle_id": sub.Id,
						"subscription_wallet_used":     0,
					}).Error; err != nil {
					return err
				}
				action := TokenSubscriptionActionAutoSameGroup
				if sub.RenewedFromId != nil {
					action = TokenSubscriptionActionAutoRenew
				}
				history := bindingHistoryFromTokens(
					&before,
					&lockedToken,
					"system",
					action,
					"configured subscription unavailable",
				)
				if err := tx.Create(history).Error; err != nil {
					return err
				}
				tokenKeyToInvalidate = lockedToken.Key
			}
			returnValue.UserSubscriptionId = sub.Id
			returnValue.PreConsumed = amount
			returnValue.AmountTotal = sub.AmountTotal
			returnValue.AmountUsedBefore = usedBefore
			returnValue.AmountUsedAfter = sub.AmountUsed
			return nil
		}
		return fmt.Errorf("subscription quota insufficient, need=%d", amount)
	})
	if err != nil {
		return nil, err
	}
	if common.RedisEnabled && tokenKeyToInvalidate != "" {
		_ = cacheDeleteToken(tokenKeyToInvalidate)
	}
	return returnValue, nil
}

func channelCostNumerator(saleQuota, ratioPPM int64) (int64, error) {
	if saleQuota < 0 {
		return 0, errors.New("sale quota cannot be negative")
	}
	if ratioPPM < 0 || ratioPPM > MaxChannelCostRatioPPM {
		return 0, errors.New("channel cost ratio is out of range")
	}
	if saleQuota != 0 && ratioPPM > (1<<63-1)/saleQuota {
		return 0, errors.New("channel cost accumulator overflow")
	}
	return saleQuota * ratioPPM, nil
}

// SettleSubscriptionPreConsume applies the final usage delta and channel-cost
// snapshot in one transaction. This is the hot-path entry used by synchronous
// subscription billing, avoiding two separate row-lock transactions.
func SettleSubscriptionPreConsume(requestId string, usageDelta, finalSaleQuota int64, channelId int, ratioPPM *int64, provisional bool) error {
	if strings.TrimSpace(requestId) == "" {
		return errors.New("requestId is empty")
	}
	// Zero final sale has zero accounting cost regardless of the ratio. This
	// matters for full refunds: they must not stay pending only because a
	// channel was missing configuration.
	missingRatio := ratioPPM == nil && finalSaleQuota > 0
	costNumerator := int64(0)
	if ratioPPM != nil {
		var err error
		costNumerator, err = channelCostNumerator(finalSaleQuota, *ratioPPM)
		if err != nil {
			return err
		}
	}
	err := DB.Transaction(func(tx *gorm.DB) error {
		var record SubscriptionPreConsumeRecord
		if err := tx.Set("gorm:query_option", "FOR UPDATE").
			Where("request_id = ?", requestId).First(&record).Error; err != nil {
			return err
		}
		if record.Status == SubscriptionCostStatusRefunded {
			return nil
		}
		var sub UserSubscription
		needsSubscription := usageDelta != 0 || !missingRatio
		if needsSubscription {
			if err := tx.Set("gorm:query_option", "FOR UPDATE").
				Where("id = ?", record.UserSubscriptionId).First(&sub).Error; err != nil {
				return err
			}
		}
		if usageDelta != 0 {
			if err := applySubscriptionUsageDeltaTx(tx, &sub, usageDelta); err != nil {
				return err
			}
		}
		// Keep enough evidence to repair a missing channel ratio later. Returning
		// ErrChannelCostRatioMissing from inside this transaction would roll this
		// snapshot back and make the real final sale/channel unrecoverable.
		if missingRatio {
			record.FinalSaleQuota = finalSaleQuota
			record.ChannelId = channelId
			record.Status = SubscriptionCostStatusReserved
			return tx.Save(&record).Error
		}
		delta := costNumerator - record.CostNumerator
		if delta > 0 && sub.CostAccumulator > (1<<63-1)-delta {
			return errors.New("subscription cost accumulator overflow")
		}
		if delta < 0 && sub.CostAccumulator < -delta {
			return errors.New("subscription cost accumulator underflow")
		}
		sub.CostAccumulator += delta
		record.FinalSaleQuota = finalSaleQuota
		record.ChannelId = channelId
		if ratioPPM != nil {
			ratioCopy := *ratioPPM
			record.ChannelCostRatioPPM = &ratioCopy
		} else {
			record.ChannelCostRatioPPM = nil
		}
		record.CostNumerator = costNumerator
		record.Status = SubscriptionCostStatusFinal
		if provisional {
			record.Status = SubscriptionCostStatusProvisional
		}
		if err := tx.Save(&sub).Error; err != nil {
			return err
		}
		return tx.Save(&record).Error
	})
	if err != nil {
		return err
	}
	if missingRatio {
		return ErrChannelCostRatioMissing
	}
	return nil
}

// FinalizeSubscriptionPreConsume updates only the final channel-cost snapshot.
// Async task reconciliation uses this after it has adjusted usage separately.
func FinalizeSubscriptionPreConsume(requestId string, finalSaleQuota int64, channelId int, ratioPPM *int64, provisional bool) error {
	return SettleSubscriptionPreConsume(requestId, 0, finalSaleQuota, channelId, ratioPPM, provisional)
}

// RefundSubscriptionPreConsume is idempotent and refunds pre-consumed subscription quota by requestId.
func RefundSubscriptionPreConsume(requestId string) error {
	if strings.TrimSpace(requestId) == "" {
		return errors.New("requestId is empty")
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		var record SubscriptionPreConsumeRecord
		if err := tx.Set("gorm:query_option", "FOR UPDATE").
			Where("request_id = ?", requestId).First(&record).Error; err != nil {
			return err
		}
		if record.Status == SubscriptionCostStatusRefunded {
			return nil
		}
		var sub UserSubscription
		if err := tx.Set("gorm:query_option", "FOR UPDATE").
			Where("id = ?", record.UserSubscriptionId).First(&sub).Error; err != nil {
			return err
		}
		if err := applySubscriptionUsageDeltaTx(tx, &sub, -record.PreConsumed); err != nil {
			return err
		}
		if record.CostNumerator > sub.CostAccumulator {
			return errors.New("subscription cost accumulator underflow")
		}
		sub.CostAccumulator -= record.CostNumerator
		if err := tx.Save(&sub).Error; err != nil {
			return err
		}
		record.FinalSaleQuota = 0
		record.CostNumerator = 0
		record.Status = SubscriptionCostStatusRefunded
		return tx.Save(&record).Error
	})
}

// ResetDueSubscriptions resets subscriptions whose next_reset_time has passed.
func ResetDueSubscriptions(limit int) (int, error) {
	if limit <= 0 {
		limit = 200
	}
	now := GetDBTimestamp()
	var subs []UserSubscription
	if err := DB.Where("next_reset_time > 0 AND next_reset_time <= ? AND status = ?", now, "active").
		Order("next_reset_time asc").
		Limit(limit).
		Find(&subs).Error; err != nil {
		return 0, err
	}
	if len(subs) == 0 {
		return 0, nil
	}
	resetCount := 0
	for _, sub := range subs {
		subCopy := sub
		plan, err := planForUserSubscriptionTx(nil, &subCopy)
		if err != nil || plan == nil {
			continue
		}
		err = DB.Transaction(func(tx *gorm.DB) error {
			var locked UserSubscription
			if err := tx.Set("gorm:query_option", "FOR UPDATE").
				Where("id = ? AND next_reset_time > 0 AND next_reset_time <= ?", subCopy.Id, now).
				First(&locked).Error; err != nil {
				return nil
			}
			if err := maybeResetUserSubscriptionWithPlanTx(tx, &locked, plan, now); err != nil {
				return err
			}
			resetCount++
			return nil
		})
		if err != nil {
			return resetCount, err
		}
	}
	return resetCount, nil
}

// CleanupSubscriptionPreConsumeRecords removes old idempotency records to keep table small.
func CleanupSubscriptionPreConsumeRecords(olderThanSeconds int64) (int64, error) {
	if olderThanSeconds <= 0 {
		olderThanSeconds = 7 * 24 * 3600
	}
	cutoff := GetDBTimestamp() - olderThanSeconds
	res := DB.Where("updated_at < ? AND status IN ?", cutoff, []string{
		SubscriptionCostStatusFinal,
		SubscriptionCostStatusRefunded,
	}).Delete(&SubscriptionPreConsumeRecord{})
	return res.RowsAffected, res.Error
}

type SubscriptionPlanInfo struct {
	PlanId    int
	PlanTitle string
}

func GetSubscriptionPlanInfoByUserSubscriptionId(userSubscriptionId int) (*SubscriptionPlanInfo, error) {
	if userSubscriptionId <= 0 {
		return nil, errors.New("invalid userSubscriptionId")
	}
	cacheKey := fmt.Sprintf("sub:%d", userSubscriptionId)
	if cached, found, err := getSubscriptionPlanInfoCache().Get(cacheKey); err == nil && found {
		return &cached, nil
	}
	var sub UserSubscription
	if err := DB.Where("id = ?", userSubscriptionId).First(&sub).Error; err != nil {
		return nil, err
	}
	plan, err := getSubscriptionPlanByIdTx(nil, sub.PlanId)
	if err != nil {
		return nil, err
	}
	info := &SubscriptionPlanInfo{
		PlanId:    sub.PlanId,
		PlanTitle: plan.Title,
	}
	_ = getSubscriptionPlanInfoCache().SetWithTTL(cacheKey, *info, subscriptionPlanInfoCacheTTL())
	return info, nil
}

// Update subscription used amount by delta (positive consume more, negative refund).
func PostConsumeUserSubscriptionDelta(userSubscriptionId int, delta int64) error {
	if userSubscriptionId <= 0 {
		return errors.New("invalid userSubscriptionId")
	}
	if delta == 0 {
		return nil
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		var sub UserSubscription
		if err := tx.Set("gorm:query_option", "FOR UPDATE").
			Where("id = ?", userSubscriptionId).
			First(&sub).Error; err != nil {
			return err
		}
		return applySubscriptionUsageDeltaTx(tx, &sub, delta)
	})
}

func applySubscriptionUsageDeltaTx(tx *gorm.DB, sub *UserSubscription, delta int64) error {
	if tx == nil || sub == nil {
		return errors.New("invalid subscription delta args")
	}
	newUsed := sub.AmountUsed + delta
	if newUsed < 0 {
		newUsed = 0
	}
	if sub.AmountTotal > 0 && newUsed > sub.AmountTotal {
		return fmt.Errorf("subscription used exceeds total, used=%d total=%d", newUsed, sub.AmountTotal)
	}
	newCapUsed := sub.AmountCapUsed
	if sub.AmountCap > 0 {
		newCapUsed += delta
		if newCapUsed < 0 {
			newCapUsed = 0
		}
		if newCapUsed > sub.AmountCap {
			return fmt.Errorf("subscription cap used exceeds cap, used=%d cap=%d", newCapUsed, sub.AmountCap)
		}
	}
	sub.AmountUsed = newUsed
	sub.AmountCapUsed = newCapUsed
	if sub.AmountCap > 0 {
		if sub.AmountCapUsed >= sub.AmountCap {
			sub.Status = "expired"
			if sub.DividendReadyAt == 0 {
				sub.DividendReadyAt = GetDBTimestamp()
			}
		} else if sub.Status == "expired" && sub.EndTime > GetDBTimestamp() {
			sub.Status = "active"
			sub.DividendReadyAt = 0
		}
	}
	return tx.Save(sub).Error
}
