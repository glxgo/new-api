package model

import (
	"errors"

	"gorm.io/gorm"
)

const (
	userCapacityOverridesMigrationKey = "UserCapacityOverridesMigratedV1"
	legacyDefaultUserConcurrencyLimit = 8
)

// migrateUserCapacityOverridesV1 preserves pre-existing non-default account
// concurrency limits as explicit administrator overrides. RPM was not
// independently configurable before this migration, so every existing account
// initially inherits the global RPM default.
func migrateUserCapacityOverridesV1() error {
	return DB.Transaction(func(tx *gorm.DB) error {
		var marker Option
		err := tx.Where(&Option{Key: userCapacityOverridesMigrationKey}).First(&marker).Error
		if err == nil {
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		if err := tx.Model(&User{}).
			Where("concurrency_limit > ? AND concurrency_limit <> ? AND concurrency_limit_override = ?", 0, legacyDefaultUserConcurrencyLimit, false).
			Update("concurrency_limit_override", true).Error; err != nil {
			return err
		}

		return tx.Create(&Option{Key: userCapacityOverridesMigrationKey, Value: "true"}).Error
	})
}
