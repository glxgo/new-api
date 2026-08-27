package model

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	// VirtualMembershipDefaultAllowedGroup is the dedicated model group used
	// by newly created virtual-membership plans. It is also treated as a
	// membership-only billing boundary by the billing session.
	VirtualMembershipDefaultAllowedGroup = "gpt会员分组"

	VirtualMembershipStatusActive    = "active"
	VirtualMembershipStatusExpired   = "expired"
	VirtualMembershipStatusCancelled = "cancelled"
	VirtualMembershipOrderPending    = "pending"
	VirtualMembershipOrderSuccess    = "success"
	VirtualMembershipOrderClosed     = "closed"
	VirtualMembershipRecordPending   = "pending"
	VirtualMembershipRecordSettled   = "settled"
	VirtualMembershipRecordRefunded  = "refunded"
	VirtualMembershipAdminGrant      = "admin_grant"
	// VirtualMembershipResetOrderType identifies an Epay order that buys one
	// user-triggered weekly reset credit.  It deliberately uses a separate
	// ledger table so a paid add-on can never accidentally create a second
	// membership instance when the provider callback is replayed.
	VirtualMembershipResetOrderType = "active_reset_credit"
)

// virtualMembershipPendingSettlementTimeout leaves enough time for the
// configured asynchronous task timeout plus a small callback/drain buffer.
// This avoids reclaiming a legitimately long-running task while still making
// multi-day orphaned records recoverable from the user's reset action.
func virtualMembershipPendingSettlementTimeout() time.Duration {
	minutes := constant.TaskTimeoutMinutes
	if minutes <= 0 {
		minutes = 24 * 60
	}
	return time.Duration(minutes+5) * time.Minute
}

var (
	ErrVirtualMembershipRenewalUnavailable       = errors.New("当前虚拟会员无法续费")
	ErrVirtualMembershipRenewalExists            = errors.New("当前虚拟会员已存在续费实例或待支付订单")
	ErrVirtualMembershipResetCreditsInsufficient = errors.New("主动重置次数不足，请先购买重置次数")
	ErrVirtualMembershipSettlementInProgress     = errors.New("该会员存在正在结算的请求，请稍后再主动重置")
)

type VirtualMembershipSetting struct {
	Id           int    `json:"id"`
	Announcement string `json:"announcement" gorm:"type:text"`
	Enabled      bool   `json:"enabled" gorm:"not null;default:true"`
	UpdatedAt    int64  `json:"updated_at" gorm:"bigint"`
}

// VirtualMembershipPlan stores operator-defined entitlements. Group variants
// only change the price and divide the quotas; no waiting-room/group state is
// persisted because the product is a pricing strategy, not a real group.
type VirtualMembershipPlan struct {
	Id                      int     `json:"id"`
	Code                    string  `json:"code" gorm:"uniqueIndex;type:varchar(64)"`
	Title                   string  `json:"title" gorm:"type:varchar(128)"`
	Subtitle                string  `json:"subtitle" gorm:"type:varchar(255)"`
	Description             string  `json:"description" gorm:"type:text"`
	OriginalPriceAmount     float64 `json:"original_price_amount"`
	PriceAmount             float64 `json:"price_amount"`
	TwoGroupOriginalPrice   float64 `json:"two_group_original_price"`
	TwoGroupPrice           float64 `json:"two_group_price"`
	ThreeGroupOriginalPrice float64 `json:"three_group_original_price"`
	ThreeGroupPrice         float64 `json:"three_group_price"`
	FourGroupOriginalPrice  float64 `json:"four_group_original_price"`
	FourGroupPrice          float64 `json:"four_group_price"`
	Currency                string  `json:"currency" gorm:"type:varchar(8);not null;default:'USD'"`
	DurationDays            int     `json:"duration_days" gorm:"not null;default:30"`
	WeeklyQuota             int64   `json:"weekly_quota" gorm:"type:bigint;not null;default:0"`
	FiveHourEnabled         bool    `json:"five_hour_enabled" gorm:"not null;default:false"`
	FiveHourQuota           int64   `json:"five_hour_quota" gorm:"type:bigint;not null;default:0"`
	ConcurrencyLimit        int     `json:"concurrency_limit" gorm:"not null;default:0"`
	RPMLimit                int     `json:"rpm_limit" gorm:"not null;default:0"`
	AllowedModels           string  `json:"allowed_models" gorm:"type:text"`
	AllowedGroup            string  `json:"allowed_group" gorm:"type:varchar(64);default:''"`
	Recommended             bool    `json:"recommended" gorm:"not null;default:false"`
	Enabled                 bool    `json:"enabled" gorm:"not null;default:true;index"`
	SortOrder               int     `json:"sort_order" gorm:"not null;default:0"`
	CreatedAt               int64   `json:"created_at" gorm:"bigint"`
	UpdatedAt               int64   `json:"updated_at" gorm:"bigint"`
}

type VirtualMembershipOrder struct {
	Id        int     `json:"id"`
	UserId    int     `json:"user_id" gorm:"index"`
	PlanId    int     `json:"plan_id" gorm:"index"`
	GroupSize int     `json:"group_size" gorm:"not null;default:1"`
	Money     float64 `json:"money"`
	// PaymentFee is a gateway-only surcharge and is excluded from membership
	// entitlements and balance-like settlement values.
	PaymentFee                 float64 `json:"payment_fee" gorm:"not null;default:0"`
	WeeklyQuota                int64   `json:"weekly_quota" gorm:"type:bigint;not null;default:0"`
	FiveHourQuota              int64   `json:"five_hour_quota" gorm:"type:bigint;not null;default:0"`
	FiveHourActive             bool    `json:"five_hour_active" gorm:"not null;default:false"`
	ConcurrencyLimit           int     `json:"concurrency_limit" gorm:"not null;default:0"`
	RPMLimit                   int     `json:"rpm_limit" gorm:"not null;default:0"`
	DividendState              string  `json:"dividend_state" gorm:"type:varchar(32);not null;default:'pending';index"`
	TradeNo                    string  `json:"trade_no" gorm:"uniqueIndex;type:varchar(255)"`
	PaymentMethod              string  `json:"payment_method" gorm:"type:varchar(50)"`
	PaymentProvider            string  `json:"payment_provider" gorm:"type:varchar(50);default:''"`
	Status                     string  `json:"status" gorm:"type:varchar(32);index"`
	CreateTime                 int64   `json:"create_time" gorm:"bigint"`
	CompleteTime               int64   `json:"complete_time" gorm:"bigint"`
	ProviderPayload            string  `json:"-" gorm:"type:text"`
	PlanSnapshot               string  `json:"-" gorm:"type:text"`
	ProductNameSnapshot        string  `json:"product_name_snapshot" gorm:"type:varchar(128);not null;default:''"`
	LuckyRuleSetId             int64   `json:"lucky_rule_set_id" gorm:"index;not null;default:0"`
	LuckyEligible              bool    `json:"lucky_eligible" gorm:"not null;default:false"`
	ExpectedPaymentAmountMinor int64   `json:"expected_payment_amount_minor" gorm:"not null;default:0"`
	ExpectedPaymentCurrency    string  `json:"expected_payment_currency" gorm:"type:varchar(8);not null;default:''"`
	CommissionBaseQuota        int64   `json:"commission_base_quota" gorm:"not null;default:0"`
	ActualPaymentAmountMinor   int64   `json:"actual_payment_amount_minor" gorm:"not null;default:0"`
	ActualPaymentCurrency      string  `json:"actual_payment_currency" gorm:"type:varchar(8);not null;default:''"`
	RenewFromMembershipId      *int    `json:"renew_from_membership_id" gorm:"default:null;index"`
}

// VirtualMembershipResetOrder is an independent, idempotent payment ledger
// for the optional user-triggered reset add-on.  Keeping it separate from
// VirtualMembershipOrder prevents callback retries from creating memberships
// or entering the normal recharge/commission pipeline.
type VirtualMembershipResetOrder struct {
	Id                         int     `json:"id"`
	UserId                     int     `json:"user_id" gorm:"index"`
	MembershipId               int     `json:"membership_id" gorm:"index"`
	Credits                    int     `json:"credits" gorm:"not null;default:1"`
	Money                      float64 `json:"money"`
	PaymentFee                 float64 `json:"payment_fee" gorm:"not null;default:0"`
	TradeNo                    string  `json:"trade_no" gorm:"uniqueIndex;type:varchar(255)"`
	PaymentMethod              string  `json:"payment_method" gorm:"type:varchar(50)"`
	PaymentProvider            string  `json:"payment_provider" gorm:"type:varchar(50);default:''"`
	Status                     string  `json:"status" gorm:"type:varchar(32);index"`
	CreateTime                 int64   `json:"create_time" gorm:"bigint"`
	CompleteTime               int64   `json:"complete_time" gorm:"bigint"`
	ProviderPayload            string  `json:"-" gorm:"type:text"`
	ProductNameSnapshot        string  `json:"product_name_snapshot" gorm:"type:varchar(255)"`
	ExpectedPaymentAmountMinor int64   `json:"expected_payment_amount_minor" gorm:"not null;default:0"`
	ExpectedPaymentCurrency    string  `json:"expected_payment_currency" gorm:"type:varchar(8);not null;default:''"`
	ActualPaymentAmountMinor   int64   `json:"actual_payment_amount_minor" gorm:"not null;default:0"`
	ActualPaymentCurrency      string  `json:"actual_payment_currency" gorm:"type:varchar(8);not null;default:''"`
}

func setVirtualMembershipPaymentExpectation(order *VirtualMembershipOrder, snapshot PaymentSnapshot) error {
	baseSnapshot, err := PaymentSnapshotAfterFee(snapshot, order.PaymentFee)
	if err != nil {
		return err
	}
	baseQuota, err := CommissionBaseQuotaForPayment(baseSnapshot)
	if err != nil {
		return err
	}
	order.ExpectedPaymentAmountMinor = snapshot.AmountMinor
	order.ExpectedPaymentCurrency = snapshot.Currency
	order.CommissionBaseQuota = baseQuota
	return nil
}

// SetVirtualMembershipOrderPaymentExpectation is the public counterpart used
// by Epay controllers when a gateway surcharge changes the final amount.
func SetVirtualMembershipOrderPaymentExpectation(order *VirtualMembershipOrder, snapshot PaymentSnapshot) error {
	if order == nil {
		return errors.New("virtual membership order is nil")
	}
	return setVirtualMembershipPaymentExpectation(order, snapshot)
}

func UpdateVirtualMembershipOrderPaymentExpectation(orderId int, snapshot PaymentSnapshot, fee float64) error {
	if orderId <= 0 {
		return errors.New("invalid virtual membership order")
	}
	tmp := VirtualMembershipOrder{PaymentFee: fee}
	if err := setVirtualMembershipPaymentExpectation(&tmp, snapshot); err != nil {
		return err
	}
	baseQuota := tmp.CommissionBaseQuota
	if baseQuota <= 0 {
		return errors.New("invalid virtual membership commission base")
	}
	result := DB.Model(&VirtualMembershipOrder{}).Where("id = ? AND status = ?", orderId, common.TopUpStatusPending).
		Updates(map[string]interface{}{
			"payment_fee": fee, "expected_payment_amount_minor": snapshot.AmountMinor,
			"expected_payment_currency": snapshot.Currency, "commission_base_quota": baseQuota,
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return errors.New("virtual membership order is no longer pending")
	}
	return nil
}

func UpdateVirtualMembershipResetOrderPaymentExpectation(orderId int, snapshot PaymentSnapshot, fee float64) error {
	if orderId <= 0 {
		return errors.New("invalid virtual membership reset order")
	}
	if _, err := NewPaymentSnapshotFromMinor(snapshot.AmountMinor, snapshot.Currency); err != nil {
		return err
	}
	result := DB.Model(&VirtualMembershipResetOrder{}).Where("id = ? AND status = ?", orderId, common.TopUpStatusPending).
		Updates(map[string]interface{}{
			"payment_fee": fee, "expected_payment_amount_minor": snapshot.AmountMinor,
			"expected_payment_currency": snapshot.Currency,
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return errors.New("virtual membership reset order is no longer pending")
	}
	return nil
}

// VirtualMembershipActiveResetPrice rounds the purchased tier price to CNY
// cents and applies the policy price of 30%.  The purchase price is a
// snapshot; changing the current plan must not silently change an existing
// customer's add-on price.
func VirtualMembershipActiveResetPrice(purchasePrice float64) float64 {
	if purchasePrice <= 0 {
		return 0
	}
	return math.Round(purchasePrice*0.3*100) / 100
}

func virtualMembershipPurchasePriceTx(tx *gorm.DB, membership *UserVirtualMembership) (float64, error) {
	if tx == nil || membership == nil {
		return 0, errors.New("虚拟会员不存在")
	}
	if membership.PurchasePriceAmount > 0 {
		return membership.PurchasePriceAmount, nil
	}
	// Backfill legacy instances from their immutable order first.  The order's
	// Money already contains the selected group tier.
	if membership.OrderId > 0 {
		var order VirtualMembershipOrder
		if err := tx.Select("id", "money").First(&order, membership.OrderId).Error; err == nil && order.Money > 0 {
			return order.Money, nil
		} else if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return 0, err
		}
	}
	if membership.PlanId > 0 {
		plan, err := getVirtualMembershipPlanTx(tx, membership.PlanId)
		if err != nil {
			return 0, err
		}
		price, _, _, err := virtualMembershipVariant(plan, maxInt(membership.GroupSize, 1))
		if err != nil {
			return 0, err
		}
		return price, nil
	}
	return 0, errors.New("虚拟会员缺少购买价格快照")
}

func attachVirtualMembershipResetPrices(tx *gorm.DB, memberships []*UserVirtualMembership) error {
	for _, membership := range memberships {
		if membership == nil {
			continue
		}
		price, err := virtualMembershipPurchasePriceTx(tx, membership)
		if err != nil {
			// Historical/manual rows may not have an order or plan anymore. Keep
			// the membership usable and expose a zero price; the payment endpoint
			// will still reject an unsafe purchase rather than guessing.
			continue
		}
		membership.PurchasePriceAmount = price
		membership.ActiveResetPriceAmount = VirtualMembershipActiveResetPrice(price)
	}
	return nil
}

func maxInt(value, fallback int) int {
	if value < fallback {
		return fallback
	}
	return value
}

type UserVirtualMembership struct {
	Id     int `json:"id"`
	UserId int `json:"user_id" gorm:"index"`
	PlanId int `json:"plan_id" gorm:"index"`
	// Hidden is presentation-only. Hidden memberships remain active funding
	// sources and stay visible to administrators and accounting queries.
	Hidden        bool   `json:"hidden" gorm:"not null;default:false;index"`
	OrderId       int    `json:"order_id" gorm:"index"`
	OrderUniqueId *int   `json:"-" gorm:"column:order_unique_id;uniqueIndex"`
	RenewedFromId *int   `json:"renewed_from_id" gorm:"default:null;uniqueIndex"`
	PlanTitle     string `json:"plan_title" gorm:"type:varchar(128)"`
	PlanCode      string `json:"plan_code" gorm:"type:varchar(64)"`
	GroupSize     int    `json:"group_size" gorm:"not null;default:1"`
	// PurchasePriceAmount is the immutable price snapshot for this instance's
	// selected tier.  It is used to calculate the 30% active-reset add-on even
	// when an administrator later edits the plan price.
	PurchasePriceAmount float64 `json:"purchase_price_amount" gorm:"default:0"`
	// ActiveResetCredits are one-shot user-triggered weekly reset credits. New
	// purchases start at zero; administrators can grant credits explicitly.
	ActiveResetCredits     int     `json:"active_reset_credits" gorm:"not null;default:0"`
	WeeklyQuota            int64   `json:"weekly_quota" gorm:"type:bigint;not null;default:0"`
	WeeklyUsed             int64   `json:"weekly_used" gorm:"type:bigint;not null;default:0"`
	FiveHourQuota          int64   `json:"five_hour_quota" gorm:"type:bigint;not null;default:0"`
	FiveHourUsed           int64   `json:"five_hour_used" gorm:"type:bigint;not null;default:0"`
	FiveHourActive         bool    `json:"five_hour_active" gorm:"not null;default:false"`
	ConcurrencyLimit       int     `json:"concurrency_limit" gorm:"not null;default:0"`
	RPMLimit               int     `json:"rpm_limit" gorm:"not null;default:0"`
	WeeklyResetAt          int64   `json:"weekly_reset_at" gorm:"bigint;index"`
	FiveHourStart          int64   `json:"five_hour_start" gorm:"bigint"`
	FiveHourResetAt        int64   `json:"five_hour_reset_at" gorm:"bigint"`
	StartTime              int64   `json:"start_time" gorm:"bigint"`
	EndTime                int64   `json:"end_time" gorm:"bigint;index"`
	Status                 string  `json:"status" gorm:"type:varchar(32);index"`
	AllowedModels          string  `json:"allowed_models" gorm:"type:text"`
	AllowedGroup           string  `json:"allowed_group" gorm:"type:varchar(64);default:''"`
	CreatedAt              int64   `json:"created_at" gorm:"bigint"`
	UpdatedAt              int64   `json:"updated_at" gorm:"bigint"`
	LifetimeUsed           int64   `json:"lifetime_used" gorm:"-"`
	ActiveResetPriceAmount float64 `json:"active_reset_price_amount" gorm:"-"`
}

// AdminVirtualMembershipRecord combines a purchased entitlement with the
// minimum user identity required by the administrator membership list.
type AdminVirtualMembershipRecord struct {
	Membership  *UserVirtualMembership
	Username    string
	DisplayName string
	Email       string
	UserDeleted bool
}

type VirtualMembershipPreConsumeRecord struct {
	Id           int    `json:"id"`
	RequestId    string `json:"request_id" gorm:"uniqueIndex;type:varchar(128)"`
	MembershipId int    `json:"membership_id" gorm:"index"`
	UserId       int    `json:"user_id" gorm:"index"`
	PreConsumed  int64  `json:"pre_consumed" gorm:"type:bigint;not null;default:0"`
	FinalQuota   int64  `json:"final_quota" gorm:"type:bigint;not null;default:0"`
	Status       string `json:"status" gorm:"type:varchar(32);index"`
	CreatedAt    int64  `json:"created_at" gorm:"bigint"`
	UpdatedAt    int64  `json:"updated_at" gorm:"bigint"`
}

// attachVirtualMembershipLifetimeUsage derives the non-resetting usage total
// from the immutable settlement ledger. Pending holds and refunded requests
// are excluded, so the value represents quota actually consumed since this
// membership instance was purchased rather than its current reset window.
func attachVirtualMembershipLifetimeUsage(tx *gorm.DB, memberships []*UserVirtualMembership) error {
	if tx == nil || len(memberships) == 0 {
		return nil
	}
	ids := make([]int, 0, len(memberships))
	byId := make(map[int]*UserVirtualMembership, len(memberships))
	for _, membership := range memberships {
		if membership == nil || membership.Id <= 0 {
			continue
		}
		membership.LifetimeUsed = 0
		ids = append(ids, membership.Id)
		byId[membership.Id] = membership
	}
	if len(ids) == 0 {
		return nil
	}
	type lifetimeUsage struct {
		MembershipId int   `gorm:"column:membership_id"`
		LifetimeUsed int64 `gorm:"column:lifetime_used"`
	}
	var totals []lifetimeUsage
	if err := tx.Model(&VirtualMembershipPreConsumeRecord{}).
		Select("membership_id, COALESCE(SUM(final_quota), 0) AS lifetime_used").
		Where("membership_id IN ? AND status = ?", ids, VirtualMembershipRecordSettled).
		Group("membership_id").
		Scan(&totals).Error; err != nil {
		return err
	}
	for _, total := range totals {
		if membership := byId[total.MembershipId]; membership != nil {
			membership.LifetimeUsed = max(total.LifetimeUsed, 0)
		}
	}
	return nil
}

func (p *VirtualMembershipPlan) Normalize() {
	if p == nil {
		return
	}
	p.Code = strings.TrimSpace(strings.ToLower(p.Code))
	p.Title = strings.TrimSpace(p.Title)
	p.Currency = strings.TrimSpace(strings.ToUpper(p.Currency))
	if p.Currency == "" {
		p.Currency = "USD"
	}
	p.AllowedGroup = strings.TrimSpace(p.AllowedGroup)
	if p.AllowedGroup == "" {
		p.AllowedGroup = VirtualMembershipDefaultAllowedGroup
	}
	if p.DurationDays <= 0 {
		p.DurationDays = 30
	}
	if p.CreatedAt == 0 {
		p.CreatedAt = common.GetTimestamp()
	}
	p.UpdatedAt = common.GetTimestamp()
}

// HasVirtualMembershipPlanByGroup reports whether a group is reserved by a
// virtual-membership plan. A key using such a group must be explicitly bound
// to a membership; it must never fall back to wallet billing after the
// membership entitlement is exhausted.
func HasVirtualMembershipPlanByGroup(group string) (bool, error) {
	group = strings.TrimSpace(group)
	if group == "" {
		return false, nil
	}
	var count int64
	query := DB.Model(&VirtualMembershipPlan{}).Where("allowed_group = ?", group)
	if group == VirtualMembershipDefaultAllowedGroup {
		// Plans created before the dedicated default was introduced may still
		// have an empty snapshot in the database. Treat those legacy rows as the
		// default group until an administrator saves them again.
		query = query.Or("allowed_group = ''")
	}
	if err := query.Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

// GetActiveUserVirtualMembershipAllowedGroups returns the currently active
// membership groups for API-key creation. The time predicates make the
// result safe even before the periodic status maintenance has run.
func GetActiveUserVirtualMembershipAllowedGroups(userId int) ([]string, error) {
	if userId <= 0 {
		return nil, nil
	}
	now := common.GetTimestamp()
	var memberships []UserVirtualMembership
	err := DB.Where("user_id = ? AND status = ? AND start_time <= ? AND end_time > ?", userId, VirtualMembershipStatusActive, now, now).
		Find(&memberships).Error
	if err != nil {
		return nil, err
	}
	groupSet := make(map[string]struct{}, len(memberships))
	for _, membership := range memberships {
		group := strings.TrimSpace(membership.AllowedGroup)
		if group == "" {
			group = VirtualMembershipDefaultAllowedGroup
		}
		groupSet[group] = struct{}{}
	}
	groups := make([]string, 0, len(groupSet))
	for group := range groupSet {
		groups = append(groups, group)
	}
	sort.Strings(groups)
	return groups, nil
}

func HasActiveUserVirtualMembershipByGroup(userId int, group string) (bool, error) {
	if userId <= 0 || strings.TrimSpace(group) == "" {
		return false, nil
	}
	now := common.GetTimestamp()
	var count int64
	group = strings.TrimSpace(group)
	query := DB.Model(&UserVirtualMembership{}).
		Where("user_id = ? AND status = ? AND start_time <= ? AND end_time > ?",
			userId, VirtualMembershipStatusActive, now, now)
	if group == VirtualMembershipDefaultAllowedGroup {
		query = query.Where("(allowed_group = ? OR allowed_group = '')", group)
	} else {
		query = query.Where("allowed_group = ?", group)
	}
	err := query.Count(&count).Error
	return count > 0, err
}

func ValidateActiveVirtualMembershipEntitlementForToken(userId int, group string, membershipId int) error {
	if membershipId <= 0 {
		return errors.New("虚拟会员不存在")
	}
	now := common.GetTimestamp()
	var membership UserVirtualMembership
	if err := DB.Where("id = ? AND user_id = ?", membershipId, userId).First(&membership).Error; err != nil {
		return err
	}
	allowedGroup := strings.TrimSpace(membership.AllowedGroup)
	if allowedGroup == "" {
		allowedGroup = VirtualMembershipDefaultAllowedGroup
	}
	if allowedGroup != strings.TrimSpace(group) {
		return errors.New("虚拟会员不支持当前 API Key 分组")
	}
	if membership.Status != VirtualMembershipStatusActive || membership.StartTime > now || membership.EndTime <= now {
		return errors.New("虚拟会员尚未生效、已结束或已失效")
	}
	return nil
}

// VirtualMembershipCapacity is the purchased, per-membership capacity that
// applies to a key bound to the membership's dedicated group.
type VirtualMembershipCapacity struct {
	MembershipId     int
	ConcurrencyLimit int
	RPMLimit         int
	AllowedGroup     string
}

// GetActiveUserVirtualMembershipCapacity resolves the capacity snapshot for
// a bound API key. A missing/expired binding intentionally returns nil so the
// normal billing and key-validation paths can produce their own entitlement
// error instead of silently changing billing behavior here.
func GetActiveUserVirtualMembershipCapacity(userId, membershipId int, group string) (*VirtualMembershipCapacity, error) {
	if userId <= 0 || membershipId <= 0 {
		return nil, nil
	}
	var membership UserVirtualMembership
	now := common.GetTimestamp()
	err := DB.Where("id = ? AND user_id = ? AND status = ? AND start_time <= ? AND end_time > ?", membershipId, userId, VirtualMembershipStatusActive, now, now).First(&membership).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	allowedGroup := strings.TrimSpace(membership.AllowedGroup)
	if allowedGroup == "" {
		allowedGroup = VirtualMembershipDefaultAllowedGroup
	}
	if strings.TrimSpace(group) != allowedGroup {
		return nil, nil
	}
	return &VirtualMembershipCapacity{
		MembershipId: membership.Id, ConcurrencyLimit: membership.ConcurrencyLimit,
		RPMLimit: membership.RPMLimit, AllowedGroup: allowedGroup,
	}, nil
}

func (p *VirtualMembershipPlan) Validate() error {
	if p == nil || p.Code == "" || p.Title == "" {
		return errors.New("虚拟会员编码和名称不能为空")
	}
	if strings.ContainsAny(p.Code, " /\\") {
		return errors.New("虚拟会员编码不能包含空格或路径分隔符")
	}
	if p.OriginalPriceAmount < 0 || p.PriceAmount < 0 ||
		p.TwoGroupOriginalPrice < 0 || p.TwoGroupPrice < 0 ||
		p.ThreeGroupOriginalPrice < 0 || p.ThreeGroupPrice < 0 ||
		p.FourGroupOriginalPrice < 0 || p.FourGroupPrice < 0 {
		return errors.New("虚拟会员价格不能为负数")
	}
	if p.WeeklyQuota < 0 || p.FiveHourQuota < 0 {
		return errors.New("虚拟会员额度不能为负数")
	}
	if p.ConcurrencyLimit < 0 || p.RPMLimit < 0 {
		return errors.New("虚拟会员并发和 RPM 不能为负数")
	}
	if p.FiveHourEnabled && p.FiveHourQuota <= 0 {
		return errors.New("开启 5 小时限额后必须填写 5 小时额度")
	}
	return nil
}

func GetVirtualMembershipSetting() (*VirtualMembershipSetting, error) {
	var setting VirtualMembershipSetting
	err := DB.First(&setting, 1).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		setting = VirtualMembershipSetting{Id: 1, Enabled: true, UpdatedAt: common.GetTimestamp()}
		err = DB.Create(&setting).Error
	}
	return &setting, err
}

func SaveVirtualMembershipSetting(setting *VirtualMembershipSetting) error {
	if setting == nil {
		return errors.New("虚拟会员设置不能为空")
	}
	setting.Id = 1
	setting.UpdatedAt = common.GetTimestamp()
	return DB.Save(setting).Error
}

func EnsureDefaultVirtualMembershipSetting() error {
	if DB == nil {
		return nil
	}
	_, err := GetVirtualMembershipSetting()
	return err
}

func ListVirtualMembershipPlans(includeDisabled bool) ([]*VirtualMembershipPlan, error) {
	query := DB.Order("sort_order asc, id asc")
	if !includeDisabled {
		query = query.Where("enabled = ?", true)
	}
	var plans []*VirtualMembershipPlan
	if err := query.Find(&plans).Error; err != nil {
		return nil, err
	}
	for _, plan := range plans {
		if plan != nil && strings.TrimSpace(plan.AllowedGroup) == "" {
			plan.AllowedGroup = VirtualMembershipDefaultAllowedGroup
		}
	}
	return plans, nil
}

func GetVirtualMembershipPlanById(planId int) (*VirtualMembershipPlan, error) {
	if planId <= 0 {
		return nil, errors.New("虚拟会员方案不存在")
	}
	var plan VirtualMembershipPlan
	if err := DB.First(&plan, planId).Error; err != nil {
		return nil, err
	}
	plan.Normalize()
	if err := plan.Validate(); err != nil {
		return nil, err
	}
	return &plan, nil
}

func SaveVirtualMembershipPlan(plan *VirtualMembershipPlan) error {
	if plan == nil {
		return errors.New("虚拟会员方案不能为空")
	}
	plan.Normalize()
	if err := plan.Validate(); err != nil {
		return err
	}
	// Channel editors and administrator group settings are backed by
	// GroupRatio, while virtual memberships keep their entitlement group on
	// the plan snapshot. Register the routing group without adding it to
	// UserUsableGroups, so only users with an active membership can select it.
	if err := EnsureVirtualMembershipGroupRegistered(plan.AllowedGroup); err != nil {
		return err
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Save(plan).Error; err != nil {
			return err
		}
		if plan.Id <= 0 {
			return nil
		}
		return syncVirtualMembershipFiveHourEntitlementsTx(tx, plan)
	})
}

// EnsureVirtualMembershipGroupRegistered makes a membership entitlement group
// available to channel/group administration. It deliberately does not add the
// group to UserUsableGroups: membership holders receive it dynamically through
// GetUserGroups, while ordinary users must not be able to select it.
func EnsureVirtualMembershipGroupRegistered(group string) error {
	group = strings.TrimSpace(group)
	if group == "" {
		group = VirtualMembershipDefaultAllowedGroup
	}

	var option Option
	result := DB.Where(optionKeyWhereClause(), "GroupRatio").First(&option)
	ratios := make(map[string]float64)
	if result.Error == nil && strings.TrimSpace(option.Value) != "" {
		if err := json.Unmarshal([]byte(option.Value), &ratios); err != nil {
			return fmt.Errorf("解析分组倍率配置失败: %w", err)
		}
	} else if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		for name, ratio := range ratio_setting.GetGroupRatioCopy() {
			ratios[name] = ratio
		}
	} else if result.Error != nil {
		return result.Error
	}

	if _, exists := ratios[group]; exists {
		// Keep the runtime map synchronized when this helper is called outside
		// the normal option-loading startup sequence.
		if !ratio_setting.ContainsGroupRatio(group) && option.Value != "" {
			return updateOptionMap("GroupRatio", option.Value)
		}
		return nil
	}
	ratios[group] = 1
	value, err := json.Marshal(ratios)
	if err != nil {
		return err
	}
	// Use the checked bulk writer rather than UpdateOption: the latter keeps
	// legacy best-effort database writes and can report success after a failed
	// Save, which would make the group disappear again after a restart.
	return UpdateOptionsBulk(map[string]string{"GroupRatio": string(value)})
}

// syncVirtualMembershipFiveHourEntitlementsTx applies the current plan's
// 5-hour policy to memberships that are usable now. Weekly quota and the other
// purchased snapshots stay immutable; only the administrator-controlled
// 5-hour switch and limit are live plan settings.
func syncVirtualMembershipFiveHourEntitlementsTx(tx *gorm.DB, plan *VirtualMembershipPlan) error {
	if tx == nil || plan == nil || plan.Id <= 0 {
		return nil
	}
	now := common.GetTimestamp()
	var memberships []UserVirtualMembership
	if err := tx.Where(
		"plan_id = ? AND status = ? AND start_time <= ? AND end_time > ?",
		plan.Id, VirtualMembershipStatusActive, now, now,
	).Find(&memberships).Error; err != nil {
		return err
	}
	for i := range memberships {
		membership := &memberships[i]
		groupSize := membership.GroupSize
		if groupSize <= 0 {
			groupSize = 1
		}
		updates := map[string]interface{}{"updated_at": now}
		if !plan.FiveHourEnabled {
			updates["five_hour_active"] = false
			updates["five_hour_quota"] = int64(0)
			updates["five_hour_used"] = int64(0)
			updates["five_hour_start"] = int64(0)
			updates["five_hour_reset_at"] = int64(0)
		} else {
			updates["five_hour_active"] = true
			updates["five_hour_quota"] = plan.FiveHourQuota / int64(groupSize)
			if !membership.FiveHourActive || membership.FiveHourResetAt <= 0 {
				updates["five_hour_used"] = int64(0)
				updates["five_hour_start"] = now
				updates["five_hour_reset_at"] = now + 5*3600
			}
		}
		if err := tx.Model(&UserVirtualMembership{}).
			Where("id = ?", membership.Id).
			Updates(updates).Error; err != nil {
			return err
		}
	}
	return nil
}

func getVirtualMembershipPlanTx(tx *gorm.DB, planId int) (*VirtualMembershipPlan, error) {
	var plan VirtualMembershipPlan
	if err := tx.First(&plan, planId).Error; err != nil {
		return nil, err
	}
	plan.Normalize()
	if err := plan.Validate(); err != nil {
		return nil, err
	}
	return &plan, nil
}

func splitVirtualMembershipLimit(value, groupSize int) int {
	if value <= 0 {
		return 0
	}
	result := value / groupSize
	if result <= 0 {
		return 1
	}
	return result
}

func virtualMembershipVariantWithLimits(plan *VirtualMembershipPlan, groupSize int) (float64, int64, int64, int, int, error) {
	if groupSize != 1 && groupSize != 2 && groupSize != 3 && groupSize != 4 {
		return 0, 0, 0, 0, 0, errors.New("虚拟会员仅支持单独购买、2 人团、3 人团和 4 人团")
	}
	price := plan.PriceAmount
	if groupSize == 2 && plan.TwoGroupPrice > 0 {
		price = plan.TwoGroupPrice
	}
	if groupSize == 3 && plan.ThreeGroupPrice > 0 {
		price = plan.ThreeGroupPrice
	}
	if groupSize == 4 && plan.FourGroupPrice > 0 {
		price = plan.FourGroupPrice
	}
	if price <= 0 {
		return 0, 0, 0, 0, 0, errors.New("该虚拟会员方案暂未配置此档位价格")
	}
	return price, plan.WeeklyQuota / int64(groupSize), plan.FiveHourQuota / int64(groupSize),
		splitVirtualMembershipLimit(plan.ConcurrencyLimit, groupSize),
		splitVirtualMembershipLimit(plan.RPMLimit, groupSize), nil
}

func virtualMembershipVariant(plan *VirtualMembershipPlan, groupSize int) (float64, int64, int64, error) {
	price, weekly, fiveHour, _, _, err := virtualMembershipVariantWithLimits(plan, groupSize)
	return price, weekly, fiveHour, err
}

// VirtualMembershipVariantForDisplay exposes the same fallback and quota
// division rule used by checkout, so the UI never displays a zero-price group
// that the backend would silently charge at the base price.
func VirtualMembershipVariantForDisplay(plan *VirtualMembershipPlan, groupSize int) (float64, int64, int64, error) {
	return virtualMembershipVariant(plan, groupSize)
}

// VirtualMembershipVariantLimitsForDisplay returns the checkout variant with
// its purchased concurrency and RPM limits. A zero limit means unlimited,
// matching the existing user/channel capacity convention.
func VirtualMembershipVariantLimitsForDisplay(plan *VirtualMembershipPlan, groupSize int) (float64, int64, int64, int, int, error) {
	return virtualMembershipVariantWithLimits(plan, groupSize)
}

// VirtualMembershipOriginalPriceForDisplay resolves the operator-configured
// crossed-out price for a purchase tier. It is display-only and never changes
// the amount charged by checkout.
func VirtualMembershipOriginalPriceForDisplay(plan *VirtualMembershipPlan, groupSize int) (float64, error) {
	if plan == nil {
		return 0, errors.New("虚拟会员方案不存在")
	}
	switch groupSize {
	case 1:
		return plan.OriginalPriceAmount, nil
	case 2:
		return plan.TwoGroupOriginalPrice, nil
	case 3:
		return plan.ThreeGroupOriginalPrice, nil
	case 4:
		return plan.FourGroupOriginalPrice, nil
	default:
		return 0, errors.New("虚拟会员仅支持单独购买、2 人团、3 人团和 4 人团")
	}
}

type virtualMembershipSnapshot struct {
	PlanId           int     `json:"plan_id"`
	Code             string  `json:"code"`
	Title            string  `json:"title"`
	Price            float64 `json:"price"`
	GroupSize        int     `json:"group_size"`
	WeeklyQuota      int64   `json:"weekly_quota"`
	FiveHourQuota    int64   `json:"five_hour_quota"`
	FiveHourEnabled  bool    `json:"five_hour_enabled"`
	ConcurrencyLimit int     `json:"concurrency_limit"`
	RPMLimit         int     `json:"rpm_limit"`
	AllowedModels    string  `json:"allowed_models"`
	AllowedGroup     string  `json:"allowed_group"`
	DurationDays     int     `json:"duration_days"`
}

func buildVirtualMembershipSnapshot(plan *VirtualMembershipPlan, price float64, groupSize int, weekly, fiveHour int64, concurrency, rpm int) string {
	data, _ := common.Marshal(virtualMembershipSnapshot{
		PlanId: plan.Id, Code: plan.Code, Title: plan.Title, Price: price,
		GroupSize: groupSize, WeeklyQuota: weekly, FiveHourQuota: fiveHour,
		FiveHourEnabled: plan.FiveHourEnabled, AllowedModels: plan.AllowedModels,
		AllowedGroup: plan.AllowedGroup, DurationDays: plan.DurationDays,
		ConcurrencyLimit: concurrency, RPMLimit: rpm,
	})
	return string(data)
}

func createVirtualMembershipOrderTx(tx *gorm.DB, userId, planId, groupSize int, tradeNo, paymentMethod, paymentProvider, status string) (*VirtualMembershipOrder, error) {
	if tx == nil || userId <= 0 || strings.TrimSpace(tradeNo) == "" {
		return nil, errors.New("虚拟会员订单参数无效")
	}
	plan, err := getVirtualMembershipPlanTx(tx, planId)
	if err != nil {
		return nil, err
	}
	if !plan.Enabled {
		return nil, errors.New("虚拟会员方案未启用")
	}
	price, weekly, fiveHour, concurrency, rpm, err := virtualMembershipVariantWithLimits(plan, groupSize)
	if err != nil {
		return nil, err
	}
	if paymentProvider == PaymentProviderEpay {
		price = NormalizePaymentAmount(price)
	}
	if price < 0.01 {
		return nil, errors.New("虚拟会员金额过低")
	}
	now := common.GetTimestamp()
	order := &VirtualMembershipOrder{
		UserId: userId, PlanId: plan.Id, GroupSize: groupSize, Money: price,
		WeeklyQuota: weekly, FiveHourQuota: fiveHour, FiveHourActive: plan.FiveHourEnabled,
		ConcurrencyLimit: concurrency, RPMLimit: rpm,
		DividendState: SubscriptionDividendPending,
		TradeNo:       tradeNo, PaymentMethod: paymentMethod, PaymentProvider: paymentProvider,
		Status: status, CreateTime: now, PlanSnapshot: buildVirtualMembershipSnapshot(plan, price, groupSize, weekly, fiveHour, concurrency, rpm),
		ProductNameSnapshot: PaymentProductNameForVirtualMembership(plan.Title),
	}
	if campaign, rule, luckyErr := GetLuckyCampaignTx(tx, false); luckyErr == nil && !campaign.IssuancePaused {
		order.LuckyRuleSetId = rule.Id
		order.LuckyEligible = true
	}
	if paymentProvider == PaymentProviderEpay {
		snapshot, snapshotErr := NewPaymentSnapshotFromMoney(price, "CNY")
		if snapshotErr != nil {
			return nil, snapshotErr
		}
		if snapshotErr = setVirtualMembershipPaymentExpectation(order, snapshot); snapshotErr != nil {
			return nil, snapshotErr
		}
	}
	if err := tx.Create(order).Error; err != nil {
		return nil, err
	}
	return order, nil
}

func CreateVirtualMembershipEpayOrder(userId, planId, groupSize int, tradeNo, paymentMethod string) (*VirtualMembershipOrder, error) {
	return CreateVirtualMembershipEpayRenewalOrder(userId, planId, groupSize, tradeNo, paymentMethod, 0)
}

// resolveVirtualMembershipRenewalTx validates a linear renewal. User renewals
// must use the currently enabled same plan and group tier; administrator
// renewals may extend a disabled legacy plan but still preserve that identity.
func resolveVirtualMembershipRenewalTx(tx *gorm.DB, userId, fromMembershipId, planId, groupSize int, lock, requireEnabled bool) (*UserVirtualMembership, *VirtualMembershipPlan, error) {
	if tx == nil || userId <= 0 || fromMembershipId <= 0 {
		return nil, nil, ErrVirtualMembershipRenewalUnavailable
	}
	query := tx.Where("id = ? AND user_id = ?", fromMembershipId, userId)
	if lock {
		query = query.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	var source UserVirtualMembership
	if err := query.First(&source).Error; err != nil {
		return nil, nil, ErrVirtualMembershipRenewalUnavailable
	}
	if source.Status != VirtualMembershipStatusActive && source.Status != VirtualMembershipStatusExpired {
		return nil, nil, ErrVirtualMembershipRenewalUnavailable
	}
	if source.GroupSize <= 0 {
		source.GroupSize = 1
	}
	if planId > 0 && planId != source.PlanId {
		return nil, nil, ErrVirtualMembershipRenewalUnavailable
	}
	if groupSize > 0 && groupSize != source.GroupSize {
		return nil, nil, ErrVirtualMembershipRenewalUnavailable
	}
	var successorCount int64
	if err := tx.Model(&UserVirtualMembership{}).Where("renewed_from_id = ?", source.Id).Count(&successorCount).Error; err != nil {
		return nil, nil, err
	}
	if successorCount > 0 {
		return nil, nil, ErrVirtualMembershipRenewalExists
	}
	var pendingOrderCount int64
	if err := tx.Model(&VirtualMembershipOrder{}).
		Where("renew_from_membership_id = ? AND status = ?", source.Id, common.TopUpStatusPending).
		Count(&pendingOrderCount).Error; err != nil {
		return nil, nil, err
	}
	if pendingOrderCount > 0 {
		return nil, nil, ErrVirtualMembershipRenewalExists
	}
	plan, err := getVirtualMembershipPlanTx(tx, source.PlanId)
	if err != nil {
		return nil, nil, ErrVirtualMembershipRenewalUnavailable
	}
	if requireEnabled && !plan.Enabled {
		return nil, nil, ErrVirtualMembershipRenewalUnavailable
	}
	return &source, plan, nil
}

func CreateVirtualMembershipEpayRenewalOrder(userId, planId, groupSize int, tradeNo, paymentMethod string, renewFromMembershipId int) (*VirtualMembershipOrder, error) {
	var order *VirtualMembershipOrder
	err := DB.Transaction(func(tx *gorm.DB) error {
		if renewFromMembershipId > 0 {
			if _, _, err := resolveVirtualMembershipRenewalTx(tx, userId, renewFromMembershipId, planId, groupSize, true, true); err != nil {
				return err
			}
		}
		var err error
		order, err = createVirtualMembershipOrderTx(tx, userId, planId, groupSize, tradeNo, paymentMethod, PaymentProviderEpay, common.TopUpStatusPending)
		if err == nil && renewFromMembershipId > 0 {
			order.RenewFromMembershipId = &renewFromMembershipId
			err = tx.Model(order).Update("renew_from_membership_id", renewFromMembershipId).Error
		}
		return err
	})
	return order, err
}

// AdminGrantVirtualMembership creates a paid-equivalent membership without
// charging the user. A zero-money success order is retained as the audit trail.
func AdminGrantVirtualMembership(userId, planId, groupSize int) (*VirtualMembershipOrder, *UserVirtualMembership, error) {
	if userId <= 0 || planId <= 0 {
		return nil, nil, errors.New("用户或虚拟会员方案无效")
	}
	if groupSize == 0 {
		groupSize = 1
	}
	var order *VirtualMembershipOrder
	var membership UserVirtualMembership
	err := DB.Transaction(func(tx *gorm.DB) error {
		var user User
		if err := tx.Select("id", "status").First(&user, userId).Error; err != nil {
			return errors.New("用户不存在")
		}
		if user.Status != common.UserStatusEnabled {
			return errors.New("用户当前未启用")
		}

		tradeNo := fmt.Sprintf("vm-admin-%d-%s", time.Now().UnixNano(), common.GetRandomString(6))
		var err error
		order, err = createVirtualMembershipOrderTx(
			tx, userId, planId, groupSize, tradeNo,
			VirtualMembershipAdminGrant, VirtualMembershipAdminGrant,
			VirtualMembershipOrderSuccess,
		)
		if err != nil {
			return err
		}
		order.Money = 0
		order.CompleteTime = common.GetTimestamp()
		order.ProviderPayload = "granted_by_admin"
		order.DividendState = SubscriptionDividendSkippedSource
		order.LuckyRuleSetId = 0
		order.LuckyEligible = false
		if err := tx.Save(order).Error; err != nil {
			return err
		}
		if err := createVirtualMembershipFromOrderTx(tx, order); err != nil {
			return err
		}
		return tx.Where("order_id = ?", order.Id).First(&membership).Error
	})
	if err != nil {
		return nil, nil, err
	}
	return order, &membership, nil
}

// AdminRenewVirtualMembership creates one no-payment, linked successor. The
// successor starts at the source end time (or now when already expired), so
// administrators extend duration without overwriting usage or audit history.
func AdminRenewVirtualMembership(fromMembershipId int) (*VirtualMembershipOrder, *UserVirtualMembership, error) {
	if fromMembershipId <= 0 {
		return nil, nil, ErrVirtualMembershipRenewalUnavailable
	}
	var order *VirtualMembershipOrder
	var membership UserVirtualMembership
	err := DB.Transaction(func(tx *gorm.DB) error {
		var owner UserVirtualMembership
		if err := tx.Select("id", "user_id", "plan_id", "group_size").First(&owner, fromMembershipId).Error; err != nil {
			return ErrVirtualMembershipRenewalUnavailable
		}
		source, _, err := resolveVirtualMembershipRenewalTx(
			tx, owner.UserId, fromMembershipId, owner.PlanId, owner.GroupSize, true, false,
		)
		if err != nil {
			return err
		}
		tradeNo := fmt.Sprintf("vm-admin-renew-%d-%s", time.Now().UnixNano(), common.GetRandomString(6))
		order, err = createVirtualMembershipOrderTx(
			tx, source.UserId, source.PlanId, source.GroupSize, tradeNo,
			VirtualMembershipAdminGrant, VirtualMembershipAdminGrant,
			VirtualMembershipOrderSuccess,
		)
		if err != nil {
			// A disabled legacy plan is still renewable by an administrator.
			// Rebuild the order from the source snapshot if the plan was delisted.
			var plan VirtualMembershipPlan
			if lookupErr := tx.First(&plan, source.PlanId).Error; lookupErr != nil || plan.Enabled {
				return err
			}
			plan.Enabled = true
			price, weekly, fiveHour, concurrency, rpm, variantErr := virtualMembershipVariantWithLimits(&plan, source.GroupSize)
			if variantErr != nil {
				return variantErr
			}
			now := common.GetTimestamp()
			order = &VirtualMembershipOrder{
				UserId: source.UserId, PlanId: plan.Id, GroupSize: source.GroupSize,
				Money: 0, WeeklyQuota: weekly, FiveHourQuota: fiveHour,
				FiveHourActive: plan.FiveHourEnabled, ConcurrencyLimit: concurrency, RPMLimit: rpm,
				DividendState: SubscriptionDividendSkippedSource, TradeNo: tradeNo,
				PaymentMethod: VirtualMembershipAdminGrant, PaymentProvider: VirtualMembershipAdminGrant,
				Status: VirtualMembershipOrderSuccess, CreateTime: now, CompleteTime: now,
				PlanSnapshot:        buildVirtualMembershipSnapshot(&plan, price, source.GroupSize, weekly, fiveHour, concurrency, rpm),
				ProductNameSnapshot: PaymentProductNameForVirtualMembership(plan.Title),
			}
			if createErr := tx.Create(order).Error; createErr != nil {
				return createErr
			}
		}
		order.Money = 0
		order.CompleteTime = common.GetTimestamp()
		order.ProviderPayload = "renewed_by_admin"
		order.DividendState = SubscriptionDividendSkippedSource
		order.LuckyRuleSetId = 0
		order.LuckyEligible = false
		order.RenewFromMembershipId = &source.Id
		if err := tx.Save(order).Error; err != nil {
			return err
		}
		if err := createVirtualMembershipFromOrderTx(tx, order); err != nil {
			return err
		}
		return tx.Where("order_id = ?", order.Id).First(&membership).Error
	})
	if err != nil {
		return nil, nil, err
	}
	return order, &membership, nil
}

func GetVirtualMembershipOrderByTradeNo(tradeNo string) *VirtualMembershipOrder {
	if strings.TrimSpace(tradeNo) == "" {
		return nil
	}
	var order VirtualMembershipOrder
	if err := DB.Where("trade_no = ?", tradeNo).First(&order).Error; err != nil {
		return nil
	}
	return &order
}

func createVirtualMembershipFromOrderTx(tx *gorm.DB, order *VirtualMembershipOrder) error {
	if tx == nil || order == nil {
		return errors.New("虚拟会员订单无效")
	}
	var snapshot virtualMembershipSnapshot
	if strings.TrimSpace(order.PlanSnapshot) != "" {
		if err := common.UnmarshalJsonStr(order.PlanSnapshot, &snapshot); err != nil {
			return err
		}
	}
	if snapshot.Title == "" {
		plan, err := getVirtualMembershipPlanTx(tx, order.PlanId)
		if err != nil {
			return err
		}
		snapshot = virtualMembershipSnapshot{
			PlanId: plan.Id, Code: plan.Code, Title: plan.Title, GroupSize: order.GroupSize,
			WeeklyQuota: order.WeeklyQuota, FiveHourQuota: order.FiveHourQuota,
			ConcurrencyLimit: order.ConcurrencyLimit, RPMLimit: order.RPMLimit,
			FiveHourEnabled: order.FiveHourActive, AllowedModels: plan.AllowedModels,
			AllowedGroup: plan.AllowedGroup, DurationDays: plan.DurationDays,
		}
	}
	if snapshot.ConcurrencyLimit == 0 && order.ConcurrencyLimit > 0 {
		snapshot.ConcurrencyLimit = order.ConcurrencyLimit
	}
	if snapshot.RPMLimit == 0 && order.RPMLimit > 0 {
		snapshot.RPMLimit = order.RPMLimit
	}
	durationDays := snapshot.DurationDays
	if durationDays <= 0 {
		durationDays = 30
	}
	now := GetDBTimestamp()
	if now <= 0 {
		now = common.GetTimestamp()
	}
	startTime := now
	var renewedFromId *int
	if order.RenewFromMembershipId != nil && *order.RenewFromMembershipId > 0 {
		var source UserVirtualMembership
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND user_id = ?", *order.RenewFromMembershipId, order.UserId).
			First(&source).Error; err != nil {
			return ErrVirtualMembershipRenewalUnavailable
		}
		if (source.Status != VirtualMembershipStatusActive && source.Status != VirtualMembershipStatusExpired) ||
			source.PlanId != order.PlanId {
			return ErrVirtualMembershipRenewalUnavailable
		}
		sourceGroupSize := source.GroupSize
		if sourceGroupSize <= 0 {
			sourceGroupSize = 1
		}
		if sourceGroupSize != order.GroupSize {
			return ErrVirtualMembershipRenewalUnavailable
		}
		var successorCount int64
		if err := tx.Model(&UserVirtualMembership{}).Where("renewed_from_id = ?", source.Id).Count(&successorCount).Error; err != nil {
			return err
		}
		if successorCount > 0 {
			return ErrVirtualMembershipRenewalExists
		}
		if source.EndTime > startTime {
			startTime = source.EndTime
		}
		renewedFromId = &source.Id
	}
	membership := &UserVirtualMembership{
		UserId: order.UserId, PlanId: order.PlanId, OrderId: order.Id,
		OrderUniqueId: &order.Id,
		RenewedFromId: renewedFromId,
		PlanTitle:     snapshot.Title, PlanCode: snapshot.Code, GroupSize: order.GroupSize,
		PurchasePriceAmount: order.Money,
		WeeklyQuota:         order.WeeklyQuota, FiveHourQuota: order.FiveHourQuota,
		ConcurrencyLimit: snapshot.ConcurrencyLimit, RPMLimit: snapshot.RPMLimit,
		FiveHourActive: order.FiveHourActive, WeeklyResetAt: startTime + 7*86400,
		FiveHourStart: startTime, FiveHourResetAt: startTime + 5*3600,
		StartTime: startTime, EndTime: startTime + int64(durationDays)*86400,
		Status: VirtualMembershipStatusActive, AllowedModels: snapshot.AllowedModels,
		AllowedGroup: snapshot.AllowedGroup, CreatedAt: now, UpdatedAt: now,
	}
	result := tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "order_unique_id"}}, DoNothing: true}).Create(membership)
	return result.Error
}

func CompleteVirtualMembershipOrder(tradeNo, providerPayload, expectedPaymentProvider, actualPaymentMethod string, paymentSnapshots ...PaymentSnapshot) error {
	if strings.TrimSpace(tradeNo) == "" {
		return errors.New("tradeNo is empty")
	}
	var completedOrder VirtualMembershipOrder
	err := DB.Transaction(func(tx *gorm.DB) error {
		var order VirtualMembershipOrder
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("trade_no = ?", tradeNo).First(&order).Error; err != nil {
			return errors.New("虚拟会员订单不存在")
		}
		if expectedPaymentProvider != "" && order.PaymentProvider != expectedPaymentProvider {
			return ErrPaymentMethodMismatch
		}
		if len(paymentSnapshots) != 1 {
			return errors.New("verified payment snapshot is required")
		}
		actual, snapshotErr := NewPaymentSnapshotFromMinor(paymentSnapshots[0].AmountMinor, paymentSnapshots[0].Currency)
		if snapshotErr != nil {
			return snapshotErr
		}
		if err := ValidatePaymentSnapshot(order.ExpectedPaymentAmountMinor, order.ExpectedPaymentCurrency, actual); err != nil {
			return err
		}
		if order.ActualPaymentAmountMinor > 0 &&
			(order.ActualPaymentAmountMinor != actual.AmountMinor || !strings.EqualFold(order.ActualPaymentCurrency, actual.Currency)) {
			return ErrPaymentSnapshotMismatch
		}
		order.ActualPaymentAmountMinor = actual.AmountMinor
		order.ActualPaymentCurrency = actual.Currency
		if order.Status == common.TopUpStatusSuccess || order.Status == VirtualMembershipOrderSuccess {
			completedOrder = order
			return nil
		}
		if order.Status != common.TopUpStatusPending {
			return errors.New("虚拟会员订单状态无效")
		}
		if err := createVirtualMembershipFromOrderTx(tx, &order); err != nil {
			return err
		}
		order.Status = VirtualMembershipOrderSuccess
		order.CompleteTime = common.GetTimestamp()
		if providerPayload != "" {
			order.ProviderPayload = providerPayload
		}
		if actualPaymentMethod != "" {
			order.PaymentMethod = actualPaymentMethod
		}
		if err := tx.Save(&order).Error; err != nil {
			return err
		}
		actualNet, netErr := PaymentSnapshotAfterFee(actual, order.PaymentFee)
		if netErr != nil {
			return netErr
		}
		amountCents, err := RechargeCentsForPayment(actualNet)
		if err != nil {
			return err
		}
		if order.CommissionBaseQuota <= 0 {
			return errors.New("virtual membership commission base snapshot is missing")
		}
		if _, err := RecordPaidRechargeCreditTx(
			tx, order.UserId, amountCents, order.CommissionBaseQuota, actual.Currency,
			RechargeSourceVirtualMembership, order.TradeNo, order.CompleteTime,
		); err != nil {
			return err
		}
		order.DividendState = SubscriptionDividendDone
		if err := tx.Model(&order).Update("dividend_state", SubscriptionDividendDone).Error; err != nil {
			return err
		}
		completedOrder = order
		return nil
	})
	if err != nil {
		return err
	}
	if completedOrder.Id > 0 {
		InvalidateUserCache(completedOrder.UserId)
		InvalidateRechargeCommissionRecipientCaches(RechargeSourceVirtualMembership, completedOrder.TradeNo)
		if err := EnsureTopupLog(
			completedOrder.UserId,
			"virtual_membership:"+completedOrder.TradeNo,
			fmt.Sprintf("虚拟会员购买成功，支付金额: %.2f，支付方式: %s", completedOrder.Money, completedOrder.PaymentMethod),
		); err != nil {
			return err
		}
		if err := DB.Transaction(func(tx *gorm.DB) error {
			actualNet, netErr := PaymentSnapshotAfterFee(PaymentSnapshot{AmountMinor: completedOrder.ActualPaymentAmountMinor, Currency: completedOrder.ActualPaymentCurrency}, completedOrder.PaymentFee)
			if netErr != nil {
				return netErr
			}
			amountCents, conversionErr := RechargeCentsForPayment(actualNet)
			if conversionErr != nil {
				return conversionErr
			}
			_, luckyErr := RecordLuckyPaidOrderRechargeSnapshotTx(
				tx, completedOrder.UserId, amountCents,
				RechargeSourceVirtualMembership, completedOrder.TradeNo, completedOrder.CompleteTime,
				completedOrder.LuckyRuleSetId, completedOrder.LuckyEligible,
			)
			return luckyErr
		}); err != nil {
			return err
		}
	}
	return nil
}

func ExpireVirtualMembershipOrder(tradeNo, expectedPaymentProvider string) error {
	if strings.TrimSpace(tradeNo) == "" {
		return errors.New("tradeNo is empty")
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		var order VirtualMembershipOrder
		if err := tx.Set("gorm:query_option", "FOR UPDATE").Where("trade_no = ?", tradeNo).First(&order).Error; err != nil {
			return errors.New("虚拟会员订单不存在")
		}
		if expectedPaymentProvider != "" && order.PaymentProvider != expectedPaymentProvider {
			return ErrPaymentMethodMismatch
		}
		if order.Status != common.TopUpStatusPending {
			return nil
		}
		order.Status = VirtualMembershipOrderClosed
		order.CompleteTime = common.GetTimestamp()
		return tx.Save(&order).Error
	})
}

func PurchaseVirtualMembershipWithBalance(userId, planId, groupSize int) (*VirtualMembershipOrder, *UserVirtualMembership, error) {
	return PurchaseVirtualMembershipWithBalanceRenewal(userId, planId, groupSize, 0)
}

func PurchaseVirtualMembershipWithBalanceRenewal(userId, planId, groupSize, renewFromMembershipId int) (*VirtualMembershipOrder, *UserVirtualMembership, error) {
	if userId <= 0 {
		return nil, nil, errors.New("用户不存在")
	}
	now := common.GetTimestamp()
	var order VirtualMembershipOrder
	var membership UserVirtualMembership
	err := DB.Transaction(func(tx *gorm.DB) error {
		if renewFromMembershipId > 0 {
			if _, _, err := resolveVirtualMembershipRenewalTx(tx, userId, renewFromMembershipId, planId, groupSize, true, true); err != nil {
				return err
			}
		}
		plan, err := getVirtualMembershipPlanTx(tx, planId)
		if err != nil {
			return err
		}
		if !plan.Enabled {
			return errors.New("虚拟会员方案未启用")
		}
		price, weekly, fiveHour, concurrency, rpm, err := virtualMembershipVariantWithLimits(plan, groupSize)
		if err != nil {
			return err
		}
		costQuota := int64(math.Round(price * common.QuotaPerUnit))
		if costQuota <= 0 {
			return errors.New("虚拟会员价格换算额度无效")
		}
		result := tx.Model(&User{}).Where("id = ? AND quota >= ?", userId, costQuota).Update("quota", gorm.Expr("quota - ?", costQuota))
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return errors.New("本金余额不足，请先充值")
		}
		tradeNo := fmt.Sprintf("vm-%s-%s", common.GetTimeString(), common.GetRandomString(8))
		planSnapshot := buildVirtualMembershipSnapshot(plan, price, groupSize, weekly, fiveHour, concurrency, rpm)
		order = VirtualMembershipOrder{
			UserId: userId, PlanId: plan.Id, GroupSize: groupSize, Money: price,
			WeeklyQuota: weekly, FiveHourQuota: fiveHour, FiveHourActive: plan.FiveHourEnabled,
			ConcurrencyLimit: concurrency, RPMLimit: rpm,
			DividendState: SubscriptionDividendSkippedSource,
			TradeNo:       tradeNo, PaymentMethod: PaymentMethodBalance, PaymentProvider: PaymentProviderBalance, Status: VirtualMembershipOrderSuccess,
			ProductNameSnapshot: PaymentProductNameForVirtualMembership(plan.Title),
			CreateTime:          now, CompleteTime: now, PlanSnapshot: planSnapshot,
		}
		if renewFromMembershipId > 0 {
			order.RenewFromMembershipId = &renewFromMembershipId
		}
		if err := tx.Create(&order).Error; err != nil {
			return err
		}
		startTime := now
		var renewedFromId *int
		if renewFromMembershipId > 0 {
			var source UserVirtualMembership
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&source, renewFromMembershipId).Error; err != nil {
				return ErrVirtualMembershipRenewalUnavailable
			}
			if source.EndTime > startTime {
				startTime = source.EndTime
			}
			renewedFromId = &source.Id
		}
		end := startTime + int64(plan.DurationDays)*86400
		membership = UserVirtualMembership{
			UserId: userId, PlanId: plan.Id, OrderId: order.Id, PlanTitle: plan.Title, PlanCode: plan.Code,
			PurchasePriceAmount: price,
			RenewedFromId:       renewedFromId,
			GroupSize:           groupSize, WeeklyQuota: weekly, FiveHourQuota: fiveHour, FiveHourActive: plan.FiveHourEnabled,
			ConcurrencyLimit: concurrency, RPMLimit: rpm,
			WeeklyResetAt: startTime + 7*86400, FiveHourStart: startTime, FiveHourResetAt: startTime + 5*3600,
			StartTime: startTime, EndTime: end, Status: VirtualMembershipStatusActive,
			AllowedModels: plan.AllowedModels, AllowedGroup: plan.AllowedGroup,
			CreatedAt: now, UpdatedAt: now,
		}
		if err := tx.Create(&membership).Error; err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	InvalidateUserCache(userId)
	RecordLog(
		userId,
		LogTypeTopup,
		fmt.Sprintf("使用余额购买虚拟会员成功，支付金额: %.2f", order.Money),
	)
	return &order, &membership, nil
}

func ListUserVirtualMemberships(userId int) ([]*UserVirtualMembership, error) {
	return listUserVirtualMemberships(userId, false)
}

// ListHiddenUserVirtualMemberships returns only the owner-hidden instances so
// the user can restore their presentation on the virtual-membership page.
func ListHiddenUserVirtualMemberships(userId int) ([]*UserVirtualMembership, error) {
	return listUserVirtualMemberships(userId, true)
}

func listUserVirtualMemberships(userId int, hidden bool) ([]*UserVirtualMembership, error) {
	var memberships []*UserVirtualMembership
	err := DB.Transaction(func(tx *gorm.DB) error {
		// Reset-on-read updates the complete membership row. Keep the row lock
		// for the whole transaction so a concurrent quota consume, payment
		// callback, or reset-credit grant cannot be overwritten by a stale
		// snapshot while the list is being refreshed.
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("user_id = ? AND hidden = ?", userId, hidden).
			Order("id desc").Find(&memberships).Error; err != nil {
			return err
		}
		now := common.GetTimestamp()
		for _, membership := range memberships {
			if err := virtualMembershipResetIfDue(tx, membership, now); err != nil {
				return err
			}
		}
		active := memberships[:0]
		for _, membership := range memberships {
			if membership != nil && membership.Status == VirtualMembershipStatusActive && membership.EndTime > now {
				active = append(active, membership)
			}
		}
		memberships = active
		if err := attachVirtualMembershipLifetimeUsage(tx, memberships); err != nil {
			return err
		}
		return attachVirtualMembershipResetPrices(tx, memberships)
	})
	return memberships, err
}

// SetVirtualMembershipHidden changes only whether the instance is rendered on
// user-facing remaining-quota pages. It does not revoke or pause consumption.
func SetVirtualMembershipHidden(membershipId int, hidden bool) error {
	if membershipId <= 0 {
		return errors.New("虚拟会员不存在")
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		var membership UserVirtualMembership
		if err := tx.Select("id", "hidden").First(&membership, membershipId).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return errors.New("虚拟会员不存在")
			}
			return err
		}
		if membership.Hidden == hidden {
			return nil
		}
		return tx.Model(&UserVirtualMembership{}).Where("id = ?", membershipId).Updates(map[string]interface{}{
			"hidden":     hidden,
			"updated_at": common.GetTimestamp(),
		}).Error
	})
}

// SetVirtualMembershipHiddenForUser limits the presentation-only control to
// the owner of the membership instance. Hidden instances remain valid funding
// sources for runtime consumption and API-key binding.
func SetVirtualMembershipHiddenForUser(membershipId, userId int, hidden bool) error {
	if membershipId <= 0 || userId <= 0 {
		return errors.New("虚拟会员不存在")
	}
	var membership UserVirtualMembership
	if err := DB.Select("id", "hidden").Where("id = ? AND user_id = ?", membershipId, userId).First(&membership).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("虚拟会员不存在")
		}
		return err
	}
	if membership.Hidden == hidden {
		return nil
	}
	result := DB.Model(&UserVirtualMembership{}).
		Where("id = ? AND user_id = ?", membershipId, userId).
		Updates(map[string]interface{}{"hidden": hidden, "updated_at": common.GetTimestamp()})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return errors.New("虚拟会员不存在")
	}
	return nil
}

// ListAdminVirtualMemberships returns every purchased membership with its
// owner. Lifecycle and quota resets are applied before the values are exposed
// so the administrator sees the same current balance as the user dashboard.
func ListAdminVirtualMemberships() ([]*AdminVirtualMembershipRecord, error) {
	records := make([]*AdminVirtualMembershipRecord, 0)
	err := DB.Transaction(func(tx *gorm.DB) error {
		var memberships []*UserVirtualMembership
		// virtualMembershipResetIfDue may Save the full row below; lock all
		// selected instances before applying lazy lifecycle resets.
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Order("id desc").Find(&memberships).Error; err != nil {
			return err
		}

		now := common.GetTimestamp()
		userIds := make([]int, 0, len(memberships))
		seenUserIds := make(map[int]struct{}, len(memberships))
		for _, membership := range memberships {
			if err := virtualMembershipResetIfDue(tx, membership, now); err != nil {
				return err
			}
			if _, exists := seenUserIds[membership.UserId]; !exists {
				seenUserIds[membership.UserId] = struct{}{}
				userIds = append(userIds, membership.UserId)
			}
		}
		if err := attachVirtualMembershipLifetimeUsage(tx, memberships); err != nil {
			return err
		}
		if err := attachVirtualMembershipResetPrices(tx, memberships); err != nil {
			return err
		}

		usersById := make(map[int]User, len(userIds))
		if len(userIds) > 0 {
			var users []User
			if err := tx.Unscoped().Select("id", "username", "display_name", "email", "deleted_at").
				Where("id IN ?", userIds).Find(&users).Error; err != nil {
				return err
			}
			for _, user := range users {
				usersById[user.Id] = user
			}
		}

		for _, membership := range memberships {
			user := usersById[membership.UserId]
			records = append(records, &AdminVirtualMembershipRecord{
				Membership:  membership,
				Username:    user.Username,
				DisplayName: user.DisplayName,
				Email:       user.Email,
				UserDeleted: user.DeletedAt.Valid,
			})
		}
		return nil
	})
	return records, err
}

// AdminDeleteVirtualMembership removes an entitlement while preserving its
// order and settled usage audit records. Any bound API keys are detached in
// the same transaction. An in-flight pre-consumption blocks deletion because
// its settlement still needs the membership ledger row.
func AdminDeleteVirtualMembership(membershipId int) (int64, error) {
	if membershipId <= 0 {
		return 0, errors.New("虚拟会员不存在")
	}
	var unboundTokens int64
	var tokenKeys []string
	err := DB.Transaction(func(tx *gorm.DB) error {
		var membership UserVirtualMembership
		if err := tx.Set("gorm:query_option", "FOR UPDATE").First(&membership, membershipId).Error; err != nil {
			return err
		}

		var pendingCount int64
		if err := tx.Model(&VirtualMembershipPreConsumeRecord{}).
			Where("membership_id = ? AND status = ?", membershipId, VirtualMembershipRecordPending).
			Count(&pendingCount).Error; err != nil {
			return err
		}
		if pendingCount > 0 {
			return errors.New("该会员存在正在结算的请求，请稍后再删除")
		}
		var renewalCount int64
		if err := tx.Model(&VirtualMembershipOrder{}).
			Where("renew_from_membership_id = ? AND status = ?", membershipId, common.TopUpStatusPending).
			Count(&renewalCount).Error; err != nil {
			return err
		}
		if renewalCount == 0 {
			if err := tx.Model(&UserVirtualMembership{}).
				Where("renewed_from_id = ?", membershipId).
				Count(&renewalCount).Error; err != nil {
				return err
			}
		}
		if renewalCount > 0 {
			return errors.New("该会员已存在续费订单或后继实例，不能删除")
		}
		var pendingResetCount int64
		if err := tx.Model(&VirtualMembershipResetOrder{}).
			Where("membership_id = ? AND status = ?", membershipId, common.TopUpStatusPending).
			Count(&pendingResetCount).Error; err != nil {
			return err
		}
		if pendingResetCount > 0 {
			return errors.New("该会员存在正在支付的主动重置订单，请稍后再删除")
		}

		if err := tx.Model(&Token{}).
			Where("virtual_membership_id = ?", membershipId).
			Pluck("key", &tokenKeys).Error; err != nil {
			return err
		}
		result := tx.Model(&Token{}).
			Where("virtual_membership_id = ?", membershipId).
			Update("virtual_membership_id", 0)
		if result.Error != nil {
			return result.Error
		}
		unboundTokens = result.RowsAffected
		return tx.Where("id = ?", membershipId).Delete(&UserVirtualMembership{}).Error
	})
	if err == nil && common.RedisEnabled {
		for _, tokenKey := range tokenKeys {
			if cacheErr := cacheDeleteToken(tokenKey); cacheErr != nil {
				// Never log the token itself. The database commit remains the
				// source of truth and the cache entry expires naturally if Redis
				// is temporarily unavailable.
				common.SysLog(fmt.Sprintf("删除虚拟会员 #%d 后清理 API Key 缓存失败: %v", membershipId, cacheErr))
			}
		}
	}
	return unboundTokens, err
}

func ListVirtualMembershipOrders(userId int) ([]*VirtualMembershipOrder, error) {
	var orders []*VirtualMembershipOrder
	query := DB.Order("id desc")
	if userId > 0 {
		query = query.Where("user_id = ?", userId)
	}
	return orders, query.Find(&orders).Error
}

func GetVirtualMembershipByIdForUser(userId, membershipId int) (*UserVirtualMembership, error) {
	var membership UserVirtualMembership
	err := DB.Where("id = ? AND user_id = ?", membershipId, userId).First(&membership).Error
	return &membership, err
}

func ListUserVirtualMembershipTokens(userId, membershipId int) ([]*Token, error) {
	var tokens []*Token
	err := DB.Where("user_id = ? AND virtual_membership_id = ?", userId, membershipId).
		Order("id desc").Find(&tokens).Error
	for _, token := range tokens {
		if token != nil {
			token.Clean()
		}
	}
	return tokens, err
}

func ReplaceUserVirtualMembershipTokens(userId, membershipId int, tokenIds []int) error {
	return DB.Transaction(func(tx *gorm.DB) error {
		var membership UserVirtualMembership
		if err := tx.Where("id = ? AND user_id = ?", membershipId, userId).First(&membership).Error; err != nil {
			return err
		}
		if err := tx.Model(&Token{}).Where("user_id = ? AND virtual_membership_id = ?", userId, membershipId).
			Update("virtual_membership_id", 0).Error; err != nil {
			return err
		}
		seen := make(map[int]struct{}, len(tokenIds))
		for _, tokenId := range tokenIds {
			if tokenId <= 0 {
				continue
			}
			if _, exists := seen[tokenId]; exists {
				continue
			}
			seen[tokenId] = struct{}{}
			result := tx.Model(&Token{}).Where("id = ? AND user_id = ?", tokenId, userId).
				Update("virtual_membership_id", membershipId)
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected != 1 {
				return fmt.Errorf("API Key %d 不属于当前用户", tokenId)
			}
		}
		return nil
	})
}

func virtualMembershipResetIfDue(tx *gorm.DB, membership *UserVirtualMembership, now int64) error {
	if membership == nil {
		return errors.New("虚拟会员不存在")
	}
	changed := false
	if membership.WeeklyResetAt > 0 && now >= membership.WeeklyResetAt {
		membership.WeeklyUsed = 0
		membership.WeeklyResetAt = now + 7*86400
		changed = true
	}
	if membership.FiveHourActive && membership.FiveHourResetAt > 0 && now >= membership.FiveHourResetAt {
		membership.FiveHourUsed = 0
		membership.FiveHourStart = now
		membership.FiveHourResetAt = now + 5*3600
		changed = true
	}
	if membership.EndTime > 0 && now >= membership.EndTime && membership.Status == VirtualMembershipStatusActive {
		membership.Status = VirtualMembershipStatusExpired
		changed = true
	}
	if changed {
		membership.UpdatedAt = now
		return tx.Save(membership).Error
	}
	return nil
}

func virtualMembershipAllowsModel(membership *UserVirtualMembership, modelName string) bool {
	if membership == nil || strings.TrimSpace(membership.AllowedModels) == "" {
		return true
	}
	for _, item := range strings.Split(membership.AllowedModels, ",") {
		if strings.TrimSpace(item) == modelName {
			return true
		}
	}
	return false
}

// ValidateVirtualMembershipForToken checks ownership, lifecycle and group
// compatibility before an API Key is allowed to spend the entitlement.
func ValidateVirtualMembershipForToken(userId int, group string, membershipId int, requireUsable bool) error {
	if userId <= 0 || membershipId <= 0 {
		return errors.New("虚拟会员不存在")
	}
	var membership UserVirtualMembership
	if err := DB.Where("id = ? AND user_id = ?", membershipId, userId).First(&membership).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("虚拟会员不存在")
		}
		return err
	}
	allowedGroup := strings.TrimSpace(membership.AllowedGroup)
	if allowedGroup == "" {
		allowedGroup = VirtualMembershipDefaultAllowedGroup
	}
	if allowedGroup != strings.TrimSpace(group) {
		return errors.New("虚拟会员不支持当前 API Key 分组")
	}
	if !requireUsable {
		return nil
	}
	now := common.GetTimestamp()
	return DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Set("gorm:query_option", "FOR UPDATE").First(&membership, membership.Id).Error; err != nil {
			return err
		}
		if err := virtualMembershipResetIfDue(tx, &membership, now); err != nil {
			return err
		}
		if membership.Status != VirtualMembershipStatusActive || membership.StartTime > now || (membership.EndTime > 0 && membership.EndTime <= now) {
			return errors.New("虚拟会员尚未生效、已结束或已失效")
		}
		if membership.WeeklyQuota > 0 && membership.WeeklyUsed >= membership.WeeklyQuota {
			return errors.New("虚拟会员周额度已用尽")
		}
		if membership.FiveHourActive && membership.FiveHourQuota > 0 && membership.FiveHourUsed >= membership.FiveHourQuota {
			return errors.New("虚拟会员 5 小时额度已用尽")
		}
		return nil
	})
}

// HasUsableVirtualMembershipForRoute performs the same lazy reset used by
// billing and answers whether an auto-allocation route currently has at least
// one usable entitlement. An empty modelName is used by policy validation.
func HasUsableVirtualMembershipForRoute(userId int, group, modelName string) (bool, error) {
	if userId <= 0 || strings.TrimSpace(group) == "" {
		return false, nil
	}
	usable := false
	err := DB.Transaction(func(tx *gorm.DB) error {
		var memberships []UserVirtualMembership
		now := common.GetTimestamp()
		group = strings.TrimSpace(group)
		query := tx.Set("gorm:query_option", "FOR UPDATE").
			Where("user_id = ? AND status = ? AND start_time <= ? AND end_time > ?",
				userId, VirtualMembershipStatusActive, now, now)
		if group == VirtualMembershipDefaultAllowedGroup {
			query = query.Where("(allowed_group = ? OR allowed_group = '')", group)
		} else {
			query = query.Where("allowed_group = ?", group)
		}
		if err := query.Order("weekly_reset_at asc, end_time asc, id asc").Find(&memberships).Error; err != nil {
			return err
		}
		for i := range memberships {
			// Lock the membership before inspecting pending settlements. The
			// request pre-consumption path takes the same row lock, so a reset
			// cannot race a new in-flight usage hold.
			var locked UserVirtualMembership
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&locked, memberships[i].Id).Error; err != nil {
				return err
			}
			membership := &locked
			if err := virtualMembershipResetIfDue(tx, membership, now); err != nil {
				return err
			}
			if modelName != "" && !virtualMembershipAllowsModel(membership, modelName) {
				continue
			}
			if membership.WeeklyQuota > 0 && membership.WeeklyUsed >= membership.WeeklyQuota {
				continue
			}
			if membership.FiveHourActive && membership.FiveHourQuota > 0 && membership.FiveHourUsed >= membership.FiveHourQuota {
				continue
			}
			usable = true
			break
		}
		return nil
	})
	return usable, err
}

// PreConsumeVirtualMembershipAuto selects and reserves one membership in a
// single transaction. The earliest weekly reset wins; exhausted instances are
// skipped and no wallet fallback is ever attempted here.
func PreConsumeVirtualMembershipAuto(requestId string, userId int, modelName string, amount int64, usingGroup string) (*UserVirtualMembership, error) {
	if amount <= 0 {
		return nil, errors.New("虚拟会员预扣额度必须大于 0")
	}
	var selected UserVirtualMembership
	err := DB.Transaction(func(tx *gorm.DB) error {
		var existing VirtualMembershipPreConsumeRecord
		if result := tx.Where("request_id = ?", requestId).Limit(1).Find(&existing); result.Error != nil {
			return result.Error
		} else if result.RowsAffected > 0 {
			return tx.Where("id = ? AND user_id = ?", existing.MembershipId, userId).First(&selected).Error
		}
		now := common.GetTimestamp()
		var memberships []UserVirtualMembership
		usingGroup = strings.TrimSpace(usingGroup)
		query := tx.Set("gorm:query_option", "FOR UPDATE").
			Where("user_id = ? AND status = ? AND start_time <= ? AND end_time > ?",
				userId, VirtualMembershipStatusActive, now, now)
		if usingGroup == VirtualMembershipDefaultAllowedGroup {
			query = query.Where("(allowed_group = ? OR allowed_group = '')", usingGroup)
		} else {
			query = query.Where("allowed_group = ?", usingGroup)
		}
		if err := query.Order("weekly_reset_at asc, end_time asc, id asc").Find(&memberships).Error; err != nil {
			return err
		}
		for i := range memberships {
			membership := &memberships[i]
			if err := virtualMembershipResetIfDue(tx, membership, now); err != nil {
				return err
			}
			if !virtualMembershipAllowsModel(membership, modelName) {
				continue
			}
			if membership.WeeklyQuota > 0 && membership.WeeklyUsed+amount > membership.WeeklyQuota {
				continue
			}
			if membership.FiveHourActive && membership.FiveHourQuota > 0 && membership.FiveHourUsed+amount > membership.FiveHourQuota {
				continue
			}
			if err := preConsumeVirtualMembershipTx(tx, requestId, membership.Id, userId, modelName, usingGroup, amount); err != nil {
				return err
			}
			selected = *membership
			selected.WeeklyUsed += amount
			if selected.FiveHourActive {
				selected.FiveHourUsed += amount
			}
			return nil
		}
		return errors.New("当前虚拟会员分组的全部周额度或 5 小时额度均不足")
	})
	if err != nil {
		return nil, err
	}
	return &selected, nil
}

func preConsumeVirtualMembershipTx(tx *gorm.DB, requestId string, membershipId, userId int, modelName, usingGroup string, amount int64) error {
	if amount <= 0 {
		return errors.New("虚拟会员预扣额度必须大于 0")
	}
	if strings.TrimSpace(requestId) == "" {
		return errors.New("请求 ID 不能为空")
	}
	var existing VirtualMembershipPreConsumeRecord
	if err := tx.Where("request_id = ?", requestId).First(&existing).Error; err == nil {
		return nil
	}
	var membership UserVirtualMembership
	if err := tx.Set("gorm:query_option", "FOR UPDATE").Where("id = ? AND user_id = ?", membershipId, userId).First(&membership).Error; err != nil {
		return err
	}
	now := common.GetTimestamp()
	if err := virtualMembershipResetIfDue(tx, &membership, now); err != nil {
		return err
	}
	if membership.Status != VirtualMembershipStatusActive {
		return errors.New("虚拟会员已失效")
	}
	if membership.StartTime > now {
		return errors.New("虚拟会员尚未生效")
	}
	if membership.EndTime > 0 && now >= membership.EndTime {
		return errors.New("虚拟会员已过期")
	}
	if !virtualMembershipAllowsModel(&membership, modelName) {
		return fmt.Errorf("模型 %s 不在虚拟会员允许范围内", modelName)
	}
	if membership.AllowedGroup != "" && membership.AllowedGroup != usingGroup {
		return errors.New("当前分组不在虚拟会员允许范围内")
	}
	if membership.WeeklyQuota > 0 && membership.WeeklyUsed+amount > membership.WeeklyQuota {
		return errors.New("虚拟会员周额度不足")
	}
	if membership.FiveHourActive && membership.FiveHourQuota > 0 && membership.FiveHourUsed+amount > membership.FiveHourQuota {
		return errors.New("虚拟会员 5 小时额度不足")
	}
	membership.WeeklyUsed += amount
	if membership.FiveHourActive {
		membership.FiveHourUsed += amount
	}
	membership.UpdatedAt = now
	if err := tx.Save(&membership).Error; err != nil {
		return err
	}
	record := &VirtualMembershipPreConsumeRecord{
		RequestId: requestId, MembershipId: membership.Id, UserId: userId,
		PreConsumed: amount, Status: VirtualMembershipRecordPending, CreatedAt: now, UpdatedAt: now,
	}
	return tx.Create(record).Error
}

func PreConsumeVirtualMembershipForToken(requestId string, userId int, modelName string, amount int64, usingGroup string, membershipId int) error {
	return DB.Transaction(func(tx *gorm.DB) error {
		return preConsumeVirtualMembershipTx(tx, requestId, membershipId, userId, modelName, usingGroup, amount)
	})
}

func PostConsumeVirtualMembershipDelta(requestId string, delta int64) error {
	return DB.Transaction(func(tx *gorm.DB) error {
		var record VirtualMembershipPreConsumeRecord
		if err := tx.Where("request_id = ?", requestId).First(&record).Error; err != nil {
			return err
		}
		if record.Status != VirtualMembershipRecordPending {
			return nil
		}
		var membership UserVirtualMembership
		// Keep the lock order consistent with administrator resets and the
		// pre-consume path: membership first, then its ledger record. This
		// prevents a settlement racing an explicit reset from deadlocking.
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&membership, record.MembershipId).Error; err != nil {
			return err
		}
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&record, record.Id).Error; err != nil {
			return err
		}
		if record.Status != VirtualMembershipRecordPending {
			// An administrator reset may have finalized this request while the
			// initial non-locking lookup was in progress.
			return nil
		}
		if delta == 0 {
			record.FinalQuota = record.PreConsumed
			record.Status = VirtualMembershipRecordSettled
			record.UpdatedAt = common.GetTimestamp()
			return tx.Save(&record).Error
		}
		membership.WeeklyUsed += delta
		if membership.FiveHourActive {
			membership.FiveHourUsed += delta
		}
		if membership.WeeklyUsed < 0 || membership.FiveHourUsed < 0 {
			return errors.New("虚拟会员额度结算后不能为负数")
		}
		membership.UpdatedAt = common.GetTimestamp()
		if err := tx.Save(&membership).Error; err != nil {
			return err
		}
		record.FinalQuota = record.PreConsumed + delta
		record.Status = VirtualMembershipRecordSettled
		record.UpdatedAt = common.GetTimestamp()
		return tx.Save(&record).Error
	})
}

func PreConsumeVirtualMembershipDelta(requestId string, delta int64) error {
	if delta <= 0 {
		return nil
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		var record VirtualMembershipPreConsumeRecord
		if err := tx.Where("request_id = ?", requestId).First(&record).Error; err != nil {
			return err
		}
		if record.Status != VirtualMembershipRecordPending {
			return errors.New("虚拟会员预扣记录已结束")
		}
		var membership UserVirtualMembership
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&membership, record.MembershipId).Error; err != nil {
			return err
		}
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&record, record.Id).Error; err != nil {
			return err
		}
		if record.Status != VirtualMembershipRecordPending {
			return errors.New("虚拟会员预扣记录已结束")
		}
		if membership.WeeklyQuota > 0 && membership.WeeklyUsed+delta > membership.WeeklyQuota {
			return errors.New("虚拟会员周额度不足")
		}
		if membership.FiveHourActive && membership.FiveHourQuota > 0 && membership.FiveHourUsed+delta > membership.FiveHourQuota {
			return errors.New("虚拟会员 5 小时额度不足")
		}
		membership.WeeklyUsed += delta
		if membership.FiveHourActive {
			membership.FiveHourUsed += delta
		}
		membership.UpdatedAt = common.GetTimestamp()
		if err := tx.Save(&membership).Error; err != nil {
			return err
		}
		record.PreConsumed += delta
		record.UpdatedAt = common.GetTimestamp()
		return tx.Save(&record).Error
	})
}

func RollbackVirtualMembershipDelta(requestId string, delta int64) error {
	if delta <= 0 {
		return nil
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		var record VirtualMembershipPreConsumeRecord
		if err := tx.Where("request_id = ?", requestId).First(&record).Error; err != nil {
			return err
		}
		if record.Status != VirtualMembershipRecordPending || record.PreConsumed < delta {
			return errors.New("虚拟会员补充预扣记录无效")
		}
		var membership UserVirtualMembership
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&membership, record.MembershipId).Error; err != nil {
			return err
		}
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&record, record.Id).Error; err != nil {
			return err
		}
		if record.Status != VirtualMembershipRecordPending || record.PreConsumed < delta {
			return errors.New("虚拟会员补充预扣记录无效")
		}
		membership.WeeklyUsed -= delta
		if membership.FiveHourActive {
			membership.FiveHourUsed -= delta
		}
		if membership.WeeklyUsed < 0 || membership.FiveHourUsed < 0 {
			return errors.New("虚拟会员回滚后额度不能为负数")
		}
		membership.UpdatedAt = common.GetTimestamp()
		if err := tx.Save(&membership).Error; err != nil {
			return err
		}
		record.PreConsumed -= delta
		record.UpdatedAt = common.GetTimestamp()
		return tx.Save(&record).Error
	})
}

func RefundVirtualMembershipPreConsume(requestId string) error {
	return DB.Transaction(func(tx *gorm.DB) error {
		var record VirtualMembershipPreConsumeRecord
		if err := tx.Where("request_id = ?", requestId).First(&record).Error; err != nil {
			return err
		}
		if record.Status != VirtualMembershipRecordPending {
			return nil
		}
		var membership UserVirtualMembership
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&membership, record.MembershipId).Error; err != nil {
			return err
		}
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&record, record.Id).Error; err != nil {
			return err
		}
		if record.Status != VirtualMembershipRecordPending {
			return nil
		}
		membership.WeeklyUsed -= record.PreConsumed
		if membership.FiveHourActive {
			membership.FiveHourUsed -= record.PreConsumed
		}
		if membership.WeeklyUsed < 0 || membership.FiveHourUsed < 0 {
			return errors.New("虚拟会员退款后额度不能为负数")
		}
		membership.UpdatedAt = common.GetTimestamp()
		if err := tx.Save(&membership).Error; err != nil {
			return err
		}
		record.Status = VirtualMembershipRecordRefunded
		record.UpdatedAt = common.GetTimestamp()
		return tx.Save(&record).Error
	})
}

type VirtualMembershipResetScope struct {
	MembershipId int
	UserId       int
	PlanCode     string
}

func ResetVirtualMemberships(scope VirtualMembershipResetScope) (int64, error) {
	now := common.GetTimestamp()
	var affected int64
	err := DB.Transaction(func(tx *gorm.DB) error {
		query := tx.Model(&UserVirtualMembership{}).
			Where("status = ? AND start_time <= ? AND end_time > ?", VirtualMembershipStatusActive, now, now)
		if scope.MembershipId > 0 {
			query = query.Where("id = ?", scope.MembershipId)
		}
		if scope.UserId > 0 {
			query = query.Where("user_id = ?", scope.UserId)
		}
		if code := strings.TrimSpace(strings.ToLower(scope.PlanCode)); code != "" {
			query = query.Where("LOWER(plan_code) = ?", code)
		}
		// Hold each selected membership row for the duration of the reset
		// transaction. The usage pre-consume path takes the same row lock, so a
		// request cannot create a pending settlement between the count check and
		// the quota reset update.
		query = query.Clauses(clause.Locking{Strength: "UPDATE"})
		var memberships []UserVirtualMembership
		if err := query.Find(&memberships).Error; err != nil {
			return err
		}
		membershipIds := make([]int, 0, len(memberships))
		for i := range memberships {
			membershipIds = append(membershipIds, memberships[i].Id)
		}
		if len(membershipIds) > 0 {
			// An explicit administrator reset supersedes unfinished requests. The
			// quota window is being replaced, so pending pre-consumption records
			// must be finalized as refunded before the reset; otherwise they would
			// permanently block this instance or add usage back later.
			if err := tx.Model(&VirtualMembershipPreConsumeRecord{}).
				Where("membership_id IN ? AND status = ?", membershipIds, VirtualMembershipRecordPending).
				Updates(map[string]interface{}{
					"status":     VirtualMembershipRecordRefunded,
					"updated_at": now,
				}).Error; err != nil {
				return err
			}
		}
		for i := range memberships {
			membership := &memberships[i]
			result := tx.Model(&UserVirtualMembership{}).Where("id = ?", membership.Id).
				Updates(map[string]interface{}{
					"weekly_used": 0, "five_hour_used": 0, "weekly_reset_at": now + 7*86400,
					"five_hour_start": now, "five_hour_reset_at": now + 5*3600, "updated_at": now,
				})
			if result.Error != nil {
				return result.Error
			}
			affected += result.RowsAffected
		}
		return nil
	})
	return affected, err
}

func ResetAllVirtualMemberships() (int64, error) {
	return ResetVirtualMemberships(VirtualMembershipResetScope{})
}

// GrantVirtualMembershipResetCredits atomically adds user-triggered reset
// credits to one membership.  It is intentionally ownership-agnostic because
// the caller is protected by the administrator route; the row lock makes two
// concurrent grants deterministic.
func GrantVirtualMembershipResetCredits(membershipId, credits int) (*UserVirtualMembership, error) {
	if membershipId <= 0 || credits <= 0 {
		return nil, errors.New("主动重置次数必须大于 0")
	}
	var membership UserVirtualMembership
	err := DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&membership, membershipId).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return errors.New("虚拟会员不存在")
			}
			return err
		}
		now := common.GetTimestamp()
		result := tx.Model(&UserVirtualMembership{}).Where("id = ?", membershipId).
			Updates(map[string]interface{}{
				"active_reset_credits": gorm.Expr("active_reset_credits + ?", credits),
				"updated_at":           now,
			})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return errors.New("虚拟会员不存在")
		}
		membership.ActiveResetCredits += credits
		membership.UpdatedAt = now
		if price, priceErr := virtualMembershipPurchasePriceTx(tx, &membership); priceErr == nil {
			membership.PurchasePriceAmount = price
			membership.ActiveResetPriceAmount = VirtualMembershipActiveResetPrice(price)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &membership, nil
}

// ActiveResetVirtualMembership consumes one administrator-granted credit and
// performs the same reset as ResetVirtualMemberships: weekly and optional
// five-hour usage are cleared, and both reset anchors move from the action
// time.  Ownership and lifecycle are checked inside the locking transaction.
func ActiveResetVirtualMembership(userId, membershipId int) (*UserVirtualMembership, error) {
	if userId <= 0 || membershipId <= 0 {
		return nil, errors.New("虚拟会员不存在")
	}
	var membership UserVirtualMembership
	err := DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND user_id = ?", membershipId, userId).First(&membership).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return errors.New("虚拟会员不存在")
			}
			return err
		}
		now := common.GetTimestamp()
		if membership.Status != VirtualMembershipStatusActive || membership.StartTime > now || (membership.EndTime > 0 && membership.EndTime <= now) {
			return errors.New("虚拟会员尚未生效、已结束或已失效")
		}
		if membership.ActiveResetCredits <= 0 {
			return ErrVirtualMembershipResetCreditsInsufficient
		}
		var pending []VirtualMembershipPreConsumeRecord
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("membership_id = ? AND status = ?", membershipId, VirtualMembershipRecordPending).
			Find(&pending).Error; err != nil {
			return err
		}
		cutoff := now - int64(virtualMembershipPendingSettlementTimeout()/time.Second)
		for _, record := range pending {
			lastActivity := record.UpdatedAt
			if lastActivity <= 0 {
				lastActivity = record.CreatedAt
			}
			if lastActivity > cutoff {
				return ErrVirtualMembershipSettlementInProgress
			}
		}
		if len(pending) > 0 {
			// A timed-out request can no longer safely hold this quota window.
			// Mark it refunded before moving the window so a late settlement sees
			// a terminal row and cannot add usage back after the reset.
			if err := tx.Model(&VirtualMembershipPreConsumeRecord{}).
				Where("membership_id = ? AND status = ?", membershipId, VirtualMembershipRecordPending).
				Updates(map[string]interface{}{"status": VirtualMembershipRecordRefunded, "updated_at": now}).Error; err != nil {
				return err
			}
		}
		membership.WeeklyUsed = 0
		membership.WeeklyResetAt = now + 7*86400
		// Keep the exact same timestamp semantics as the administrator reset,
		// including clearing dormant 5-hour fields for future plan edits.
		membership.FiveHourUsed = 0
		membership.FiveHourStart = now
		membership.FiveHourResetAt = now + 5*3600
		membership.ActiveResetCredits--
		membership.UpdatedAt = now
		if price, priceErr := virtualMembershipPurchasePriceTx(tx, &membership); priceErr == nil {
			membership.PurchasePriceAmount = price
			membership.ActiveResetPriceAmount = VirtualMembershipActiveResetPrice(price)
		}
		return tx.Save(&membership).Error
	})
	if err != nil {
		return nil, err
	}
	return &membership, nil
}

// GetVirtualMembershipActiveResetPrice resolves the immutable tier price and
// applies the 30% add-on rule. It is used by both the user endpoint and the
// payment-order creation path, so display and settlement cannot diverge.
func GetVirtualMembershipActiveResetPrice(userId, membershipId int) (float64, error) {
	if userId <= 0 || membershipId <= 0 {
		return 0, errors.New("虚拟会员不存在")
	}
	var membership UserVirtualMembership
	err := DB.Where("id = ? AND user_id = ?", membershipId, userId).First(&membership).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return 0, errors.New("虚拟会员不存在")
		}
		return 0, err
	}
	price, err := virtualMembershipPurchasePriceTx(DB, &membership)
	if err != nil {
		return 0, err
	}
	resetPrice := VirtualMembershipActiveResetPrice(price)
	if resetPrice < 0.01 {
		return 0, errors.New("主动重置价格无效")
	}
	return resetPrice, nil
}

// CreateVirtualMembershipResetEpayOrder creates at most one pending add-on
// order for a membership. Reusing an existing pending order makes browser
// retries and double-clicks idempotent before the provider callback arrives.
func CreateVirtualMembershipResetEpayOrder(userId, membershipId int, tradeNo, paymentMethod, _ string) (*VirtualMembershipResetOrder, error) {
	if userId <= 0 || membershipId <= 0 || strings.TrimSpace(tradeNo) == "" {
		return nil, errors.New("主动重置订单参数无效")
	}
	var order *VirtualMembershipResetOrder
	err := DB.Transaction(func(tx *gorm.DB) error {
		var membership UserVirtualMembership
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND user_id = ?", membershipId, userId).First(&membership).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return errors.New("虚拟会员不存在")
			}
			return err
		}
		now := common.GetTimestamp()
		if membership.Status != VirtualMembershipStatusActive || membership.StartTime > now || (membership.EndTime > 0 && membership.EndTime <= now) {
			return errors.New("虚拟会员尚未生效、已结束或已失效")
		}
		if membership.ActiveResetCredits > 0 {
			return errors.New("已有主动重置次数，请先使用现有次数")
		}
		var pending VirtualMembershipResetOrder
		if result := tx.Where("membership_id = ? AND user_id = ? AND status = ?", membershipId, userId, common.TopUpStatusPending).Order("id desc").First(&pending); result.Error == nil {
			order = &pending
			return nil
		} else if !errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return result.Error
		}
		price, priceErr := virtualMembershipPurchasePriceTx(tx, &membership)
		if priceErr != nil {
			return priceErr
		}
		price = VirtualMembershipActiveResetPrice(price)
		price = NormalizePaymentAmount(price)
		if price < 0.01 {
			return errors.New("主动重置价格无效")
		}
		snapshot, snapshotErr := NewPaymentSnapshotFromMoney(price, "CNY")
		if snapshotErr != nil {
			return snapshotErr
		}
		// Product names are gateway-facing audit data.  Always derive them from
		// the server-side plan snapshot; never trust a browser/internal caller's
		// free-form label (including on a retry of a pending order).
		title := strings.TrimSpace(membership.PlanTitle)
		if title == "" && membership.PlanId > 0 {
			if plan, planErr := getVirtualMembershipPlanTx(tx, membership.PlanId); planErr == nil {
				title = plan.Title
			}
		}
		productName := PaymentProductNameForVirtualMembershipReset(title)
		order = &VirtualMembershipResetOrder{
			UserId: userId, MembershipId: membershipId, Credits: 1, Money: price,
			TradeNo: tradeNo, PaymentMethod: paymentMethod, PaymentProvider: PaymentProviderEpay,
			Status: common.TopUpStatusPending, CreateTime: now, ProductNameSnapshot: productName,
			ExpectedPaymentAmountMinor: snapshot.AmountMinor, ExpectedPaymentCurrency: snapshot.Currency,
		}
		return tx.Create(order).Error
	})
	return order, err
}

func ExpireVirtualMembershipResetOrder(tradeNo, expectedPaymentProvider string) error {
	if strings.TrimSpace(tradeNo) == "" {
		return errors.New("tradeNo is empty")
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		var order VirtualMembershipResetOrder
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("trade_no = ?", tradeNo).First(&order).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return errors.New("主动重置订单不存在")
			}
			return err
		}
		if expectedPaymentProvider != "" && order.PaymentProvider != expectedPaymentProvider {
			return ErrPaymentMethodMismatch
		}
		if order.Status == common.TopUpStatusPending {
			order.Status = VirtualMembershipOrderClosed
			order.CompleteTime = common.GetTimestamp()
			return tx.Save(&order).Error
		}
		return nil
	})
}

func CompleteVirtualMembershipResetOrder(tradeNo, providerPayload, expectedPaymentProvider, actualPaymentMethod string, paymentSnapshots ...PaymentSnapshot) error {
	if strings.TrimSpace(tradeNo) == "" {
		return errors.New("tradeNo is empty")
	}
	if len(paymentSnapshots) != 1 {
		return errors.New("verified payment snapshot is required")
	}
	actual, err := NewPaymentSnapshotFromMinor(paymentSnapshots[0].AmountMinor, paymentSnapshots[0].Currency)
	if err != nil {
		return err
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		var order VirtualMembershipResetOrder
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("trade_no = ?", tradeNo).First(&order).Error; err != nil {
			return errors.New("主动重置订单不存在")
		}
		if expectedPaymentProvider != "" && order.PaymentProvider != expectedPaymentProvider {
			return ErrPaymentMethodMismatch
		}
		if err := ValidatePaymentSnapshot(order.ExpectedPaymentAmountMinor, order.ExpectedPaymentCurrency, actual); err != nil {
			return err
		}
		if order.ActualPaymentAmountMinor > 0 && (order.ActualPaymentAmountMinor != actual.AmountMinor || !strings.EqualFold(order.ActualPaymentCurrency, actual.Currency)) {
			return ErrPaymentSnapshotMismatch
		}
		if order.Status == common.TopUpStatusSuccess {
			return nil
		}
		if order.Status != common.TopUpStatusPending {
			return errors.New("主动重置订单状态无效")
		}
		var membership UserVirtualMembership
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND user_id = ?", order.MembershipId, order.UserId).First(&membership).Error; err != nil {
			return errors.New("虚拟会员不存在")
		}
		credits := order.Credits
		if credits <= 0 {
			credits = 1
		}
		membership.ActiveResetCredits += credits
		membership.UpdatedAt = common.GetTimestamp()
		if err := tx.Save(&membership).Error; err != nil {
			return err
		}
		order.ActualPaymentAmountMinor = actual.AmountMinor
		order.ActualPaymentCurrency = actual.Currency
		order.Status = VirtualMembershipOrderSuccess
		order.CompleteTime = common.GetTimestamp()
		if providerPayload != "" {
			order.ProviderPayload = providerPayload
		}
		if actualPaymentMethod != "" {
			order.PaymentMethod = actualPaymentMethod
		}
		return tx.Save(&order).Error
	})
}

// CompleteVirtualMembershipPayment dispatches a verified provider callback
// to the correct idempotent ledger. Existing membership orders keep their
// original settlement behavior; reset add-ons never create memberships.
func CompleteVirtualMembershipPayment(tradeNo, providerPayload, expectedPaymentProvider, actualPaymentMethod string, paymentSnapshots ...PaymentSnapshot) error {
	var resetOrder VirtualMembershipResetOrder
	err := DB.Select("id").Where("trade_no = ?", tradeNo).First(&resetOrder).Error
	if err == nil {
		return CompleteVirtualMembershipResetOrder(tradeNo, providerPayload, expectedPaymentProvider, actualPaymentMethod, paymentSnapshots...)
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	return CompleteVirtualMembershipOrder(tradeNo, providerPayload, expectedPaymentProvider, actualPaymentMethod, paymentSnapshots...)
}

func EnsureVirtualMembershipMigrations() error {
	if err := EnsureDefaultVirtualMembershipSetting(); err != nil {
		return err
	}
	groups := []string{VirtualMembershipDefaultAllowedGroup}
	var storedGroups []string
	if err := DB.Model(&VirtualMembershipPlan{}).Distinct("allowed_group").Pluck("allowed_group", &storedGroups).Error; err != nil {
		return err
	}
	groups = append(groups, storedGroups...)
	for _, group := range groups {
		if err := EnsureVirtualMembershipGroupRegistered(group); err != nil {
			return err
		}
	}
	var count int64
	if err := DB.Model(&VirtualMembershipPlan{}).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	// Seed the three requested product slots without making unconfigured
	// products purchasable. The administrator fills price and quota before
	// enabling them.
	plans := []*VirtualMembershipPlan{
		{Code: "plus", Title: "GPT Plus", Enabled: false, SortOrder: 10},
		{Code: "pro5x", Title: "GPT Pro 5x", Enabled: false, SortOrder: 20},
		{Code: "pro20x", Title: "GPT Pro 20x", Enabled: false, SortOrder: 30},
	}
	for _, plan := range plans {
		plan.Normalize()
		if err := DB.Create(plan).Error; err != nil {
			return err
		}
	}
	return nil
}

func VirtualMembershipQuotaPercent(used, total int64) int {
	if total <= 0 {
		return 0
	}
	percent := int(math.Round(float64(used) * 100 / float64(total)))
	if percent < 0 {
		return 0
	}
	if percent > 100 {
		return 100
	}
	return percent
}

func VirtualMembershipNextResetText(membership *UserVirtualMembership) string {
	if membership == nil {
		return ""
	}
	next := membership.WeeklyResetAt
	if membership.FiveHourActive && membership.FiveHourResetAt > 0 && (next == 0 || membership.FiveHourResetAt < next) {
		next = membership.FiveHourResetAt
	}
	if next <= 0 {
		return ""
	}
	return time.Unix(next, 0).Format("01/02 15:04")
}
