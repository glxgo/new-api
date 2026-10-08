package model

import (
	"errors"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/pkg/kiro"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// These records are internal accounting evidence, never public pricing data.
// A hash gives exact, case-sensitive metering ID uniqueness on all supported
// databases, including MySQL installations with case-insensitive collations.
type KiroCreditSettlement struct {
	MeteringKey      string `gorm:"primaryKey;size:64"`
	MeteringID       string `gorm:"size:200;not null"`
	RequestID        string `gorm:"size:64;index"`
	AttemptID        string `gorm:"size:64"`
	UserID           int    `gorm:"index"`
	TokenID          int
	ChannelID        int
	Status           string `gorm:"size:32;index"`
	Quota            int64
	RawReceipt       string  `gorm:"type:text"`
	PriceSnapshot    string  `gorm:"type:text"`
	KnownCostCNY     *string `gorm:"type:text"`
	ConsumedCoverage string  `gorm:"size:16"`
	CreatedAt        int64
	UpdatedAt        int64
}

type KiroCreditAttempt struct {
	AttemptID        string `gorm:"primaryKey;size:64"`
	RequestID        string `gorm:"size:64;index"`
	UserID           int    `gorm:"index"`
	ChannelID        int
	Mode             string  `gorm:"size:16"`
	Status           string  `gorm:"size:32"`
	RawReceipt       string  `gorm:"type:text"`
	ReceiptEvidence  string  `gorm:"type:text"`
	Error            string  `gorm:"type:text"`
	CostPerCredit    string  `gorm:"type:text"`
	KnownCostCNY     *string `gorm:"type:text"`
	ConsumedCoverage string  `gorm:"size:16"`
	ShadowQuota      *int64
	CreatedAt        int64
}

// ClaimKiroSettlement persists the pending evidence BEFORE any final debit.
// A duplicate in ANY state is never replayed automatically. Reconciliation of
// pending/needs_review requires examining the funding and token ledgers.
func ClaimKiroSettlement(row *KiroCreditSettlement) (bool, error) {
	if DB == nil {
		return false, errors.New("settlement database unavailable")
	}
	row.MeteringKey = kiro.MeteringKey(row.MeteringID)
	row.Status = "pending"
	row.CreatedAt = time.Now().Unix()
	row.UpdatedAt = row.CreatedAt
	result := DB.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "metering_key"}}, DoNothing: true}).Create(row)
	return result.RowsAffected == 1, result.Error
}

func FinishKiroSettlement(key, status string) error {
	if DB == nil {
		return errors.New("settlement database unavailable")
	}
	result := DB.Model(&KiroCreditSettlement{}).Where("metering_key = ? AND status = ?", key, "pending").Updates(map[string]any{"status": status, "updated_at": time.Now().Unix()})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return errors.New("settlement state changed unexpectedly")
	}
	return nil
}

func RecordKiroCreditAttempt(row *KiroCreditAttempt) error {
	if DB == nil {
		return errors.New("attempt database unavailable")
	}
	row.CreatedAt = time.Now().Unix()
	return DB.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "attempt_id"}}, DoNothing: true}).Create(row).Error
}

// The legacy cost field is a quota integer. It cannot describe partial or
// unknown CNY consumption, or deduplicate costs across repeated receipts.
// Persist NULL for rule 3; accounting must use the nullable credit ledger.
func createConsumeLog(log *Log) error {
	if log.CostRuleVersion != 3 {
		return LOG_DB.Create(log).Error
	}
	return LOG_DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(log).Error; err != nil {
			return err
		}
		return tx.Model(log).UpdateColumn("cost", nil).Error
	})
}

func (log Log) MarshalJSON() ([]byte, error) {
	type plain Log
	if log.CostRuleVersion != 3 {
		return common.Marshal(plain(log))
	}
	return common.Marshal(struct {
		plain
		Cost *int `json:"cost"`
	}{plain: plain(log), Cost: nil})
}
