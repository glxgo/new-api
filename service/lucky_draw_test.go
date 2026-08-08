package service

import (
	"fmt"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type fixedLuckyRandom int64

func (r fixedLuckyRandom) Intn(max int64) (int64, error) {
	return int64(r), nil
}

func setupLuckyDrawTest(t *testing.T, prize model.LuckyPrizeConfig) (*gorm.DB, model.User, model.LuckyCard) {
	t.Helper()
	db, err := gorm.Open(
		sqlite.Open(fmt.Sprintf("file:lucky-draw-%d?mode=memory&cache=shared", time.Now().UnixNano())),
		&gorm.Config{},
	)
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&model.User{}, &model.UserSubscription{}, &model.Token{},
		&model.TokenSubscriptionBindingHistory{}, &model.LuckyCampaign{},
		&model.LuckyRuleSet{}, &model.LuckyCard{}, &model.LuckyDraw{},
		&model.LuckyRewardBucket{}, &model.LuckyPausePeriod{},
	))
	oldDB := model.DB
	oldSQLite := common.UsingSQLite
	oldRedis := common.RedisEnabled
	model.DB = db
	common.UsingSQLite = true
	common.RedisEnabled = false
	t.Cleanup(func() {
		model.DB = oldDB
		common.UsingSQLite = oldSQLite
		common.RedisEnabled = oldRedis
	})

	user := model.User{Username: "lucky-draw-user", Password: "hashed", AffCode: "ldraw"}
	require.NoError(t, db.Create(&user).Error)
	campaign := model.LuckyCampaign{
		Code: model.LuckyCampaignCode, Name: "幸运大转盘",
	}
	require.NoError(t, db.Create(&campaign).Error)
	require.NoError(t, db.Model(&campaign).Updates(map[string]interface{}{
		"issuance_paused": false,
		"draw_paused":     false,
	}).Error)
	poolJSON, err := common.Marshal([]model.LuckyPrizeConfig{prize})
	require.NoError(t, err)
	rule := model.LuckyRuleSet{
		CampaignId: campaign.Id, Version: 1, Status: "active",
		SubscriptionPool: string(poolJSON), RechargePool: string(poolJSON),
		ThresholdConfig: "[]", RechargeBonusUsdMicros: 40_000_000,
		RechargeRewardValidSeconds: 30 * 24 * 3600,
		RechargeCardValidSeconds:   30 * 24 * 3600,
		CrazyCardValidSeconds:      5 * 3600, CrazyCardQuotaUsdMicros: 600_000_000,
		ActivityGroup: "套餐专用分组", Checksum: "test-checksum",
	}
	require.NoError(t, db.Create(&rule).Error)
	require.NoError(t, db.Model(&campaign).Update("active_rule_set_id", rule.Id).Error)
	card := model.LuckyCard{
		UserId: user.Id, CampaignId: campaign.Id, RuleSetId: rule.Id,
		PoolType: model.LuckyPoolRecharge, SourceType: "recharge_threshold",
		SourceRef: "test", GrantKey: fmt.Sprintf("test:%d", time.Now().UnixNano()),
		Status: model.LuckyCardAvailable, ExpiresAt: common.GetTimestamp() + 3600,
	}
	require.NoError(t, db.Create(&card).Error)
	return db, user, card
}

func TestSelectLuckyPrizeUsesLeftClosedRightOpenIntervals(t *testing.T) {
	pool := []model.LuckyPrizeConfig{
		{Code: "first", Weight: 340_000},
		{Code: "second", Weight: 660_000},
	}
	first, err := SelectLuckyPrize(pool, 339_999)
	require.NoError(t, err)
	require.Equal(t, "first", first.Code)
	second, err := SelectLuckyPrize(pool, 340_000)
	require.NoError(t, err)
	require.Equal(t, "second", second.Code)
}

func TestRechargeGiftDrawIsAtomicAndIdempotent(t *testing.T) {
	db, user, card := setupLuckyDrawTest(t, model.LuckyPrizeConfig{
		Code: model.LuckyPrizeGift5, DisplayUsdMicros: 5_000_000, Weight: model.LuckyWeightScale,
	})
	draw, err := drawLuckyCardWithSource(user.Id, card.Id, "same-request", fixedLuckyRandom(0))
	require.NoError(t, err)
	require.Equal(t, model.LuckyPrizeGift5, draw.PrizeType)
	require.EqualValues(t, quotaFromUsdMicros(5_000_000), draw.GiftQuotaAwarded)

	again, err := drawLuckyCardWithSource(user.Id, card.Id, "same-request", fixedLuckyRandom(999_999))
	require.NoError(t, err)
	require.Equal(t, draw.Id, again.Id)
	var stored model.User
	require.NoError(t, db.First(&stored, user.Id).Error)
	require.EqualValues(t, quotaFromUsdMicros(5_000_000), stored.GiftQuota)
}

func TestRechargeFiveDollarQuotaDrawAddsFortyDollars(t *testing.T) {
	db, user, card := setupLuckyDrawTest(t, model.LuckyPrizeConfig{
		Code: model.LuckyPrizeQuota5, DisplayUsdMicros: 5_000_000, Weight: model.LuckyWeightScale,
	})
	draw, err := drawLuckyCardWithSource(user.Id, card.Id, "quota-request", fixedLuckyRandom(0))
	require.NoError(t, err)
	require.EqualValues(t, 45_000_000, draw.ActualUsdMicros)
	require.NotZero(t, draw.RewardSubscriptionId)
	var reward model.UserSubscription
	require.NoError(t, db.First(&reward, draw.RewardSubscriptionId).Error)
	require.EqualValues(t, quotaFromUsdMicros(45_000_000), reward.AmountTotal)
	require.True(t, reward.LuckyCardDisabled)
	require.Equal(t, "never", model.NormalizeResetPeriod(""))
}

func TestResumeLuckyDrawExtendsCardsCreatedBeforeAndDuringPause(t *testing.T) {
	db, _, card := setupLuckyDrawTest(t, model.LuckyPrizeConfig{
		Code: model.LuckyPrizeGift5, DisplayUsdMicros: 5_000_000, Weight: model.LuckyWeightScale,
	})
	now := model.GetDBTimestamp()
	startedAt := now - 100
	require.NoError(t, db.Model(&model.LuckyCampaign{}).
		Where("id = ?", card.CampaignId).
		Updates(map[string]interface{}{
			"draw_paused":           true,
			"draw_pause_started_at": startedAt,
		}).Error)
	require.NoError(t, db.Create(&model.LuckyPausePeriod{
		CampaignId: card.CampaignId, PauseType: "draw", StartedAt: startedAt,
		Status: "active", CreatedAt: startedAt, UpdatedAt: startedAt,
	}).Error)

	beforeExpiry := now + 1_000
	require.NoError(t, db.Model(&card).Updates(map[string]interface{}{
		"issued_at":                 startedAt - 20,
		"expires_at":                beforeExpiry,
		"source_effective_end_time": beforeExpiry,
	}).Error)
	during := model.LuckyCard{
		UserId: card.UserId, CampaignId: card.CampaignId, RuleSetId: card.RuleSetId,
		PoolType: model.LuckyPoolRecharge, SourceType: "recharge_threshold",
		SourceRef: "during-pause", GrantKey: "during-pause",
		Status: model.LuckyCardAvailable, IssuedAt: now - 40,
		ExpiresAt: beforeExpiry, SourceEffectiveEndTime: beforeExpiry,
	}
	require.NoError(t, db.Create(&during).Error)

	require.NoError(t, ResumeLuckyDraw(1, "maintenance completed"))
	var storedBefore, storedDuring model.LuckyCard
	require.NoError(t, db.First(&storedBefore, card.Id).Error)
	require.NoError(t, db.First(&storedDuring, during.Id).Error)
	require.InDelta(t, beforeExpiry+100, storedBefore.ExpiresAt, 2)
	require.InDelta(t, beforeExpiry+40, storedDuring.ExpiresAt, 2)
	require.InDelta(t, int64(100), storedBefore.PauseExtensionSeconds, 2)
	require.InDelta(t, int64(40), storedDuring.PauseExtensionSeconds, 2)
}
