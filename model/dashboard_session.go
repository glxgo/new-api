package model

import (
	"errors"
	"time"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

// DashboardSession makes logout and credential changes effective even when a
// client retains an old, correctly signed session cookie.
type DashboardSession struct {
	ID           string `gorm:"primaryKey;size:64"`
	UserID       int    `gorm:"index;not null"`
	PasswordHash string `gorm:"size:64;not null"`
	AuthVersion  int    `gorm:"not null;default:0"`
	ExpiresAt    int64  `gorm:"index;not null"`
}

func CreateDashboardSession(user *User, token string) error {
	return DB.Transaction(func(tx *gorm.DB) error {
		var current User
		if err := LockAuthenticationUser(tx, user.Id, &current); err != nil {
			return err
		}
		if current.Status != common.UserStatusEnabled || current.Password != user.Password || current.AuthVersion != user.AuthVersion {
			return ErrSecurityProof
		}
		return tx.Create(&DashboardSession{ID: SecurityDigest(token), UserID: user.Id, AuthVersion: current.AuthVersion,
			PasswordHash: common.GenerateHMAC(user.Password), ExpiresAt: time.Now().Add(30 * 24 * time.Hour).Unix()}).Error
	})
}

func ValidateDashboardSession(userID int, token string) (*User, error) {
	if userID <= 0 || len(token) != 43 {
		return nil, ErrSecurityProof
	}
	var session DashboardSession
	if err := DB.Where("id = ? AND user_id = ? AND expires_at > ?", SecurityDigest(token), userID, time.Now().Unix()).First(&session).Error; err != nil {
		return nil, err
	}
	user, err := GetUserById(userID, true)
	if err != nil {
		return nil, err
	}
	if user.Status != common.UserStatusEnabled || session.AuthVersion != user.AuthVersion || session.PasswordHash != common.GenerateHMAC(user.Password) {
		return nil, ErrSecurityProof
	}
	return user, nil
}

func RevokeDashboardSession(token string) error {
	return DB.Where("id = ?", SecurityDigest(token)).Delete(&DashboardSession{}).Error
}

func RevokeUserAuthentication(tx *gorm.DB, userID int) error {
	if err := tx.Model(&User{}).Where("id = ?", userID).UpdateColumn("auth_version", gorm.Expr("auth_version + 1")).Error; err != nil {
		return err
	}
	if err := tx.Where("user_id = ?", userID).Delete(&DashboardSession{}).Error; err != nil {
		return err
	}
	return tx.Where("user_id = ?", userID).Delete(&SecurityProof{}).Error
}

func DeleteAccountWithSession(userID int, sessionToken string) error {
	err := DB.Transaction(func(tx *gorm.DB) error {
		var user User
		if err := LockAuthenticationUser(tx, userID, &user); err != nil {
			return err
		}
		if user.Role == common.RoleRootUser {
			return errors.New("Cannot delete the root account")
		}
		var session DashboardSession
		if err := tx.Where("id = ? AND user_id = ? AND expires_at > ?", SecurityDigest(sessionToken), userID, time.Now().Unix()).First(&session).Error; err != nil {
			return ErrSecurityProof
		}
		if user.Status != common.UserStatusEnabled || session.AuthVersion != user.AuthVersion || session.PasswordHash != common.GenerateHMAC(user.Password) {
			return ErrSecurityProof
		}
		if err := RevokeUserAuthentication(tx, userID); err != nil {
			return err
		}
		if err := tx.Model(&user).Update("access_token", nil).Error; err != nil {
			return err
		}
		return tx.Delete(&user).Error
	})
	if err != nil {
		return err
	}
	return invalidateUserCache(userID)
}

// LockAuthenticationUser must precede checks of remaining login methods. The
// harmless update also obtains SQLite's writer lock before the first read.
func LockAuthenticationUser(tx *gorm.DB, userID int, user *User) error {
	if userID <= 0 {
		return ErrSecurityProof
	}
	if err := tx.Model(&User{}).Where("id = ?", userID).UpdateColumn("id", gorm.Expr("id")).Error; err != nil {
		return err
	}
	return tx.First(user, userID).Error
}
