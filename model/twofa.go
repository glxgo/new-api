package model

import (
	"errors"
	"fmt"
	"time"

	"github.com/QuantumNous/new-api/common"

	"gorm.io/gorm"
)

// TwoFA 用户2FA设置表
type TwoFA struct {
	LastVerifiedStep int64          `json:"-" gorm:"not null;default:0"`
	Id               int            `json:"id" gorm:"primaryKey"`
	UserId           int            `json:"user_id" gorm:"unique;not null;index"`
	Secret           string         `json:"-" gorm:"type:varchar(255);not null"` // TOTP密钥，不返回给前端
	IsEnabled        bool           `json:"is_enabled"`
	FailedAttempts   int            `json:"failed_attempts" gorm:"default:0"`
	LockedUntil      *time.Time     `json:"locked_until,omitempty"`
	LastUsedAt       *time.Time     `json:"last_used_at,omitempty"`
	CreatedAt        time.Time      `json:"created_at"`
	UpdatedAt        time.Time      `json:"updated_at"`
	DeletedAt        gorm.DeletedAt `json:"-" gorm:"index"`
}

// TwoFABackupCode 备用码使用记录表
type TwoFABackupCode struct {
	Id        int            `json:"id" gorm:"primaryKey"`
	UserId    int            `json:"user_id" gorm:"not null;index"`
	CodeHash  string         `json:"-" gorm:"type:varchar(255);not null"` // 备用码哈希
	IsUsed    bool           `json:"is_used"`
	UsedAt    *time.Time     `json:"used_at,omitempty"`
	CreatedAt time.Time      `json:"created_at"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`
}

// GetTwoFAByUserId 根据用户ID获取2FA设置
func GetTwoFAByUserId(userId int) (*TwoFA, error) {
	if userId == 0 {
		return nil, errors.New("用户ID不能为空")
	}

	var twoFA TwoFA
	err := DB.Where("user_id = ?", userId).First(&twoFA).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil // 返回nil表示未设置2FA
		}
		return nil, err
	}

	return &twoFA, nil
}

// IsTwoFAEnabled 检查用户是否启用了2FA
func IsTwoFAEnabled(userId int) bool {
	twoFA, err := GetTwoFAByUserId(userId)
	if err != nil || twoFA == nil {
		return false
	}
	return twoFA.IsEnabled
}

// CreateTwoFA 创建2FA设置
func (t *TwoFA) Create() error {
	// 检查用户是否已存在2FA设置
	existing, err := GetTwoFAByUserId(t.UserId)
	if err != nil {
		return err
	}
	if existing != nil {
		return errors.New("用户已存在2FA设置")
	}

	// 验证用户存在
	var user User
	if err := DB.First(&user, t.UserId).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("用户不存在")
		}
		return err
	}

	return DB.Create(t).Error
}

// Update 更新2FA设置
func (t *TwoFA) Update() error {
	if t.Id == 0 {
		return errors.New("2FA记录ID不能为空")
	}
	return DB.Model(&TwoFA{}).Where("id = ?", t.Id).Omit("last_verified_step").Select("*").Updates(t).Error
}

// Delete 删除2FA设置
func (t *TwoFA) Delete() error {
	if t.Id == 0 {
		return errors.New("2FA记录ID不能为空")
	}

	// 使用事务确保原子性
	return DB.Transaction(func(tx *gorm.DB) error {
		// 同时删除相关的备用码记录（硬删除）
		if err := tx.Unscoped().Where("user_id = ?", t.UserId).Delete(&TwoFABackupCode{}).Error; err != nil {
			return err
		}

		if err := tx.Unscoped().Delete(t).Error; err != nil {
			return err
		}
		return RevokeUserAuthentication(tx, t.UserId)
	})
}

// ResetFailedAttempts 重置失败尝试次数
func (t *TwoFA) ResetFailedAttempts() error {
	return DB.Model(&TwoFA{}).Where("id = ?", t.Id).Updates(map[string]any{"failed_attempts": 0, "locked_until": nil}).Error
}

func (t *TwoFA) IncrementFailedAttempts() error {
	return DB.Transaction(func(tx *gorm.DB) error {
		var current TwoFA
		if err := tx.Model(&TwoFA{}).Where("id = ?", t.Id).UpdateColumn("failed_attempts", gorm.Expr("failed_attempts + 1")).Error; err != nil {
			return err
		}
		if err := tx.First(&current, t.Id).Error; err != nil {
			return err
		}
		if current.FailedAttempts >= common.MaxFailAttempts {
			return tx.Model(&current).Update("locked_until", time.Now().Add(time.Duration(common.LockoutDuration)*time.Second)).Error
		}
		return nil
	})
}

// IsLocked 检查账户是否被锁定
func (t *TwoFA) IsLocked() bool {
	if t.LockedUntil == nil {
		return false
	}
	return time.Now().Before(*t.LockedUntil)
}

// CreateBackupCodes 创建备用码
func CreateBackupCodes(userId int, codes []string) error {
	return DB.Transaction(func(tx *gorm.DB) error {
		// 先删除现有的备用码
		if err := tx.Where("user_id = ?", userId).Delete(&TwoFABackupCode{}).Error; err != nil {
			return err
		}

		// 创建新的备用码记录
		for _, code := range codes {
			hashedCode, err := common.HashBackupCode(code)
			if err != nil {
				return err
			}

			backupCode := TwoFABackupCode{
				UserId:   userId,
				CodeHash: hashedCode,
				IsUsed:   false,
			}

			if err := tx.Create(&backupCode).Error; err != nil {
				return err
			}
		}

		return nil
	})
}

// ValidateBackupCode 验证并使用备用码
func ValidateBackupCode(userId int, code string) (bool, error) {
	if !common.ValidateBackupCode(code) {
		return false, errors.New("验证码或备用码不正确")
	}

	normalizedCode := common.NormalizeBackupCode(code)

	// 查找未使用的备用码
	var backupCodes []TwoFABackupCode
	if err := DB.Where("user_id = ? AND is_used = false", userId).Find(&backupCodes).Error; err != nil {
		return false, err
	}

	// 验证备用码
	for _, bc := range backupCodes {
		if common.ValidatePasswordAndHash(normalizedCode, bc.CodeHash) {
			result := DB.Model(&TwoFABackupCode{}).Where("id = ? AND is_used = ?", bc.Id, false).Updates(map[string]any{"is_used": true, "used_at": time.Now()})
			return result.RowsAffected == 1, result.Error
		}
	}

	return false, nil
}

// GetUnusedBackupCodeCount 获取未使用的备用码数量
func GetUnusedBackupCodeCount(userId int) (int, error) {
	var count int64
	err := DB.Model(&TwoFABackupCode{}).Where("user_id = ? AND is_used = false", userId).Count(&count).Error
	return int(count), err
}

// DisableTwoFA 禁用用户的2FA
func DisableTwoFA(userId int) error {
	twoFA, err := GetTwoFAByUserId(userId)
	if err != nil {
		return err
	}
	if twoFA == nil {
		return ErrTwoFANotEnabled
	}

	// 删除2FA设置和备用码
	return twoFA.Delete()
}

// EnableTwoFA 启用2FA
func (t *TwoFA) Enable() error {
	return DB.Transaction(func(tx *gorm.DB) error {
		var user User
		if err := LockAuthenticationUser(tx, t.UserId, &user); err != nil {
			return err
		}
		result := tx.Model(&TwoFA{}).Where("id = ? AND secret = ? AND is_enabled = ?", t.Id, t.Secret, false).Updates(map[string]any{"is_enabled": true, "failed_attempts": 0, "locked_until": nil})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ErrSecurityProof
		}
		return RevokeUserAuthentication(tx, t.UserId)
	})
}

// ValidateTOTPAndUpdateUsage 验证TOTP并更新使用记录
func (t *TwoFA) ValidateTOTPAndUpdateUsage(code string) (bool, error) {
	// 检查是否被锁定
	if t.IsLocked() {
		return false, fmt.Errorf("账户已被锁定，请在%v后重试", t.LockedUntil.Format("2006-01-02 15:04:05"))
	}

	// 验证TOTP码
	step, valid := common.TOTPTimeStep(t.Secret, code, time.Now())
	if !valid {
		// 增加失败次数
		if err := t.IncrementFailedAttempts(); err != nil {
			common.SysLog("更新2FA失败次数失败: " + err.Error())
		}
		return false, nil
	}

	result := DB.Model(&TwoFA{}).Where("id = ? AND is_enabled = ? AND last_verified_step < ?", t.Id, true, step).
		Where("locked_until IS NULL OR locked_until <= ?", time.Now()).
		Updates(map[string]any{"last_verified_step": step, "last_used_at": time.Now(), "failed_attempts": 0, "locked_until": nil})
	return result.RowsAffected == 1, result.Error
}

// ValidateBackupCodeAndUpdateUsage 验证备用码并更新使用记录
func (t *TwoFA) ValidateBackupCodeAndUpdateUsage(code string) (bool, error) {
	// 检查是否被锁定
	if t.IsLocked() {
		return false, fmt.Errorf("账户已被锁定，请在%v后重试", t.LockedUntil.Format("2006-01-02 15:04:05"))
	}

	// 验证备用码
	valid, err := ValidateBackupCode(t.UserId, code)
	if err != nil {
		return false, err
	}

	if !valid {
		// 增加失败次数
		if err := t.IncrementFailedAttempts(); err != nil {
			common.SysLog("更新2FA失败次数失败: " + err.Error())
		}
		return false, nil
	}

	result := DB.Model(&TwoFA{}).Where("id = ? AND is_enabled = ?", t.Id, true).Updates(map[string]any{"failed_attempts": 0, "locked_until": nil, "last_used_at": time.Now()})
	return result.RowsAffected == 1, result.Error
}

// GetTwoFAStats 获取2FA统计信息（管理员使用）
func GetTwoFAStats() (map[string]interface{}, error) {
	var totalUsers, enabledUsers int64

	// 总用户数
	if err := DB.Model(&User{}).Count(&totalUsers).Error; err != nil {
		return nil, err
	}

	// 启用2FA的用户数
	if err := DB.Model(&TwoFA{}).Where("is_enabled = true").Count(&enabledUsers).Error; err != nil {
		return nil, err
	}

	enabledRate := float64(0)
	if totalUsers > 0 {
		enabledRate = float64(enabledUsers) / float64(totalUsers) * 100
	}

	return map[string]interface{}{
		"total_users":   totalUsers,
		"enabled_users": enabledUsers,
		"enabled_rate":  fmt.Sprintf("%.1f%%", enabledRate),
	}, nil
}
