package service

import (
	"errors"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func SetLuckyIssuancePaused(paused bool, operatorId int, reason string) error {
	return model.DB.Transaction(func(tx *gorm.DB) error {
		campaign, _, err := model.GetLuckyCampaignTx(tx, true)
		if err != nil {
			return err
		}
		if campaign.IssuancePaused == paused {
			return nil
		}
		campaign.IssuancePaused = paused
		campaign.SettingsVersion++
		if err := tx.Save(campaign).Error; err != nil {
			return err
		}
		now := model.GetDBTimestamp()
		period := model.LuckyPausePeriod{
			CampaignId: campaign.Id, PauseType: "issuance", StartedAt: now,
			Reason: strings.TrimSpace(reason), OperatorId: operatorId,
			Status: "completed", CreatedAt: now, UpdatedAt: now,
		}
		if !paused {
			period.EndedAt = now
		}
		return tx.Create(&period).Error
	})
}

func PauseLuckyDraw(operatorId int, reason string) error {
	return model.DB.Transaction(func(tx *gorm.DB) error {
		campaign, _, err := model.GetLuckyCampaignTx(tx, true)
		if err != nil {
			return err
		}
		if campaign.DrawPaused {
			return nil
		}
		now := model.GetDBTimestamp()
		campaign.DrawPaused = true
		campaign.DrawPauseStartedAt = now
		campaign.SettingsVersion++
		if err := tx.Save(campaign).Error; err != nil {
			return err
		}
		return tx.Create(&model.LuckyPausePeriod{
			CampaignId: campaign.Id, PauseType: "draw", StartedAt: now,
			Reason: strings.TrimSpace(reason), OperatorId: operatorId,
			Status: "active", CreatedAt: now, UpdatedAt: now,
		}).Error
	})
}

func ResumeLuckyDraw(operatorId int, reason string) error {
	return model.DB.Transaction(func(tx *gorm.DB) error {
		campaign, _, err := model.GetLuckyCampaignTx(tx, true)
		if err != nil {
			return err
		}
		if !campaign.DrawPaused {
			return nil
		}
		var period model.LuckyPausePeriod
		result := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("campaign_id = ? AND pause_type = ? AND status = ?", campaign.Id, "draw", "active").
			Order("id desc").First(&period)
		if result.Error != nil && !errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return result.Error
		}
		startedAt := campaign.DrawPauseStartedAt
		if result.Error == nil && period.StartedAt > 0 {
			startedAt = period.StartedAt
		}
		now := model.GetDBTimestamp()
		duration := int64(0)
		if startedAt > 0 {
			duration = now - startedAt
		}
		if duration < 0 {
			duration = 0
		}
		beforePause := tx.Model(&model.LuckyCard{}).
			Where("campaign_id = ? AND status = ? AND issued_at <= ? AND expires_at > ?",
				campaign.Id, model.LuckyCardAvailable, startedAt, startedAt).
			Updates(map[string]interface{}{
				"expires_at":                gorm.Expr("expires_at + ?", duration),
				"source_effective_end_time": gorm.Expr("CASE WHEN source_effective_end_time > 0 THEN source_effective_end_time + ? ELSE 0 END", duration),
				"pause_extension_seconds":   gorm.Expr("pause_extension_seconds + ?", duration),
				"updated_at":                now,
			})
		if beforePause.Error != nil {
			return beforePause.Error
		}
		// Issuance and drawing can be paused independently. Cards created while
		// drawing is paused must recover only the time since their own issuance,
		// not the whole pause interval.
		duringPause := tx.Model(&model.LuckyCard{}).
			Where("campaign_id = ? AND status = ? AND issued_at > ? AND issued_at <= ? AND expires_at > issued_at",
				campaign.Id, model.LuckyCardAvailable, startedAt, now).
			Updates(map[string]interface{}{
				"expires_at": gorm.Expr("expires_at + (? - issued_at)", now),
				"source_effective_end_time": gorm.Expr(
					"CASE WHEN source_effective_end_time > 0 THEN source_effective_end_time + (? - issued_at) ELSE 0 END",
					now,
				),
				"pause_extension_seconds": gorm.Expr("pause_extension_seconds + (? - issued_at)", now),
				"updated_at":              now,
			})
		if duringPause.Error != nil {
			return duringPause.Error
		}
		if result.Error == nil {
			period.EndedAt = now
			period.DurationSeconds = duration
			period.AffectedCards = beforePause.RowsAffected + duringPause.RowsAffected
			period.Status = "completed"
			if strings.TrimSpace(reason) != "" {
				period.Reason = strings.TrimSpace(reason)
			}
			period.OperatorId = operatorId
			period.UpdatedAt = now
			if err := tx.Save(&period).Error; err != nil {
				return err
			}
		}
		campaign.DrawPaused = false
		campaign.DrawPauseStartedAt = 0
		campaign.SettingsVersion++
		campaign.UpdatedAt = common.GetTimestamp()
		return tx.Save(campaign).Error
	})
}
