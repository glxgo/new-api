package model

import "github.com/QuantumNous/new-api/common"

// AffiliateSettle T+1 结算批次(幂等键 + 审计汇总)。每日一条, BatchId 为主键。
type AffiliateSettle struct {
	BatchId    string `json:"batch_id" gorm:"type:varchar(40);primaryKey"` // 如 "2026-06-16"
	DayStart   int64  `json:"day_start" gorm:"bigint;not null"`
	DayEnd     int64  `json:"day_end" gorm:"bigint;not null"`
	Status     int    `json:"status" gorm:"not null"` // 0 进行中 / 1 完成 / 2 失败
	LogCount   int    `json:"log_count" gorm:"not null"`
	TotalGross int    `json:"total_gross" gorm:"not null"` // 当日已结算总毛利(quota 单位)
	CreatedAt  int64  `json:"created_at" gorm:"bigint"`
	FinishedAt int64  `json:"finished_at" gorm:"bigint"`
}

func (AffiliateSettle) TableName() string {
	return "affiliate_settles"
}

const (
	AffiliateSettleStatusRunning = 0
	AffiliateSettleStatusDone    = 1
	AffiliateSettleStatusFailed  = 2
)

// GetAffiliateSettle 查询批次。不存在返回 (nil, error)。
func GetAffiliateSettle(batchId string) (*AffiliateSettle, error) {
	var s AffiliateSettle
	err := DB.Where("batch_id = ?", batchId).First(&s).Error
	if err != nil {
		return nil, err
	}
	return &s, nil
}

// CreateAffiliateSettle 创建批次记录(标记进行中)。
func CreateAffiliateSettle(s *AffiliateSettle) error {
	return DB.Create(s).Error
}

// FinishAffiliateSettle 标记批次完成并写入汇总。
func FinishAffiliateSettle(batchId string, logCount, totalGross int) error {
	return DB.Model(&AffiliateSettle{}).Where("batch_id = ?", batchId).
		Updates(map[string]interface{}{
			"status":      AffiliateSettleStatusDone,
			"log_count":   logCount,
			"total_gross": totalGross,
			"finished_at": common.GetTimestamp(),
		}).Error
}
