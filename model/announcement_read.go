package model

import (
	"errors"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type UserAnnouncementRead struct {
	Id             int   `json:"id" gorm:"primaryKey;autoIncrement"`
	UserId         int   `json:"user_id" gorm:"uniqueIndex:idx_user_announcement_read,priority:1;index"`
	AnnouncementId int64 `json:"announcement_id" gorm:"uniqueIndex:idx_user_announcement_read,priority:2;index"`
	ReadAt         int64 `json:"read_at" gorm:"bigint"`
}

func GetReadAnnouncementIds(userId int) (map[int64]struct{}, error) {
	result := map[int64]struct{}{}
	if userId <= 0 {
		return result, nil
	}
	var ids []int64
	if err := DB.Model(&UserAnnouncementRead{}).Where("user_id = ?", userId).
		Pluck("announcement_id", &ids).Error; err != nil {
		return nil, err
	}
	for _, id := range ids {
		result[id] = struct{}{}
	}
	return result, nil
}

func MarkAnnouncementsRead(userId int, announcementIds []int64) error {
	if userId <= 0 {
		return errors.New("用户不存在")
	}
	rows := make([]UserAnnouncementRead, 0, len(announcementIds))
	seen := map[int64]struct{}{}
	for _, id := range announcementIds {
		if id <= 0 {
			continue
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		rows = append(rows, UserAnnouncementRead{UserId: userId, AnnouncementId: id, ReadAt: common.GetTimestamp()})
	}
	if len(rows) == 0 {
		return nil
	}
	return DB.Clauses(clause.OnConflict{DoNothing: true}).CreateInBatches(rows, 100).Error
}

func DeleteAnnouncementReadHistory(announcementIds []int64) error {
	if len(announcementIds) == 0 || DB == nil {
		return nil
	}
	result := DB.Where("announcement_id IN ?", announcementIds).Delete(&UserAnnouncementRead{})
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return nil
	}
	return result.Error
}
