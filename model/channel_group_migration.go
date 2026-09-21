package model

import (
	"fmt"
	"strings"

	"gorm.io/gorm"
)

// Run before AutoMigrate so old MySQL TEXT defaults do not interfere with the
// widening. Keep existing values; the application supplies the create default.
func migrateChannelGroupsToLongText(db *gorm.DB) error {
	if db.Dialector.Name() == "sqlite" || !db.Migrator().HasTable(&Channel{}) || !db.Migrator().HasColumn(&Channel{}, "group") {
		return nil
	}
	var dataType string
	var err error
	switch db.Dialector.Name() {
	case "mysql":
		err = db.Raw("SELECT DATA_TYPE FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'channels' AND column_name = 'group'").Scan(&dataType).Error
		if err == nil && strings.ToLower(dataType) != "longtext" {
			err = db.Exec("ALTER TABLE channels MODIFY COLUMN `group` LONGTEXT").Error
		}
	case "postgres":
		err = db.Raw("SELECT data_type FROM information_schema.columns WHERE table_schema = current_schema() AND table_name = 'channels' AND column_name = 'group'").Scan(&dataType).Error
		if err == nil && dataType != "text" {
			err = db.Exec(`ALTER TABLE channels ALTER COLUMN "group" DROP DEFAULT, ALTER COLUMN "group" TYPE TEXT`).Error
		}
	}
	if err != nil {
		return fmt.Errorf("migrate channels.group to long text: %w", err)
	}
	return nil
}
