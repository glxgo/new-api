package model

import (
	"errors"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Serialize uniqueness checks for legacy binding columns, including databases
// which cannot add a partial unique index while historical empty values exist.
func validateAccountBindings(tx *gorm.DB, user *User, previous ...*User) error {
	fields := map[string]string{"email": user.Email, "github_id": user.GitHubId, "discord_id": user.DiscordId, "oidc_id": user.OidcId, "wechat_id": user.WeChatId, "telegram_id": user.TelegramId, "linux_do_id": user.LinuxDOId}
	if len(previous) > 0 && previous[0] != nil {
		old := previous[0]
		prior := map[string]string{"email": old.Email, "github_id": old.GitHubId, "discord_id": old.DiscordId, "oidc_id": old.OidcId, "wechat_id": old.WeChatId, "telegram_id": old.TelegramId, "linux_do_id": old.LinuxDOId}
		for key, value := range fields {
			if value == prior[key] {
				delete(fields, key)
			}
		}
	}
	hasBinding := false
	for _, value := range fields {
		if value != "" {
			hasBinding = true
			break
		}
	}
	if !hasBinding {
		return nil
	}
	guard := Option{Key: "_authentication_binding_lock", Value: "1"}
	if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&guard).Error; err != nil {
		return err
	}
	if err := tx.Model(&Option{}).Where(&Option{Key: guard.Key}).UpdateColumn("value", gorm.Expr("value")).Error; err != nil {
		return err
	}
	for column, value := range fields {
		if value == "" {
			continue
		}
		var count int64
		if err := tx.Model(&User{}).Where(column+" = ? AND id <> ?", value, user.Id).Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			return errors.New("This login identity is already linked to another account")
		}
	}
	return nil
}
