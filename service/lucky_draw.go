package service

import (
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const luckyRechargeRewardDefaultValidSeconds int64 = 30 * 24 * 3600

var (
	ErrLuckyDrawPaused      = errors.New("幸运大转盘抽奖暂时关闭")
	ErrLuckyCardUnavailable = errors.New("幸运卡不可用或已过期")
)

type LuckyRandomSource interface {
	Intn(max int64) (int64, error)
}

type cryptoLuckyRandom struct{}

func (cryptoLuckyRandom) Intn(max int64) (int64, error) {
	if max <= 0 {
		return 0, errors.New("invalid random upper bound")
	}
	value, err := rand.Int(rand.Reader, big.NewInt(max))
	if err != nil {
		return 0, err
	}
	return value.Int64(), nil
}

func SelectLuckyPrize(pool []model.LuckyPrizeConfig, value int64) (model.LuckyPrizeConfig, error) {
	if value < 0 || value >= model.LuckyWeightScale {
		return model.LuckyPrizeConfig{}, errors.New("random value out of range")
	}
	var cursor int64
	for _, prize := range pool {
		cursor += prize.Weight
		if value < cursor {
			return prize, nil
		}
	}
	return model.LuckyPrizeConfig{}, errors.New("lucky prize pool is incomplete")
}

func quotaFromUsdMicros(usdMicros int64) int64 {
	return decimal.NewFromInt(usdMicros).
		Div(decimal.NewFromInt(1_000_000)).
		Mul(decimal.NewFromFloat(common.QuotaPerUnit)).
		Round(0).
		IntPart()
}

func usdMicrosFromQuota(quota int64) int64 {
	if quota <= 0 || common.QuotaPerUnit <= 0 {
		return 0
	}
	return decimal.NewFromInt(quota).
		Div(decimal.NewFromFloat(common.QuotaPerUnit)).
		Mul(decimal.NewFromInt(1_000_000)).
		Round(0).
		IntPart()
}

func luckyQuotaPlanTitle(usdMicros int64) string {
	amount := decimal.NewFromInt(usdMicros).Div(decimal.NewFromInt(1_000_000)).String()
	return fmt.Sprintf("幸运大转盘 · $%s 套餐额度", amount)
}

// luckyRechargeRewardEndTime aligns wallet-card subscription prizes to local
// midnight. The default 30-day term remains visible as 30 calendar days while
// every draw made on the same local day shares one mergeable expiry bucket.
func luckyRechargeRewardEndTime(now, validSeconds int64) int64 {
	if validSeconds <= 0 {
		validSeconds = luckyRechargeRewardDefaultValidSeconds
	}
	calendarDays := int((validSeconds + 24*3600 - 1) / (24 * 3600))
	localNow := time.Unix(now, 0).In(time.Local)
	localMidnight := time.Date(
		localNow.Year(), localNow.Month(), localNow.Day(),
		0, 0, 0, 0, time.Local,
	)
	return localMidnight.AddDate(0, 0, calendarDays).Unix()
}

func buildLuckyPlanSnapshot(base *model.SubscriptionPlan, title string, total, cap int64, group string, durationSeconds int64, resetPeriod string) (string, error) {
	plan := model.SubscriptionPlan{Id: 1}
	if base != nil {
		plan = *base
		if plan.Id <= 0 {
			plan.Id = 1
		}
	}
	plan.Title = title
	plan.PriceAmount = 0
	plan.Enabled = false
	plan.TotalAmount = total
	plan.AmountCap = cap
	plan.DurationUnit = model.SubscriptionDurationCustom
	plan.DurationValue = 1
	plan.CustomSeconds = durationSeconds
	plan.QuotaResetPeriod = resetPeriod
	plan.QuotaResetCustomSeconds = 0
	plan.UpgradeGroup = ""
	plan.AllowedGroup = strings.TrimSpace(group)
	plan.LuckyCardGrantCount = 0
	plan.LuckyCardOnReset = false
	return model.BuildSubscriptionPlanSnapshot(&plan)
}

func createLuckySubscriptionTx(
	tx *gorm.DB,
	userId int,
	planId int,
	title string,
	planSnapshot string,
	amountTotal int64,
	amountCap int64,
	startTime int64,
	endTime int64,
	allowedGroup string,
	source string,
	drawId int64,
	sourceSubscriptionId int,
	renewedFromId *int,
) (*model.UserSubscription, error) {
	if tx == nil || userId <= 0 || endTime <= startTime || amountTotal < 0 {
		return nil, errors.New("invalid lucky subscription")
	}
	sub := &model.UserSubscription{
		UserId: userId, PlanId: planId, PlanSnapshot: planSnapshot, PlanTitle: title,
		RenewedFromId: renewedFromId, AmountTotal: amountTotal, AmountCap: amountCap,
		StartTime: startTime, EndTime: endTime, Status: "active", Source: source,
		AllowedGroup: strings.TrimSpace(allowedGroup), PaidRevenueQuota: 0,
		DividendState:     model.SubscriptionDividendSkippedSource,
		LuckyCardDisabled: true, PromotionOriginDrawId: drawId,
		PromotionSourceSubscriptionId: sourceSubscriptionId,
	}
	if err := tx.Create(sub).Error; err != nil {
		return nil, err
	}
	return sub, nil
}

func awardQuotaPrizeTx(tx *gorm.DB, card *model.LuckyCard, rule *model.LuckyRuleSet, draw *model.LuckyDraw, prize model.LuckyPrizeConfig, now int64) error {
	actualUsd := prize.DisplayUsdMicros
	if card.PoolType == model.LuckyPoolRecharge {
		actualUsd += rule.RechargeBonusUsdMicros
	}
	quota := quotaFromUsdMicros(actualUsd)
	if quota <= 0 {
		return errors.New("invalid lucky quota award")
	}
	draw.ActualUsdMicros = actualUsd
	draw.AwardedQuota = quota

	if card.PoolType == model.LuckyPoolRecharge {
		end := luckyRechargeRewardEndTime(now, rule.RechargeRewardValidSeconds)
		var bucket model.LuckyRewardBucket
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("user_id = ? AND source_subscription_id = 0 AND effective_end_time = ?", card.UserId, end).
			First(&bucket).Error
		if err == nil {
			var reward model.UserSubscription
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
				Where("id = ? AND user_id = ? AND source = ? AND end_time = ?", bucket.RewardSubscriptionId, card.UserId, "lucky_quota", end).
				First(&reward).Error; err != nil {
				return err
			}
			newTotal := reward.AmountTotal + quota
			title := luckyQuotaPlanTitle(usdMicrosFromQuota(newTotal))
			plan, parseErr := model.ParseSubscriptionPlanSnapshot(reward.PlanSnapshot)
			if parseErr != nil {
				plan = nil
			}
			snapshot, snapshotErr := buildLuckyPlanSnapshot(
				plan, title, newTotal, 0, rule.ActivityGroup, end-reward.StartTime, model.SubscriptionResetNever,
			)
			if snapshotErr != nil {
				return snapshotErr
			}
			if updateErr := tx.Model(&model.UserSubscription{}).
				Where("id = ? AND user_id = ?", reward.Id, card.UserId).
				Updates(map[string]interface{}{
					"amount_total":  newTotal,
					"plan_title":    title,
					"plan_snapshot": snapshot,
				}).Error; updateErr != nil {
				return updateErr
			}
			draw.RewardSubscriptionId = reward.Id
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		title := luckyQuotaPlanTitle(actualUsd)
		snapshot, err := buildLuckyPlanSnapshot(
			nil, title, quota, 0, rule.ActivityGroup, end-now, model.SubscriptionResetNever,
		)
		if err != nil {
			return err
		}
		sub, err := createLuckySubscriptionTx(
			tx, card.UserId, 0, title, snapshot, quota, 0, now, end,
			rule.ActivityGroup, "lucky_quota", draw.Id, 0, nil,
		)
		if err != nil {
			return err
		}
		bucket = model.LuckyRewardBucket{
			UserId: card.UserId, SourceSubscriptionId: 0,
			EffectiveEndTime: end, RewardSubscriptionId: sub.Id,
			CreatedAt: now, UpdatedAt: now,
		}
		if err := tx.Create(&bucket).Error; err != nil {
			return err
		}
		draw.RewardSubscriptionId = sub.Id
		return nil
	}

	var source model.UserSubscription
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("id = ? AND user_id = ?", card.SourceSubscriptionId, card.UserId).
		First(&source).Error; err != nil {
		return err
	}
	end := card.SourceEffectiveEndTime
	if end <= now {
		return ErrLuckyCardUnavailable
	}
	var bucket model.LuckyRewardBucket
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("user_id = ? AND source_subscription_id = ? AND effective_end_time = ?",
			card.UserId, source.Id, end).
		First(&bucket).Error
	if err == nil {
		if updateErr := tx.Model(&model.UserSubscription{}).
			Where("id = ? AND user_id = ?", bucket.RewardSubscriptionId, card.UserId).
			Update("amount_total", gorm.Expr("amount_total + ?", quota)).Error; updateErr != nil {
			return updateErr
		}
		draw.RewardSubscriptionId = bucket.RewardSubscriptionId
		return nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	basePlan, err := model.ParseSubscriptionPlanSnapshot(source.PlanSnapshot)
	if err != nil {
		basePlan = nil
	}
	title := fmt.Sprintf("%s · 幸运额度", source.PlanTitle)
	snapshot, err := buildLuckyPlanSnapshot(
		basePlan, title, quota, 0, source.AllowedGroup, end-now, model.SubscriptionResetNever,
	)
	if err != nil {
		return err
	}
	sub, err := createLuckySubscriptionTx(
		tx, card.UserId, source.PlanId, title,
		snapshot, quota, 0, now, end, source.AllowedGroup,
		"lucky_quota", draw.Id, source.Id, nil,
	)
	if err != nil {
		return err
	}
	bucket = model.LuckyRewardBucket{
		UserId: card.UserId, SourceSubscriptionId: source.Id,
		EffectiveEndTime: end, RewardSubscriptionId: sub.Id,
		CreatedAt: now, UpdatedAt: now,
	}
	if err := tx.Create(&bucket).Error; err != nil {
		return err
	}
	draw.RewardSubscriptionId = sub.Id
	return nil
}

func awardGiftPrizeTx(tx *gorm.DB, card *model.LuckyCard, draw *model.LuckyDraw, prize model.LuckyPrizeConfig) error {
	quota := quotaFromUsdMicros(prize.DisplayUsdMicros)
	if quota <= 0 {
		return errors.New("invalid lucky gift award")
	}
	result := tx.Model(&model.User{}).Where("id = ?", card.UserId).
		Update("gift_quota", gorm.Expr("gift_quota + ?", quota))
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return errors.New("lucky draw user not found")
	}
	draw.ActualUsdMicros = prize.DisplayUsdMicros
	draw.AwardedQuota = quota
	draw.GiftQuotaAwarded = quota
	return nil
}

func awardDoublePrizeTx(tx *gorm.DB, card *model.LuckyCard, draw *model.LuckyDraw, now int64) error {
	var source model.UserSubscription
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("id = ? AND user_id = ?", card.SourceSubscriptionId, card.UserId).
		First(&source).Error; err != nil {
		return err
	}
	if card.SourceEffectiveEndTime <= now {
		return ErrLuckyCardUnavailable
	}
	sub, err := createLuckySubscriptionTx(
		tx, source.UserId, source.PlanId, source.PlanTitle+" · 双倍奖励", source.PlanSnapshot,
		source.AmountTotal, source.AmountCap, now, card.SourceEffectiveEndTime,
		source.AllowedGroup, "lucky_double", draw.Id, source.Id, nil,
	)
	if err != nil {
		return err
	}
	sub.LastResetTime = source.LastResetTime
	sub.NextResetTime = source.NextResetTime
	if sub.NextResetTime >= sub.EndTime {
		sub.NextResetTime = 0
	}
	if err := tx.Save(sub).Error; err != nil {
		return err
	}
	draw.RewardSubscriptionId = sub.Id
	return nil
}

func shiftRenewalChainTx(tx *gorm.DB, first *model.UserSubscription, shift int64) error {
	current := first
	for hop := 0; current != nil && hop < 64; hop++ {
		current.StartTime += shift
		current.EndTime += shift
		if current.LastResetTime > 0 {
			current.LastResetTime += shift
		}
		if current.NextResetTime > 0 {
			current.NextResetTime += shift
		}
		if err := tx.Save(current).Error; err != nil {
			return err
		}
		if err := tx.Model(&model.Token{}).
			Where("planned_subscription_id = ?", current.Id).
			Update("planned_subscription_effective", current.StartTime).Error; err != nil {
			return err
		}
		var next model.UserSubscription
		result := tx.Where("renewed_from_id = ?", current.Id).Limit(1).Find(&next)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			break
		}
		current = &next
	}
	return nil
}

func awardFullResetPrizeTx(tx *gorm.DB, card *model.LuckyCard, draw *model.LuckyDraw, now int64) error {
	var source model.UserSubscription
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("id = ? AND user_id = ?", card.SourceSubscriptionId, card.UserId).
		First(&source).Error; err != nil {
		return err
	}
	if source.Status == "cancelled" || source.Status == "superseded" {
		return ErrLuckyCardUnavailable
	}
	plan, err := model.ParseSubscriptionPlanSnapshot(source.PlanSnapshot)
	if err != nil {
		plan, err = model.GetSubscriptionPlanById(source.PlanId)
		if err != nil {
			return err
		}
	}
	var paidSuccessor model.UserSubscription
	successorResult := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("renewed_from_id = ?", source.Id).Limit(1).Find(&paidSuccessor)
	if successorResult.Error != nil {
		return successorResult.Error
	}
	if successorResult.RowsAffected > 0 {
		if err := tx.Model(&paidSuccessor).Update("renewed_from_id", nil).Error; err != nil {
			return err
		}
	}
	cleanPlan := *plan
	cleanPlan.UpgradeGroup = ""
	successor, err := model.CreateUserSubscriptionFromPlanWithOptionsTx(
		tx, source.UserId, &cleanPlan, "lucky_full_reset",
		model.CreateUserSubscriptionOptions{
			StartTime: now, RenewedFromId: &source.Id, PlanSnapshot: source.PlanSnapshot,
			SkipPurchaseLimit: true,
		},
	)
	if err != nil {
		return err
	}
	successor.LuckyCardDisabled = true
	successor.PromotionOriginDrawId = draw.Id
	successor.PromotionSourceSubscriptionId = source.Id
	successor.PaidRevenueQuota = 0
	successor.DividendState = model.SubscriptionDividendSkippedSource
	if err := tx.Save(successor).Error; err != nil {
		return err
	}
	if successorResult.RowsAffected > 0 {
		if err := tx.Model(&paidSuccessor).Update("renewed_from_id", successor.Id).Error; err != nil {
			return err
		}
		if err := shiftRenewalChainTx(tx, &paidSuccessor, successor.EndTime-successor.StartTime); err != nil {
			return err
		}
	}
	source.Status = "superseded"
	source.SupersededById = &successor.Id
	if err := tx.Save(&source).Error; err != nil {
		return err
	}
	var tokens []model.Token
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("user_id = ? AND subscription_id = ?", source.UserId, source.Id).
		Find(&tokens).Error; err != nil {
		return err
	}
	for i := range tokens {
		beforeId := tokens[i].SubscriptionId
		tokens[i].SubscriptionId = successor.Id
		tokens[i].SubscriptionWalletCycleId = successor.Id
		if tokens[i].PlannedSubscriptionId > 0 {
			tokens[i].PlannedSubscriptionEffective = successor.EndTime
		}
		if err := tx.Save(&tokens[i]).Error; err != nil {
			return err
		}
		history := model.TokenSubscriptionBindingHistory{
			UserId: source.UserId, TokenId: tokens[i].Id, ActorType: "system",
			Action: model.TokenSubscriptionActionRebind, FromSubscriptionId: beforeId,
			ToSubscriptionId: successor.Id, FromGroup: tokens[i].Group, ToGroup: tokens[i].Group,
			Reason: "lucky_full_reset", CreatedAt: now,
		}
		if err := tx.Create(&history).Error; err != nil {
			return err
		}
	}
	draw.RewardSubscriptionId = successor.Id
	return nil
}

func awardCrazyPrizeTx(tx *gorm.DB, card *model.LuckyCard, rule *model.LuckyRuleSet, draw *model.LuckyDraw, now int64) error {
	quota := quotaFromUsdMicros(rule.CrazyCardQuotaUsdMicros)
	snapshot, err := buildLuckyPlanSnapshot(
		nil, "5 小时狂蹬卡", quota, 0, rule.ActivityGroup,
		rule.CrazyCardValidSeconds, model.SubscriptionResetNever,
	)
	if err != nil {
		return err
	}
	sub, err := createLuckySubscriptionTx(
		tx, card.UserId, 0, "5 小时狂蹬卡", snapshot, quota, 0, now,
		now+rule.CrazyCardValidSeconds, rule.ActivityGroup, "lucky_crazy_5h",
		draw.Id, card.SourceSubscriptionId, nil,
	)
	if err != nil {
		return err
	}
	draw.ActualUsdMicros = rule.CrazyCardQuotaUsdMicros
	draw.AwardedQuota = quota
	draw.RewardSubscriptionId = sub.Id
	return nil
}

func drawLuckyCardWithSource(userId int, cardId int64, idempotencyKey string, randomSource LuckyRandomSource) (*model.LuckyDraw, error) {
	if userId <= 0 || cardId <= 0 || strings.TrimSpace(idempotencyKey) == "" || len(idempotencyKey) > 64 {
		return nil, errors.New("invalid lucky draw request")
	}
	if randomSource == nil {
		randomSource = cryptoLuckyRandom{}
	}
	var result model.LuckyDraw
	err := model.DB.Transaction(func(tx *gorm.DB) error {
		existing := tx.Where("user_id = ? AND idempotency_key = ?", userId, idempotencyKey).First(&result)
		if existing.Error == nil {
			return nil
		}
		if !errors.Is(existing.Error, gorm.ErrRecordNotFound) {
			return existing.Error
		}
		campaign, rule, err := model.GetLuckyCampaignTx(tx, true)
		if err != nil {
			return err
		}
		if campaign.DrawPaused {
			return ErrLuckyDrawPaused
		}
		var card model.LuckyCard
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND user_id = ?", cardId, userId).First(&card).Error; err != nil {
			return ErrLuckyCardUnavailable
		}
		now := model.GetDBTimestamp()
		if card.Status != model.LuckyCardAvailable || card.ExpiresAt <= now {
			return ErrLuckyCardUnavailable
		}
		rawPool := rule.SubscriptionPool
		if card.PoolType == model.LuckyPoolRecharge {
			rawPool = rule.RechargePool
		}
		pool, err := model.ParseLuckyPool(rawPool)
		if err != nil {
			return err
		}
		randomValue, err := randomSource.Intn(model.LuckyWeightScale)
		if err != nil {
			return err
		}
		prize, err := SelectLuckyPrize(pool, randomValue)
		if err != nil {
			return err
		}
		result = model.LuckyDraw{
			UserId: userId, CardId: card.Id, RuleSetId: rule.Id,
			IdempotencyKey: idempotencyKey, RandomValue: randomValue,
			PrizeType: prize.Code, DisplayUsdMicros: prize.DisplayUsdMicros,
			ActualUsdMicros: prize.DisplayUsdMicros, RuleChecksum: rule.Checksum,
			Status: "awarded", AwardedAt: now,
		}
		if err := tx.Create(&result).Error; err != nil {
			return err
		}
		switch prize.Code {
		case model.LuckyPrizeQuota5, model.LuckyPrizeQuota10, model.LuckyPrizeQuota20, model.LuckyPrizeQuota30, model.LuckyPrizeQuota50, model.LuckyPrizeQuota100:
			err = awardQuotaPrizeTx(tx, &card, rule, &result, prize, now)
		case model.LuckyPrizeGift5, model.LuckyPrizeGift10, model.LuckyPrizeGift20:
			err = awardGiftPrizeTx(tx, &card, &result, prize)
		case model.LuckyPrizeDouble:
			if card.PoolType != model.LuckyPoolSubscription {
				return errors.New("subscription-only prize in recharge pool")
			}
			err = awardDoublePrizeTx(tx, &card, &result, now)
		case model.LuckyPrizeFullReset:
			if card.PoolType != model.LuckyPoolSubscription {
				return errors.New("subscription-only prize in recharge pool")
			}
			err = awardFullResetPrizeTx(tx, &card, &result, now)
		case model.LuckyPrizeCrazy5H:
			err = awardCrazyPrizeTx(tx, &card, rule, &result, now)
		default:
			err = fmt.Errorf("unknown lucky prize: %s", prize.Code)
		}
		if err != nil {
			return err
		}
		if err := tx.Save(&result).Error; err != nil {
			return err
		}
		card.Status = model.LuckyCardConsumed
		card.ConsumedAt = now
		return tx.Save(&card).Error
	})
	if err != nil {
		return nil, err
	}
	_ = model.InvalidateUserCache(userId)
	model.InvalidateUserTokenCaches(userId)
	return &result, nil
}

func DrawLuckyCard(userId int, cardId int64, idempotencyKey string) (*model.LuckyDraw, error) {
	return drawLuckyCardWithSource(userId, cardId, idempotencyKey, cryptoLuckyRandom{})
}
