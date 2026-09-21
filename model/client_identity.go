package model

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type ClientIdentity struct {
	ClientKey    string `json:"client_key" gorm:"primaryKey;type:varchar(96)"`
	Family       string `json:"family" gorm:"type:varchar(40);index"`
	Variant      string `json:"variant" gorm:"type:varchar(40)"`
	DisplayName  string `json:"display_name" gorm:"type:varchar(100)"`
	UserAgent    string `json:"user_agent" gorm:"type:text"`
	Truncated    bool   `json:"truncated"`
	RequestCount int64  `json:"request_count"`
	LastSeen     int64  `json:"last_seen" gorm:"index"`
	Status       string `json:"status" gorm:"type:varchar(16);index;default:pending"`
	Revision     int64  `json:"revision"`
}
type ClientReview struct {
	ID             int64  `json:"id" gorm:"primaryKey"`
	ClientKey      string `json:"client_key" gorm:"type:varchar(96);index"`
	OperatorID     int    `json:"operator_id"`
	PreviousStatus string `json:"previous_status" gorm:"type:varchar(16)"`
	Status         string `json:"status" gorm:"type:varchar(16)"`
	Reason         string `json:"reason" gorm:"type:text"`
	CreatedAt      int64  `json:"created_at"`
}
type ClientGroupPolicy struct {
	GroupName        string   `json:"group_name" gorm:"primaryKey;type:varchar(64)"`
	IsCoding         bool     `json:"is_coding"`
	SupportedClients []string `json:"supported_clients" gorm:"serializer:json;type:text"`
	Revision         int64    `json:"revision"`
}
type ClientGroupPolicyReview struct {
	ID             int64  `json:"id" gorm:"primaryKey"`
	GroupName      string `json:"group_name" gorm:"type:varchar(64);index"`
	OperatorID     int    `json:"operator_id"`
	Reason         string `json:"reason" gorm:"type:text"`
	PreviousPolicy string `json:"previous_policy" gorm:"type:text"`
	Policy         string `json:"policy" gorm:"type:text"`
	CreatedAt      int64  `json:"created_at"`
}

// One atomic increment per authenticated request, never per retry. Upgrades
// update the sample but cannot reset an approval or create a new known identity.
func ObserveClient(ctx context.Context, s common.ClientSnapshot) error {
	row := ClientIdentity{ClientKey: s.ClientKey, Family: s.Family, Variant: s.Variant, DisplayName: s.DisplayName, UserAgent: s.UserAgent, Truncated: s.Truncated, RequestCount: 1, LastSeen: time.Now().Unix(), Status: "pending"}
	return DB.WithContext(ctx).Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "client_key"}}, DoUpdates: clause.Assignments(map[string]interface{}{"request_count": gorm.Expr("client_identities.request_count + ?", 1), "last_seen": row.LastSeen, "user_agent": s.UserAgent, "truncated": s.Truncated})}).Create(&row).Error
}
func ReviewClient(ctx context.Context, key, status, reason string, operator int, revision int64) error {
	reason = strings.TrimSpace(reason)
	if reason == "" || len(reason) > 2000 || operator <= 0 || (status != "approved" && status != "rejected" && status != "pending") {
		return errors.New("invalid review or missing reason")
	}
	return DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var row ClientIdentity
		if err := tx.Where("client_key = ?", key).First(&row).Error; err != nil {
			return err
		}
		if row.Revision != revision {
			return errors.New("review changed; refresh and retry")
		}
		if row.Family == "unknown" && status == "approved" {
			return errors.New("unknown UA requires an explicit recognition rule before approval")
		}
		result := tx.Model(&ClientIdentity{}).Where("client_key = ? AND revision = ?", key, revision).Updates(map[string]interface{}{"status": status, "revision": gorm.Expr("revision + 1")})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return errors.New("review changed; refresh and retry")
		}
		return tx.Create(&ClientReview{ClientKey: key, OperatorID: operator, PreviousStatus: row.Status, Status: status, Reason: reason, CreatedAt: time.Now().Unix()}).Error
	})
}
func IsCodingGroupName(group string) bool { return strings.Contains(strings.ToLower(group), "coding") }
func CheckClientGroupAccess(ctx context.Context, s common.ClientSnapshot, group string) (bool, error) {
	if group == "" || group == "auto" {
		return false, nil
	}
	var p ClientGroupPolicy
	result := DB.WithContext(ctx).Where("group_name = ?", group).Limit(1).Find(&p)
	if result.Error != nil {
		return IsCodingGroupName(group), errors.New("client policy is unavailable")
	}
	if result.RowsAffected == 0 {
		p.IsCoding = IsCodingGroupName(group)
	}
	if !p.IsCoding {
		return false, nil
	}
	if s.Family == "unknown" || s.ClientKey == "" {
		return true, errors.New("unknown client cannot use a Coding group")
	}
	var count int64
	if err := DB.WithContext(ctx).Model(&ClientIdentity{}).Where("client_key = ? AND status = ?", s.ClientKey, "approved").Count(&count).Error; err != nil {
		return true, errors.New("client approval is unavailable")
	}
	if count != 1 {
		return true, errors.New("client is not approved for Coding groups")
	}
	for _, key := range p.SupportedClients {
		if key == s.ClientKey {
			return true, nil
		}
	}
	return true, errors.New("Coding group does not support this client")
}
func SaveClientGroupPolicy(ctx context.Context, p ClientGroupPolicy, reason string, operator int) error {
	reason = strings.TrimSpace(reason)
	if p.GroupName == "" || len(p.GroupName) > 64 || len(p.SupportedClients) > 200 || reason == "" || len(reason) > 2000 || operator <= 0 {
		return errors.New("invalid group policy or missing reason")
	}
	for _, k := range p.SupportedClients {
		if k == "" || len(k) > 96 || strings.HasPrefix(k, "unknown:") {
			return errors.New("invalid supported client")
		}
	}
	return DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var old ClientGroupPolicy
		err := tx.Where("group_name = ?", p.GroupName).First(&old).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		before, _ := common.Marshal(old)
		if errors.Is(err, gorm.ErrRecordNotFound) {
			if p.Revision != 0 {
				return errors.New("policy changed; refresh and retry")
			}
			p.Revision = 1
			if err := tx.Create(&p).Error; err != nil {
				return err
			}
		} else {
			next := p
			next.Revision++
			result := tx.Model(&ClientGroupPolicy{}).Where("group_name = ? AND revision = ?", p.GroupName, p.Revision).Select("IsCoding", "SupportedClients", "Revision").Updates(&next)
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected != 1 {
				return errors.New("policy changed; refresh and retry")
			}
			p = next
		}
		after, _ := common.Marshal(p)
		return tx.Create(&ClientGroupPolicyReview{GroupName: p.GroupName, OperatorID: operator, Reason: reason, PreviousPolicy: string(before), Policy: string(after), CreatedAt: time.Now().Unix()}).Error
	})
}
