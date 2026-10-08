package model

import (
	"errors"
	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

// StorePendingTwoFA serializes setup with enable/disable and replaces backup
// codes in the same transaction. An enabled factor can never be overwritten.
func StorePendingTwoFA(userID int, secret string, codes []string) error {
	rows := make([]TwoFABackupCode, 0, len(codes))
	for _, code := range codes {
		hash, err := common.HashBackupCode(code)
		if err != nil {
			return err
		}
		rows = append(rows, TwoFABackupCode{UserId: userID, CodeHash: hash})
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		var user User
		if err := LockAuthenticationUser(tx, userID, &user); err != nil {
			return err
		}
		var current TwoFA
		err := tx.Where("user_id = ?", userID).First(&current).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if current.IsEnabled {
			return ErrSecurityProof
		}
		if current.Id != 0 {
			if err := tx.Unscoped().Delete(&current).Error; err != nil {
				return err
			}
		}
		if err := tx.Create(&TwoFA{UserId: userID, Secret: secret}).Error; err != nil {
			return err
		}
		if err := tx.Unscoped().Where("user_id = ?", userID).Delete(&TwoFABackupCode{}).Error; err != nil {
			return err
		}
		return tx.Create(&rows).Error
	})
}
