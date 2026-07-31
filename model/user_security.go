package model

import (
	"errors"
	"fmt"
	"time"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

const (
	UserSecurityRuleCyberPolicy = "cyber_policy"

	UserSecurityActionSuspend10Minutes = "suspend_10_minutes"
	UserSecurityActionSuspend2Hours    = "suspend_2_hours"
	UserSecurityActionSuspend24Hours   = "suspend_24_hours"
	UserSecurityActionPermanentBan     = "permanent_ban"
	UserSecurityActionObserved         = "observed_during_restriction"
	UserSecurityActionIntercepted      = "intercepted"

	UserSecurityErrorCodeContentPolicyBlocked = "content_policy_blocked"

	cyberPolicyInterceptionNotificationCooldown = 6 * time.Hour
)

type UserSecurityIncident struct {
	Id             int64  `json:"id" gorm:"primaryKey"`
	RequestId      string `json:"request_id" gorm:"type:varchar(64);not null;uniqueIndex"`
	UserId         int    `json:"user_id" gorm:"not null;index:idx_user_security_incident_user_created,priority:1"`
	TokenId        int    `json:"token_id" gorm:"not null;default:0;index"`
	RuleCode       string `json:"rule_code" gorm:"type:varchar(64);not null;index"`
	UpstreamCode   string `json:"upstream_code" gorm:"type:varchar(128)"`
	ModelName      string `json:"model_name" gorm:"type:varchar(128)"`
	Action         string `json:"action" gorm:"type:varchar(64);not null;index"`
	Counted        bool   `json:"counted" gorm:"not null;default:false;index"`
	StrikeNumber   int    `json:"strike_number" gorm:"not null;default:0"`
	SuspendedUntil int64  `json:"suspended_until" gorm:"not null;default:0"`
	CreatedAt      int64  `json:"created_at" gorm:"bigint;not null;index:idx_user_security_incident_user_created,priority:2"`
}

func (UserSecurityIncident) TableName() string {
	return "user_security_incidents"
}

type UserSecurityEnforcementResult struct {
	Counted        bool
	StrikeNumber   int
	Action         string
	SuspendedUntil int64
	Permanent      bool
}

type UserSecurityInterceptionResult struct {
	Recorded     bool
	ShouldNotify bool
}

func securityActionForStrike(strike int, now int64) (string, int64, bool) {
	switch strike {
	case 1:
		return UserSecurityActionSuspend10Minutes, now + int64((10 * time.Minute).Seconds()), false
	case 2:
		return UserSecurityActionSuspend2Hours, now + int64((2 * time.Hour).Seconds()), false
	case 3:
		return UserSecurityActionSuspend24Hours, now + int64((24 * time.Hour).Seconds()), false
	default:
		return UserSecurityActionPermanentBan, 0, true
	}
}

func ApplyCyberPolicyViolation(userId int, tokenId int, requestId string, modelName string, upstreamCode string, now int64) (UserSecurityEnforcementResult, error) {
	result := UserSecurityEnforcementResult{}
	if userId <= 0 || requestId == "" {
		return result, errors.New("invalid security incident identity")
	}
	if now <= 0 {
		now = common.GetTimestamp()
	}

	err := DB.Transaction(func(tx *gorm.DB) error {
		var user User
		if err := tx.Set("gorm:query_option", "FOR UPDATE").Where("id = ?", userId).First(&user).Error; err != nil {
			return err
		}
		if common.CyberPolicyInterceptionEnabled || !common.CyberPolicyEnforcementEnabled || user.SecurityWhitelisted {
			return nil
		}

		var existing UserSecurityIncident
		if err := tx.Where("request_id = ?", requestId).First(&existing).Error; err == nil {
			result = UserSecurityEnforcementResult{
				Counted:        false,
				StrikeNumber:   existing.StrikeNumber,
				Action:         existing.Action,
				SuspendedUntil: existing.SuspendedUntil,
				Permanent:      existing.Action == UserSecurityActionPermanentBan,
			}
			return nil
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		counted := !user.SecurityPermanentBan && user.SecuritySuspendedUntil <= now
		strike := user.SecurityStrikeCount
		action := UserSecurityActionObserved
		suspendedUntil := user.SecuritySuspendedUntil
		permanent := user.SecurityPermanentBan
		if counted {
			strike++
			action, suspendedUntil, permanent = securityActionForStrike(strike, now)
			updates := map[string]interface{}{
				"security_strike_count":    strike,
				"security_suspended_until": suspendedUntil,
				"security_permanent_ban":   permanent,
			}
			if err := tx.Model(&User{}).Where("id = ?", userId).Updates(updates).Error; err != nil {
				return err
			}
		}

		incident := UserSecurityIncident{
			RequestId:      requestId,
			UserId:         userId,
			TokenId:        tokenId,
			RuleCode:       UserSecurityRuleCyberPolicy,
			UpstreamCode:   upstreamCode,
			ModelName:      modelName,
			Action:         action,
			Counted:        counted,
			StrikeNumber:   strike,
			SuspendedUntil: suspendedUntil,
			CreatedAt:      now,
		}
		if err := tx.Create(&incident).Error; err != nil {
			return err
		}
		result = UserSecurityEnforcementResult{
			Counted:        counted,
			StrikeNumber:   strike,
			Action:         action,
			SuspendedUntil: suspendedUntil,
			Permanent:      permanent,
		}
		return nil
	})
	if err != nil {
		return UserSecurityEnforcementResult{}, err
	}
	if err := InvalidateUserCache(userId); err != nil {
		common.SysLog(fmt.Sprintf("failed to invalidate security cache for user %d: %s", userId, err.Error()))
	}
	return result, nil
}

// RecordCyberPolicyInterception records a privacy-safe, non-punitive audit
// event. It never changes the user's strike, suspension, permanent-ban,
// status, API keys, group, or channel affinity.
func RecordCyberPolicyInterception(userId int, tokenId int, requestId string, modelName string, upstreamCode string, now int64) (UserSecurityInterceptionResult, error) {
	result := UserSecurityInterceptionResult{}
	if !common.CyberPolicyInterceptionEnabled {
		return result, nil
	}
	if userId <= 0 || requestId == "" {
		return result, errors.New("invalid security interception identity")
	}
	if now <= 0 {
		now = common.GetTimestamp()
	}

	err := DB.Transaction(func(tx *gorm.DB) error {
		var user User
		if err := tx.Set("gorm:query_option", "FOR UPDATE").
			Select("id", "security_whitelisted").
			Where("id = ?", userId).
			First(&user).Error; err != nil {
			return err
		}

		var existing UserSecurityIncident
		if err := tx.Where("request_id = ?", requestId).First(&existing).Error; err == nil {
			return nil
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		var recentNotifications int64
		if err := tx.Model(&UserSecurityIncident{}).
			Where(
				"user_id = ? AND rule_code = ? AND action = ? AND created_at >= ?",
				userId,
				UserSecurityRuleCyberPolicy,
				UserSecurityActionIntercepted,
				now-int64(cyberPolicyInterceptionNotificationCooldown.Seconds()),
			).
			Count(&recentNotifications).Error; err != nil {
			return err
		}

		incident := UserSecurityIncident{
			RequestId:    requestId,
			UserId:       userId,
			TokenId:      tokenId,
			RuleCode:     UserSecurityRuleCyberPolicy,
			UpstreamCode: upstreamCode,
			ModelName:    modelName,
			Action:       UserSecurityActionIntercepted,
			Counted:      false,
			CreatedAt:    now,
		}
		if err := tx.Create(&incident).Error; err != nil {
			return err
		}
		result.Recorded = true
		result.ShouldNotify = !user.SecurityWhitelisted && recentNotifications == 0
		return nil
	})
	if err != nil {
		return UserSecurityInterceptionResult{}, err
	}
	return result, nil
}

func ResetUserSecurityRestriction(userId int) error {
	if userId <= 0 {
		return errors.New("invalid user id")
	}
	result := DB.Model(&User{}).Where("id = ?", userId).Updates(map[string]interface{}{
		"security_strike_count":    0,
		"security_suspended_until": 0,
		"security_permanent_ban":   false,
	})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return fmt.Errorf("user %d not found", userId)
	}
	if err := InvalidateUserCache(userId); err != nil {
		common.SysLog(fmt.Sprintf("failed to invalidate reset security cache for user %d: %s", userId, err.Error()))
	}
	return nil
}

func SetUserSecurityWhitelist(userId int, enabled bool) error {
	if userId <= 0 {
		return errors.New("invalid user id")
	}
	updates := map[string]interface{}{
		"security_whitelisted": enabled,
	}
	if enabled {
		updates["security_strike_count"] = 0
		updates["security_suspended_until"] = 0
		updates["security_permanent_ban"] = false
	}
	result := DB.Model(&User{}).Where("id = ?", userId).Updates(updates)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return fmt.Errorf("user %d not found", userId)
	}
	if err := InvalidateUserCache(userId); err != nil {
		common.SysLog(fmt.Sprintf("failed to invalidate security whitelist cache for user %d: %s", userId, err.Error()))
	}
	return nil
}
