package service

import (
	"errors"
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/oauth"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"gorm.io/gorm"
)

// RemoveLoginMethod serializes the availability check with deletion on the user
// row. A concurrent second removal observes the first one's committed state.
func RemoveLoginMethod(userID int, removePasskey bool, removeProvider int) error {
	return model.DB.Transaction(func(tx *gorm.DB) error {
		var user model.User
		if err := model.LockAuthenticationUser(tx, userID, &user); err != nil {
			return err
		}
		if err := canRemoveLoginMethod(tx, &user, removePasskey, removeProvider); err != nil {
			return err
		}
		var err error
		if removePasskey {
			err = tx.Unscoped().Where("user_id = ?", userID).Delete(&model.PasskeyCredential{}).Error
		} else {
			err = tx.Where("user_id = ? AND provider_id = ?", userID, removeProvider).Delete(&model.UserOAuthBinding{}).Error
		}
		if err != nil {
			return err
		}
		return model.RevokeUserAuthentication(tx, userID)
	})
}

func canRemoveLoginMethod(tx *gorm.DB, user *model.User, removePasskey bool, removeProvider int) error {
	userID := user.Id
	if common.PasswordLoginEnabled && user.Password != "" {
		return nil
	}
	if !removePasskey && system_setting.PasskeySettingsSnapshot().Enabled {
		var credential model.PasskeyCredential
		err := tx.Where("user_id = ?", userID).First(&credential).Error
		if err == nil {
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
	}
	if common.WeChatAuthEnabled && user.WeChatId != "" {
		return nil
	}
	if common.TelegramOAuthEnabled && user.TelegramId != "" {
		return nil
	}
	var bindings []model.UserOAuthBinding
	err := tx.Where("user_id = ?", userID).Find(&bindings).Error
	if err != nil {
		return err
	}
	for slug, provider := range oauth.GetAllProviders() {
		if !provider.IsEnabled() {
			continue
		}
		if custom, ok := provider.(*oauth.GenericOAuthProvider); ok {
			if custom.GetProviderId() == removeProvider {
				continue
			}
			for _, binding := range bindings {
				if binding.ProviderId == custom.GetProviderId() {
					return nil
				}
			}
		} else if (slug == "github" && user.GitHubId != "") || (slug == "discord" && user.DiscordId != "") || (slug == "oidc" && user.OidcId != "") || (slug == "linuxdo" && user.LinuxDOId != "") {
			return nil
		}
	}
	return errors.New("Keep at least one available login method before removing this binding")
}
