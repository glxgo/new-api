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

func TestDrawUsesLatestActiveRuleForHistoricalCards(t *testing.T) {
	db, user, historicalCard := setupLuckyDrawTest(t, model.LuckyPrizeConfig{
		Code: model.LuckyPrizeGift5, DisplayUsdMicros: 5_000_000, Weight: model.LuckyWeightScale,
	})
	var campaign model.LuckyCampaign
	require.NoError(t, db.First(&campaign, historicalCard.CampaignId).Error)
	poolJSON, err := common.Marshal([]model.LuckyPrizeConfig{{
		Code: model.LuckyPrizeGift10, DisplayUsdMicros: 10_000_000, Weight: model.LuckyWeightScale,
	}})
	require.NoError(t, err)
	currentRule := model.LuckyRuleSet{
		CampaignId: campaign.Id, BaseRuleSetId: historicalCard.RuleSetId,
		Version: 2, Status: "active", SubscriptionPool: string(poolJSON), RechargePool: string(poolJSON),
		ThresholdConfig: "[]", RechargeBonusUsdMicros: 40_000_000,
		RechargeRewardValidSeconds: 30 * 24 * 3600, RechargeCardValidSeconds: 30 * 24 * 3600,
		CrazyCardValidSeconds: 5 * 3600, CrazyCardQuotaUsdMicros: 600_000_000,
		ActivityGroup: "套餐专用分组", Checksum: "current-checksum",
	}
	require.NoError(t, db.Create(&currentRule).Error)
	require.NoError(t, db.Model(&model.LuckyRuleSet{}).Where("id = ?", historicalCard.RuleSetId).
		Update("status", "retired").Error)
	require.NoError(t, db.Model(&campaign).Update("active_rule_set_id", currentRule.Id).Error)

	currentCard := historicalCard
	currentCard.Id = 0
	currentCard.RuleSetId = currentRule.Id
	currentCard.GrantKey = fmt.Sprintf("current-rule:%d", time.Now().UnixNano())
	require.NoError(t, db.Create(&currentCard).Error)

	historicalDraw, err := drawLuckyCardWithSource(user.Id, historicalCard.Id, "historical-rule", fixedLuckyRandom(0))
	require.NoError(t, err)
	require.Equal(t, model.LuckyPrizeGift10, historicalDraw.PrizeType)
	require.Equal(t, currentRule.Id, historicalDraw.RuleSetId)
	currentDraw, err := drawLuckyCardWithSource(user.Id, currentCard.Id, "current-rule", fixedLuckyRandom(0))
	require.NoError(t, err)
	require.Equal(t, model.LuckyPrizeGift10, currentDraw.PrizeType)
	require.Equal(t, currentRule.Id, currentDraw.RuleSetId)
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

func TestRechargeQuotaDrawExpiresAtMidnightAndMergesSameExpiry(t *testing.T) {
	db, user, firstCard := setupLuckyDrawTest(t, model.LuckyPrizeConfig{
		Code: model.LuckyPrizeQuota5, DisplayUsdMicros: 5_000_000, Weight: model.LuckyWeightScale,
	})
	firstDraw, err := drawLuckyCardWithSource(user.Id, firstCard.Id, "merge-first", fixedLuckyRandom(0))
	require.NoError(t, err)

	secondCard := firstCard
	secondCard.Id = 0
	secondCard.GrantKey = fmt.Sprintf("merge-second:%d", time.Now().UnixNano())
	secondCard.Status = model.LuckyCardAvailable
	secondCard.ConsumedAt = 0
	require.NoError(t, db.Create(&secondCard).Error)
	secondDraw, err := drawLuckyCardWithSource(user.Id, secondCard.Id, "merge-second", fixedLuckyRandom(0))
	require.NoError(t, err)
	require.Equal(t, firstDraw.RewardSubscriptionId, secondDraw.RewardSubscriptionId)

	var rewards []model.UserSubscription
	require.NoError(t, db.Where("user_id = ? AND source = ?", user.Id, "lucky_quota").Find(&rewards).Error)
	require.Len(t, rewards, 1)
	reward := rewards[0]
	require.EqualValues(t, 2*quotaFromUsdMicros(45_000_000), reward.AmountTotal)
	require.Equal(t, "幸运大转盘 · $90 套餐额度", reward.PlanTitle)

	expiresAt := time.Unix(reward.EndTime, 0).In(time.Local)
	require.Zero(t, expiresAt.Hour())
	require.Zero(t, expiresAt.Minute())
	require.Zero(t, expiresAt.Second())
	remaining := reward.EndTime - firstDraw.AwardedAt
	require.GreaterOrEqual(t, remaining, int64(29*24*3600))
	require.LessOrEqual(t, remaining, int64(30*24*3600))

	snapshot, err := model.ParseSubscriptionPlanSnapshot(reward.PlanSnapshot)
	require.NoError(t, err)
	require.EqualValues(t, reward.AmountTotal, snapshot.TotalAmount)
	require.Equal(t, reward.PlanTitle, snapshot.Title)
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
