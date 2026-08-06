package model

import (
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
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
	Id              int     `json:"id"`
	Code            string  `json:"code" gorm:"uniqueIndex;type:varchar(64)"`
	Title           string  `json:"title" gorm:"type:varchar(128)"`
	Subtitle        string  `json:"subtitle" gorm:"type:varchar(255)"`
	Description     string  `json:"description" gorm:"type:text"`
	PriceAmount     float64 `json:"price_amount"`
	TwoGroupPrice   float64 `json:"two_group_price"`
	ThreeGroupPrice float64 `json:"three_group_price"`
	FourGroupPrice  float64 `json:"four_group_price"`
	Currency        string  `json:"currency" gorm:"type:varchar(8);not null;default:'USD'"`
	DurationDays    int     `json:"duration_days" gorm:"not null;default:30"`
	WeeklyQuota     int64   `json:"weekly_quota" gorm:"type:bigint;not null;default:0"`
	FiveHourEnabled bool    `json:"five_hour_enabled" gorm:"not null;default:false"`
	FiveHourQuota   int64   `json:"five_hour_quota" gorm:"type:bigint;not null;default:0"`
	AllowedModels   string  `json:"allowed_models" gorm:"type:text"`
	AllowedGroup    string  `json:"allowed_group" gorm:"type:varchar(64);default:''"`
	Recommended     bool    `json:"recommended" gorm:"not null;default:false"`
	Enabled         bool    `json:"enabled" gorm:"not null;default:true;index"`
	SortOrder       int     `json:"sort_order" gorm:"not null;default:0"`
	CreatedAt       int64   `json:"created_at" gorm:"bigint"`
	UpdatedAt       int64   `json:"updated_at" gorm:"bigint"`
}

type VirtualMembershipOrder struct {
	Id              int     `json:"id"`
	UserId          int     `json:"user_id" gorm:"index"`
	PlanId          int     `json:"plan_id" gorm:"index"`
	GroupSize       int     `json:"group_size" gorm:"not null;default:1"`
	Money           float64 `json:"money"`
	WeeklyQuota     int64   `json:"weekly_quota" gorm:"type:bigint;not null;default:0"`
	FiveHourQuota   int64   `json:"five_hour_quota" gorm:"type:bigint;not null;default:0"`
	FiveHourActive  bool    `json:"five_hour_active" gorm:"not null;default:false"`
	TradeNo         string  `json:"trade_no" gorm:"uniqueIndex;type:varchar(255)"`
	PaymentMethod   string  `json:"payment_method" gorm:"type:varchar(50)"`
	PaymentProvider string  `json:"payment_provider" gorm:"type:varchar(50);default:''"`
	Status          string  `json:"status" gorm:"type:varchar(32);index"`
	CreateTime      int64   `json:"create_time" gorm:"bigint"`
	CompleteTime    int64   `json:"complete_time" gorm:"bigint"`
	ProviderPayload string  `json:"-" gorm:"type:text"`
	PlanSnapshot    string  `json:"-" gorm:"type:text"`
}

type UserVirtualMembership struct {
	Id              int    `json:"id"`
	UserId          int    `json:"user_id" gorm:"index"`
	PlanId          int    `json:"plan_id" gorm:"index"`
	OrderId         int    `json:"order_id" gorm:"index"`
	PlanTitle       string `json:"plan_title" gorm:"type:varchar(128)"`
	PlanCode        string `json:"plan_code" gorm:"type:varchar(64)"`
	GroupSize       int    `json:"group_size" gorm:"not null;default:1"`
	WeeklyQuota     int64  `json:"weekly_quota" gorm:"type:bigint;not null;default:0"`
	WeeklyUsed      int64  `json:"weekly_used" gorm:"type:bigint;not null;default:0"`
	FiveHourQuota   int64  `json:"five_hour_quota" gorm:"type:bigint;not null;default:0"`
	FiveHourUsed    int64  `json:"five_hour_used" gorm:"type:bigint;not null;default:0"`
	FiveHourActive  bool   `json:"five_hour_active" gorm:"not null;default:false"`
	WeeklyResetAt   int64  `json:"weekly_reset_at" gorm:"bigint;index"`
	FiveHourStart   int64  `json:"five_hour_start" gorm:"bigint"`
	FiveHourResetAt int64  `json:"five_hour_reset_at" gorm:"bigint"`
	StartTime       int64  `json:"start_time" gorm:"bigint"`
	EndTime         int64  `json:"end_time" gorm:"bigint;index"`
	Status          string `json:"status" gorm:"type:varchar(32);index"`
	AllowedModels   string `json:"allowed_models" gorm:"type:text"`
	AllowedGroup    string `json:"allowed_group" gorm:"type:varchar(64);default:''"`
	CreatedAt       int64  `json:"created_at" gorm:"bigint"`
	UpdatedAt       int64  `json:"updated_at" gorm:"bigint"`
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

func (p *VirtualMembershipPlan) Validate() error {
	if p == nil || p.Code == "" || p.Title == "" {
		return errors.New("虚拟会员编码和名称不能为空")
	}
	if strings.ContainsAny(p.Code, " /\\") {
		return errors.New("虚拟会员编码不能包含空格或路径分隔符")
	}
	if p.PriceAmount < 0 || p.TwoGroupPrice < 0 || p.ThreeGroupPrice < 0 || p.FourGroupPrice < 0 {
		return errors.New("虚拟会员价格不能为负数")
	}
	if p.WeeklyQuota < 0 || p.FiveHourQuota < 0 {
		return errors.New("虚拟会员额度不能为负数")
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
	return DB.Save(plan).Error
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

func virtualMembershipVariant(plan *VirtualMembershipPlan, groupSize int) (float64, int64, int64, error) {
	if groupSize != 1 && groupSize != 2 && groupSize != 3 && groupSize != 4 {
		return 0, 0, 0, errors.New("虚拟会员仅支持单独购买、2 人团、3 人团和 4 人团")
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
		return 0, 0, 0, errors.New("该虚拟会员方案暂未配置此档位价格")
	}
	return price, plan.WeeklyQuota / int64(groupSize), plan.FiveHourQuota / int64(groupSize), nil
}

// VirtualMembershipVariantForDisplay exposes the same fallback and quota
// division rule used by checkout, so the UI never displays a zero-price group
// that the backend would silently charge at the base price.
func VirtualMembershipVariantForDisplay(plan *VirtualMembershipPlan, groupSize int) (float64, int64, int64, error) {
	return virtualMembershipVariant(plan, groupSize)
}

type virtualMembershipSnapshot struct {
	PlanId          int     `json:"plan_id"`
	Code            string  `json:"code"`
	Title           string  `json:"title"`
	Price           float64 `json:"price"`
	GroupSize       int     `json:"group_size"`
	WeeklyQuota     int64   `json:"weekly_quota"`
	FiveHourQuota   int64   `json:"five_hour_quota"`
	FiveHourEnabled bool    `json:"five_hour_enabled"`
	AllowedModels   string  `json:"allowed_models"`
	AllowedGroup    string  `json:"allowed_group"`
	DurationDays    int     `json:"duration_days"`
}

func buildVirtualMembershipSnapshot(plan *VirtualMembershipPlan, price float64, groupSize int, weekly, fiveHour int64) string {
	data, _ := common.Marshal(virtualMembershipSnapshot{
		PlanId: plan.Id, Code: plan.Code, Title: plan.Title, Price: price,
		GroupSize: groupSize, WeeklyQuota: weekly, FiveHourQuota: fiveHour,
		FiveHourEnabled: plan.FiveHourEnabled, AllowedModels: plan.AllowedModels,
		AllowedGroup: plan.AllowedGroup, DurationDays: plan.DurationDays,
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
	price, weekly, fiveHour, err := virtualMembershipVariant(plan, groupSize)
	if err != nil {
		return nil, err
	}
	if price < 0.01 {
		return nil, errors.New("虚拟会员金额过低")
	}
	now := common.GetTimestamp()
	order := &VirtualMembershipOrder{
		UserId: userId, PlanId: plan.Id, GroupSize: groupSize, Money: price,
		WeeklyQuota: weekly, FiveHourQuota: fiveHour, FiveHourActive: plan.FiveHourEnabled,
		TradeNo: tradeNo, PaymentMethod: paymentMethod, PaymentProvider: paymentProvider,
		Status: status, CreateTime: now, PlanSnapshot: buildVirtualMembershipSnapshot(plan, price, groupSize, weekly, fiveHour),
	}
	if err := tx.Create(order).Error; err != nil {
		return nil, err
	}
	return order, nil
}

func CreateVirtualMembershipEpayOrder(userId, planId, groupSize int, tradeNo, paymentMethod string) (*VirtualMembershipOrder, error) {
	var order *VirtualMembershipOrder
	err := DB.Transaction(func(tx *gorm.DB) error {
		var err error
		order, err = createVirtualMembershipOrderTx(tx, userId, planId, groupSize, tradeNo, paymentMethod, PaymentProviderEpay, common.TopUpStatusPending)
		return err
	})
	return order, err
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
			FiveHourEnabled: order.FiveHourActive, AllowedModels: plan.AllowedModels,
			AllowedGroup: plan.AllowedGroup, DurationDays: plan.DurationDays,
		}
	}
	durationDays := snapshot.DurationDays
	if durationDays <= 0 {
		durationDays = 30
	}
	now := GetDBTimestamp()
	if now <= 0 {
		now = common.GetTimestamp()
	}
	membership := &UserVirtualMembership{
		UserId: order.UserId, PlanId: order.PlanId, OrderId: order.Id,
		PlanTitle: snapshot.Title, PlanCode: snapshot.Code, GroupSize: order.GroupSize,
		WeeklyQuota: order.WeeklyQuota, FiveHourQuota: order.FiveHourQuota,
		FiveHourActive: order.FiveHourActive, WeeklyResetAt: now + 7*86400,
		FiveHourStart: now, FiveHourResetAt: now + 5*3600,
		StartTime: now, EndTime: now + int64(durationDays)*86400,
		Status: VirtualMembershipStatusActive, AllowedModels: snapshot.AllowedModels,
		AllowedGroup: snapshot.AllowedGroup, CreatedAt: now, UpdatedAt: now,
	}
	return tx.Create(membership).Error
}

func CompleteVirtualMembershipOrder(tradeNo, providerPayload, expectedPaymentProvider, actualPaymentMethod string) error {
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
		if order.Status == common.TopUpStatusSuccess || order.Status == VirtualMembershipOrderSuccess {
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
		return tx.Save(&order).Error
	})
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
	if userId <= 0 {
		return nil, nil, errors.New("用户不存在")
	}
	now := common.GetTimestamp()
	var order VirtualMembershipOrder
	var membership UserVirtualMembership
	err := DB.Transaction(func(tx *gorm.DB) error {
		plan, err := getVirtualMembershipPlanTx(tx, planId)
		if err != nil {
			return err
		}
		if !plan.Enabled {
			return errors.New("虚拟会员方案未启用")
		}
		price, weekly, fiveHour, err := virtualMembershipVariant(plan, groupSize)
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
		planSnapshot := buildVirtualMembershipSnapshot(plan, price, groupSize, weekly, fiveHour)
		order = VirtualMembershipOrder{
			UserId: userId, PlanId: plan.Id, GroupSize: groupSize, Money: price,
			WeeklyQuota: weekly, FiveHourQuota: fiveHour, FiveHourActive: plan.FiveHourEnabled,
			TradeNo: tradeNo, PaymentMethod: PaymentMethodBalance, PaymentProvider: PaymentProviderBalance, Status: VirtualMembershipOrderSuccess,
			CreateTime: now, CompleteTime: now, PlanSnapshot: planSnapshot,
		}
		if err := tx.Create(&order).Error; err != nil {
			return err
		}
		end := now + int64(plan.DurationDays)*86400
		membership = UserVirtualMembership{
			UserId: userId, PlanId: plan.Id, OrderId: order.Id, PlanTitle: plan.Title, PlanCode: plan.Code,
			GroupSize: groupSize, WeeklyQuota: weekly, FiveHourQuota: fiveHour, FiveHourActive: plan.FiveHourEnabled,
			WeeklyResetAt: now + 7*86400, FiveHourStart: now, FiveHourResetAt: now + 5*3600,
			StartTime: now, EndTime: end, Status: VirtualMembershipStatusActive,
			AllowedModels: plan.AllowedModels, AllowedGroup: plan.AllowedGroup,
			CreatedAt: now, UpdatedAt: now,
		}
		return tx.Create(&membership).Error
	})
	if err != nil {
		return nil, nil, err
	}
	return &order, &membership, nil
}

func ListUserVirtualMemberships(userId int) ([]*UserVirtualMembership, error) {
	var memberships []*UserVirtualMembership
	err := DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("user_id = ?", userId).Order("id desc").Find(&memberships).Error; err != nil {
			return err
		}
		now := common.GetTimestamp()
		for _, membership := range memberships {
			if err := virtualMembershipResetIfDue(tx, membership, now); err != nil {
				return err
			}
		}
		return nil
	})
	return memberships, err
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
	if membership.AllowedGroup != "" && strings.TrimSpace(membership.AllowedGroup) != strings.TrimSpace(group) {
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
		if membership.Status != VirtualMembershipStatusActive || (membership.EndTime > 0 && membership.EndTime <= now) {
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
		if err := tx.Set("gorm:query_option", "FOR UPDATE").Where("request_id = ?", requestId).First(&record).Error; err != nil {
			return err
		}
		if record.Status != VirtualMembershipRecordPending {
			return nil
		}
		if delta == 0 {
			record.FinalQuota = record.PreConsumed
			record.Status = VirtualMembershipRecordSettled
			record.UpdatedAt = common.GetTimestamp()
			return tx.Save(&record).Error
		}
		var membership UserVirtualMembership
		if err := tx.Set("gorm:query_option", "FOR UPDATE").First(&membership, record.MembershipId).Error; err != nil {
			return err
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
		if err := tx.Set("gorm:query_option", "FOR UPDATE").Where("request_id = ?", requestId).First(&record).Error; err != nil {
			return err
		}
		if record.Status != VirtualMembershipRecordPending {
			return errors.New("虚拟会员预扣记录已结束")
		}
		var membership UserVirtualMembership
		if err := tx.Set("gorm:query_option", "FOR UPDATE").First(&membership, record.MembershipId).Error; err != nil {
			return err
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
		if err := tx.Set("gorm:query_option", "FOR UPDATE").Where("request_id = ?", requestId).First(&record).Error; err != nil {
			return err
		}
		if record.Status != VirtualMembershipRecordPending || record.PreConsumed < delta {
			return errors.New("虚拟会员补充预扣记录无效")
		}
		var membership UserVirtualMembership
		if err := tx.Set("gorm:query_option", "FOR UPDATE").First(&membership, record.MembershipId).Error; err != nil {
			return err
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
		if err := tx.Set("gorm:query_option", "FOR UPDATE").Where("request_id = ?", requestId).First(&record).Error; err != nil {
			return err
		}
		if record.Status != VirtualMembershipRecordPending {
			return nil
		}
		var membership UserVirtualMembership
		if err := tx.Set("gorm:query_option", "FOR UPDATE").First(&membership, record.MembershipId).Error; err != nil {
			return err
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

func ResetAllVirtualMemberships() (int64, error) {
	now := common.GetTimestamp()
	var affected int64
	err := DB.Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&UserVirtualMembership{}).Where("status = ?", VirtualMembershipStatusActive).Updates(map[string]interface{}{
			"weekly_used": 0, "five_hour_used": 0, "weekly_reset_at": now + 7*86400,
			"five_hour_start": now, "five_hour_reset_at": now + 5*3600, "updated_at": now,
		})
		affected = result.RowsAffected
		return result.Error
	})
	return affected, err
}

func EnsureVirtualMembershipMigrations() error {
	if err := EnsureDefaultVirtualMembershipSetting(); err != nil {
		return err
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
