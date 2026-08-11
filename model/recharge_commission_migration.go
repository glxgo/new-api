package model

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	rechargeCommissionMigrationKey = "RechargeCommissionPolicyV1CutoverV3"
	rechargeCommissionCutoverAtKey = "RechargeCommissionPolicyV1CutoverAt"
	rechargeCommissionLogBatchV1   = "recharge_policy_v1"
)

// MigrateRechargeCommissionPolicyV1 establishes a strict cutover boundary for
// the fixed recharge-commission policy. Historical settled rewards remain
// immutable, while historical unsettled orders and consumption-profit rows are
// deliberately ignored: the new policy never scans or backfills them.
func MigrateRechargeCommissionPolicyV1() error {
	if !common.IsMasterNode {
		return nil
	}
	var marker Option
	if err := DB.Where(optionKeyWhereClause(), rechargeCommissionMigrationKey).First(&marker).Error; err == nil && marker.Value == "true" {
		return nil
	} else if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}

	cutoverAt := common.GetTimestamp()
	var existing Option
	if err := DB.Where(optionKeyWhereClause(), rechargeCommissionCutoverAtKey).First(&existing).Error; err == nil {
		parsed, parseErr := strconv.ParseInt(strings.TrimSpace(existing.Value), 10, 64)
		if parseErr != nil || parsed <= 0 {
			return errors.New("invalid recharge commission cutover timestamp")
		}
		cutoverAt = parsed
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}

	return DB.Transaction(func(tx *gorm.DB) error {
		// A running legacy batch was never paid and must remain unpaid. Completed
		// historical batches are left untouched.
		if err := tx.Model(&AffiliateSettle{}).
			Where("status = ?", AffiliateSettleStatusRunning).
			Update("status", AffiliateSettleStatusFailed).Error; err != nil {
			return err
		}
		options := []Option{
			{Key: rechargeCommissionMigrationKey, Value: "true"},
			{Key: rechargeCommissionCutoverAtKey, Value: strconv.FormatInt(cutoverAt, 10)},
		}
		return tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "key"}},
			DoUpdates: clause.AssignmentColumns([]string{"value"}),
		}).Create(&options).Error
	})
}

func rechargeCommissionCutoverAtTx(tx *gorm.DB) (int64, error) {
	if tx == nil || !tx.Migrator().HasTable(&Option{}) {
		return 0, nil
	}
	var option Option
	if err := tx.Where(optionKeyWhereClause(), rechargeCommissionCutoverAtKey).First(&option).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return 0, nil
		}
		return 0, err
	}
	cutoverAt, err := strconv.ParseInt(strings.TrimSpace(option.Value), 10, 64)
	if err != nil || cutoverAt <= 0 {
		return 0, fmt.Errorf("invalid recharge commission cutover timestamp")
	}
	return cutoverAt, nil
}

// RechargeCommissionCutoverAt returns the persisted boundary used by reports
// and reconciliation guards. Zero means an older database has not migrated yet.
func RechargeCommissionCutoverAt() (int64, error) {
	return rechargeCommissionCutoverAtTx(DB)
}
