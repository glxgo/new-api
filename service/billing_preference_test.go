package service

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// ---------------------------------------------------------------------------
// "仅用订阅"(subscription_only) 的分组边界:
// 只有订阅分组(套餐指定分组、用户已绑定订阅的分组、无分组限制的通用订阅)
// 才抑制钱包计费; 普通分组没有订阅可用, 仍按余额计费。
// ---------------------------------------------------------------------------

const (
	billingPrefTestUserId   = 388
	billingPrefTestQuota    = 50_000_000
	billingPrefTestConsume  = 1_000
	billingPrefPlanGroup    = "套餐专用分组"
	billingPrefBoundGroup   = "gpt pro"
	billingPrefPlainGroup   = "default"
	billingPrefOtherPlanGrp = "gpt套餐专用分组"
)

func setupBillingPreferenceTest(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(
		sqlite.Open(fmt.Sprintf("file:billing-pref-%d?mode=memory&cache=shared", time.Now().UnixNano())),
		&gorm.Config{},
	)
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, db.AutoMigrate(
		&model.User{},
		&model.Token{},
		&model.UserSubscription{},
		&model.SubscriptionPlan{},
		&model.VirtualMembershipPlan{},
		&model.SubscriptionPreConsumeRecord{},
	))

	previousDB := model.DB
	previousSQLite := common.UsingSQLite
	previousRedis := common.RedisEnabled
	model.DB = db
	common.UsingSQLite = true
	common.RedisEnabled = false
	t.Cleanup(func() {
		model.DB = previousDB
		common.UsingSQLite = previousSQLite
		common.RedisEnabled = previousRedis
	})
	return db
}

func seedBillingPreferenceUser(t *testing.T, db *gorm.DB) {
	t.Helper()
	user := &model.User{
		Id:       billingPrefTestUserId,
		Username: "billing-pref-user",
		Quota:    billingPrefTestQuota,
		Status:   common.UserStatusEnabled,
	}
	user.SetSetting(dto.UserSetting{BillingPreference: "subscription_only"})
	require.NoError(t, db.Create(user).Error)
}

func seedBillingPreferencePlan(t *testing.T, db *gorm.DB, id int, title string, allowedGroup string) {
	t.Helper()
	require.NoError(t, db.Create(&model.SubscriptionPlan{
		Id:           id,
		Title:        title,
		Enabled:      true,
		AllowedGroup: allowedGroup,
	}).Error)
}

func seedBillingPreferenceSubscription(t *testing.T, db *gorm.DB, id int, planId int, allowedGroup string) {
	t.Helper()
	now := time.Now().Unix()
	require.NoError(t, db.Create(&model.UserSubscription{
		Id:           id,
		UserId:       billingPrefTestUserId,
		PlanId:       planId,
		PlanTitle:    "套餐月卡",
		AmountTotal:  10_000_000,
		Status:       "active",
		StartTime:    now - 3600,
		EndTime:      now + 24*3600,
		AllowedGroup: allowedGroup,
	}).Error)
}

// newBillingSessionForGroup 用最接近真实 relay 的输入构造计费会话。
// IsPlayground 置真以避免测试依赖 token 表, ForcePreConsume 置真以关闭信任旁路,
// 保证预扣费路径真实执行。
func newBillingSessionForGroup(t *testing.T, group string, pref string) (*BillingSession, *types.NewAPIError) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	info := &relaycommon.RelayInfo{
		UserId:          billingPrefTestUserId,
		UsingGroup:      group,
		OriginModelName: "gpt-6-astra",
		RequestId:       fmt.Sprintf("billing-pref-%d", time.Now().UnixNano()),
		IsPlayground:    true,
		ForcePreConsume: true,
		UserSetting:     dto.UserSetting{BillingPreference: pref},
	}
	return NewBillingSession(c, info, billingPrefTestConsume)
}

func billingPreferenceUserQuota(t *testing.T, id int) int {
	t.Helper()
	var user model.User
	require.NoError(t, model.DB.First(&user, id).Error)
	return user.Quota
}

func billingPreferenceSubscriptionUsed(t *testing.T, id int) int64 {
	t.Helper()
	var sub model.UserSubscription
	require.NoError(t, model.DB.First(&sub, id).Error)
	return sub.AmountUsed
}

// 核心回归: 用户勾选"仅用订阅", 但当前分组是普通分组且他的有效订阅限定在别的
// 套餐分组, 请求应按余额计费而不是 403。
func TestSubscriptionOnlyUsesWalletOutsideSubscriptionGroups(t *testing.T) {
	db := setupBillingPreferenceTest(t)
	seedBillingPreferenceUser(t, db)
	seedBillingPreferencePlan(t, db, 1, "套餐月卡", billingPrefPlanGroup)
	seedBillingPreferenceSubscription(t, db, 287, 1, billingPrefPlanGroup)

	session, apiErr := newBillingSessionForGroup(t, billingPrefBoundGroup, "subscription_only")
	require.Nil(t, apiErr)
	require.Equal(t, BillingSourceWallet, session.funding.Source())
	require.Equal(t, billingPrefTestQuota-billingPrefTestConsume, billingPreferenceUserQuota(t, billingPrefTestUserId))
	require.EqualValues(t, 0, billingPreferenceSubscriptionUsed(t, 287))
}

// 套餐专属分组仍然是订阅专属入口: 即使没有匹配订阅也不能改扣钱包。
func TestSubscriptionOnlyKeepsPlanGroupOnSubscription(t *testing.T) {
	db := setupBillingPreferenceTest(t)
	seedBillingPreferenceUser(t, db)
	seedBillingPreferencePlan(t, db, 1, "套餐月卡", billingPrefPlanGroup)
	seedBillingPreferenceSubscription(t, db, 287, 1, billingPrefPlanGroup)

	session, apiErr := newBillingSessionForGroup(t, billingPrefPlanGroup, "subscription_only")
	require.Nil(t, apiErr)
	require.Equal(t, BillingSourceSubscription, session.funding.Source())
	require.Equal(t, billingPrefTestQuota, billingPreferenceUserQuota(t, billingPrefTestUserId), "订阅计费不应扣钱包")
	require.EqualValues(t, billingPrefTestConsume, billingPreferenceSubscriptionUsed(t, 287))
}

// 套餐分组但没有可用订阅时仍按订阅硬边界失败, 不允许回退余额。
func TestSubscriptionOnlyDoesNotFallBackToWalletInPlanGroup(t *testing.T) {
	db := setupBillingPreferenceTest(t)
	seedBillingPreferenceUser(t, db)
	seedBillingPreferencePlan(t, db, 3, "企业套餐", billingPrefOtherPlanGrp)

	_, apiErr := newBillingSessionForGroup(t, billingPrefOtherPlanGrp, "subscription_only")
	require.NotNil(t, apiErr)
	require.Equal(t, types.ErrorCodeInsufficientUserQuota, apiErr.GetErrorCode())
	require.Equal(t, billingPrefTestQuota, billingPreferenceUserQuota(t, billingPrefTestUserId), "套餐分组失败不得扣钱包")
}

// 用户绑定订阅的分组(非套餐指定分组)同样属于订阅分组, 继续按订阅计费。
func TestSubscriptionOnlyUsesBoundGroupSubscription(t *testing.T) {
	db := setupBillingPreferenceTest(t)
	seedBillingPreferenceUser(t, db)
	seedBillingPreferencePlan(t, db, 2, "通用月卡", "")
	seedBillingPreferenceSubscription(t, db, 300, 2, billingPrefBoundGroup)

	session, apiErr := newBillingSessionForGroup(t, billingPrefBoundGroup, "subscription_only")
	require.Nil(t, apiErr)
	require.Equal(t, BillingSourceSubscription, session.funding.Source())
	require.EqualValues(t, billingPrefTestConsume, billingPreferenceSubscriptionUsed(t, 300))
}

// 无分组限制的通用订阅在所有分组都可用, "仅用订阅"对它继续生效。
func TestSubscriptionOnlyKeepsUnrestrictedSubscription(t *testing.T) {
	db := setupBillingPreferenceTest(t)
	seedBillingPreferenceUser(t, db)
	seedBillingPreferencePlan(t, db, 2, "通用月卡", "")
	seedBillingPreferenceSubscription(t, db, 301, 2, "")

	session, apiErr := newBillingSessionForGroup(t, billingPrefPlainGroup, "subscription_only")
	require.Nil(t, apiErr)
	require.Equal(t, BillingSourceSubscription, session.funding.Source())
	require.EqualValues(t, billingPrefTestConsume, billingPreferenceSubscriptionUsed(t, 301))
}

// 完全没有订阅时, 普通分组的"仅用订阅"退化为余额计费。
func TestSubscriptionOnlyUsesWalletWithoutAnySubscription(t *testing.T) {
	db := setupBillingPreferenceTest(t)
	seedBillingPreferenceUser(t, db)

	session, apiErr := newBillingSessionForGroup(t, billingPrefPlainGroup, "subscription_only")
	require.Nil(t, apiErr)
	require.Equal(t, BillingSourceWallet, session.funding.Source())
	require.Equal(t, billingPrefTestQuota-billingPrefTestConsume, billingPreferenceUserQuota(t, billingPrefTestUserId))
}
