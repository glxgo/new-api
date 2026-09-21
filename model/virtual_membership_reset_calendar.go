package model

import (
	"errors"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

// VirtualMembershipResetCalendarEntry is an operator-authored calendar fact.
// It is intentionally independent from active reset requests and quota
// ledgers: the displayed count is exactly the count entered by an operator.
type VirtualMembershipResetCalendarEntry struct {
	Id        int    `json:"id"`
	ResetAt   int64  `json:"reset_at" gorm:"bigint;index;not null"`
	Count     int    `json:"count" gorm:"not null;default:1"`
	Reason    string `json:"reason" gorm:"type:varchar(500);not null"`
	CreatedAt int64  `json:"created_at" gorm:"bigint"`
	UpdatedAt int64  `json:"updated_at" gorm:"bigint"`
}

func (VirtualMembershipResetCalendarEntry) TableName() string {
	return "virtual_membership_reset_calendar_entries"
}

func ListVirtualMembershipResetCalendarEntries(startAt, endAt int64) ([]VirtualMembershipResetCalendarEntry, int, error) {
	if startAt <= 0 || endAt <= startAt {
		return nil, 0, errors.New("日期范围无效")
	}
	var entries []VirtualMembershipResetCalendarEntry
	if err := DB.Where("reset_at >= ? AND reset_at < ?", startAt, endAt).Order("reset_at asc, id asc").Find(&entries).Error; err != nil {
		return nil, 0, err
	}
	total := 0
	for _, entry := range entries {
		total += entry.Count
	}
	return entries, total, nil
}

func GetVirtualMembershipResetCalendarEntry(id int) (*VirtualMembershipResetCalendarEntry, error) {
	var entry VirtualMembershipResetCalendarEntry
	if err := DB.First(&entry, id).Error; err != nil {
		return nil, err
	}
	return &entry, nil
}

func SaveVirtualMembershipResetCalendarEntry(entry *VirtualMembershipResetCalendarEntry) error {
	if entry == nil || entry.ResetAt <= 0 {
		return errors.New("重置时间无效")
	}
	if entry.Count <= 0 || entry.Count > 100000 {
		return errors.New("重置次数必须是 1 到 100000 的整数")
	}
	entry.Reason = strings.TrimSpace(entry.Reason)
	if entry.Reason == "" {
		return errors.New("请填写重置理由")
	}
	now := common.GetTimestamp()
	if entry.CreatedAt == 0 {
		entry.CreatedAt = now
	}
	entry.UpdatedAt = now
	return DB.Save(entry).Error
}

func DeleteVirtualMembershipResetCalendarEntry(id int) error {
	if id <= 0 {
		return errors.New("重置记录不存在")
	}
	result := DB.Delete(&VirtualMembershipResetCalendarEntry{}, id)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}
