package model

import "gorm.io/gorm"

// ClaimLowBalanceWarning 原子领取一次低余额提醒资格。只有数据库中的真实
// 钱包总余额(本金+赠金)低于阈值且当前充值周期仍处于 armed 状态时才成功。
// 成功后立即关闭资格，避免并发请求重复发送。
func ClaimLowBalanceWarning(userId, threshold int) (remaining int, claimed bool, err error) {
	if userId <= 0 || threshold <= 0 {
		return 0, false, nil
	}
	err = DB.Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&User{}).
			Where("id = ? AND low_balance_warning_armed = ? AND (quota + gift_quota) < ?", userId, true, threshold).
			Update("low_balance_warning_armed", false)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return nil
		}
		var balances struct {
			Quota     int
			GiftQuota int
		}
		if err := tx.Model(&User{}).Select("quota, gift_quota").Where("id = ?", userId).Take(&balances).Error; err != nil {
			return err
		}
		remaining = balances.Quota + balances.GiftQuota
		claimed = true
		return nil
	})
	return remaining, claimed, err
}

// RearmLowBalanceWarning 仅在提醒发送失败时恢复本次充值周期的提醒资格。
func RearmLowBalanceWarning(userId int) error {
	if userId <= 0 {
		return nil
	}
	return DB.Model(&User{}).Where("id = ?", userId).
		Update("low_balance_warning_armed", true).Error
}
