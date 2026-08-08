package model

// DividendRecord 分润发放明细(T+1 结算审计用)。钱包消费按
// 「收款人 + 消费用户 + 结算日 + 类型」聚合为一条，订单分润仍按订单记录。
type DividendRecord struct {
	Id                  int    `json:"id" gorm:"primaryKey;autoIncrement"`
	BatchId             string `json:"batch_id" gorm:"type:varchar(40);index;not null"`              // 结算批次, 如 "2026-06-16"
	UserId              int    `json:"user_id" gorm:"index;not null"`                                // 收款用户(分润归属)
	SourceUserId        int    `json:"source_user_id" gorm:"index;not null"`                         // 产生消费的用户
	LogId               int    `json:"log_id" gorm:"index;not null"`                                 // 兼容旧记录；按日聚合记录为 0
	Type                int    `json:"type" gorm:"not null"`                                         // 见 DividendType* 常量
	GrossProfit         int    `json:"gross_profit" gorm:"not null"`                                 // 当日该用户产生的毛利(quota 单位)
	Amount              int    `json:"amount" gorm:"not null"`                                       // 当日该用户产生的返利(quota 单位)
	SourceUsage         int    `json:"source_usage" gorm:"not null;default:0"`                       // 当日该用户总用量(quota 单位)
	SourceRechargeCents int64  `json:"source_recharge_cents" gorm:"not null;default:0"`              // 当日该用户实际充值金额(分)
	RequestCount        int    `json:"request_count" gorm:"not null;default:0"`                      // 聚合的消费请求数
	SourceRef           string `json:"source_ref" gorm:"type:varchar(64);index;not null;default:''"` // 幂等/聚合键
	CreatedAt           int64  `json:"created_at" gorm:"bigint"`
}

func (DividendRecord) TableName() string {
	return "dividend_records"
}

// 分润类型常量
const (
	DividendTypeDirect   = 1 // 拉新返利 - 直接上级(毛利 × AffiliateDirectRate)
	DividendTypeIndirect = 2 // 拉新返利 - 间接上级(毛利 × AffiliateIndirectRate)
	DividendTypeAdmin    = 3 // 管理员分红(树顶管理员, 毛利 × DividendRate, 上限 MaxDividendRate)
	DividendTypeRoot     = 4 // 超管分红(毛利 × RootDividendRate)
)

// BatchInsertDividendRecords 批量插入分润明细。
func BatchInsertDividendRecords(records []*DividendRecord) error {
	if len(records) == 0 {
		return nil
	}
	return DB.CreateInBatches(records, 500).Error
}

// HasDividendRecordBySourceRef 幂等检查: 该 sourceRef(如订单号)是否已发过分润。防 webhook 重放重复发放。
func HasDividendRecordBySourceRef(sourceRef string) (bool, error) {
	if sourceRef == "" {
		return false, nil
	}
	var count int64
	err := DB.Model(&DividendRecord{}).Where("source_ref = ?", sourceRef).Count(&count).Error
	return count > 0, err
}

// GetDividendRecordsByRecipient 用户收到的返利明细(仅 type=1,2 拉新返利), 分页倒序。
func GetDividendRecordsByRecipient(userId int, page, pageSize int) ([]*DividendRecord, int64, error) {
	var records []*DividendRecord
	var total int64
	tx := DB.Model(&DividendRecord{}).Where("user_id = ? AND type IN ?", userId, []int{DividendTypeDirect, DividendTypeIndirect})
	tx.Count(&total)
	err := tx.Order("id desc").Offset((page - 1) * pageSize).Limit(pageSize).Find(&records).Error
	return records, total, err
}

type AffiliateSourceSummary struct {
	SourceUserId  int
	RechargeCents int64
	Usage         int64
	GrossProfit   int64
	Rebate        int64
}

// GetAffiliateSourceSummaries 返回指定下级为某个收款用户累计产生的充值、用量、
// 已结算毛利和返利。充值取真实支付台账，用量取消费日志，毛利/返利取分润审计表。
func GetAffiliateSourceSummaries(recipientId int, sourceUserIds []int) (map[int]AffiliateSourceSummary, error) {
	result := make(map[int]AffiliateSourceSummary, len(sourceUserIds))
	if recipientId <= 0 || len(sourceUserIds) == 0 {
		return result, nil
	}
	for _, id := range sourceUserIds {
		result[id] = AffiliateSourceSummary{SourceUserId: id}
	}

	type rechargeRow struct {
		UserId int
		Amount int64
	}
	var recharges []rechargeRow
	if err := DB.Model(&RechargeCredit{}).
		Select("user_id, COALESCE(SUM(amount_cents), 0) AS amount").
		Where("user_id IN ?", sourceUserIds).
		Group("user_id").Scan(&recharges).Error; err != nil {
		return nil, err
	}
	for _, row := range recharges {
		summary := result[row.UserId]
		summary.RechargeCents = row.Amount
		result[row.UserId] = summary
	}

	type usageRow struct {
		UserId int
		Amount int64
	}
	var usages []usageRow
	if LOG_DB != nil {
		if err := LOG_DB.Model(&Log{}).
			Select("user_id, COALESCE(SUM(quota), 0) AS amount").
			Where("user_id IN ? AND type = ?", sourceUserIds, LogTypeConsume).
			Group("user_id").Scan(&usages).Error; err != nil {
			return nil, err
		}
	}
	for _, row := range usages {
		summary := result[row.UserId]
		summary.Usage = row.Amount
		result[row.UserId] = summary
	}

	type dividendRow struct {
		SourceUserId int
		GrossProfit  int64
		Rebate       int64
	}
	var dividends []dividendRow
	if err := DB.Model(&DividendRecord{}).
		Select("source_user_id, COALESCE(SUM(gross_profit), 0) AS gross_profit, COALESCE(SUM(amount), 0) AS rebate").
		Where("user_id = ? AND source_user_id IN ? AND type IN ?", recipientId, sourceUserIds, []int{DividendTypeDirect, DividendTypeIndirect}).
		Group("source_user_id").Scan(&dividends).Error; err != nil {
		return nil, err
	}
	for _, row := range dividends {
		summary := result[row.SourceUserId]
		summary.GrossProfit = row.GrossProfit
		summary.Rebate = row.Rebate
		result[row.SourceUserId] = summary
	}
	return result, nil
}

// GetRechargeCentsByUsersBetween 返回指定时间窗内的真实支付充值汇总。
func GetRechargeCentsByUsersBetween(userIds []int, start, end int64) (map[int]int64, error) {
	result := make(map[int]int64, len(userIds))
	if len(userIds) == 0 {
		return result, nil
	}
	type row struct {
		UserId int
		Amount int64
	}
	var rows []row
	err := DB.Model(&RechargeCredit{}).
		Select("user_id, COALESCE(SUM(amount_cents), 0) AS amount").
		Where("user_id IN ? AND created_at >= ? AND created_at < ?", userIds, start, end).
		Group("user_id").Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	for _, item := range rows {
		result[item.UserId] = item.Amount
	}
	return result, nil
}

// SumDividendByRecipient 用户累计收到的返利(quota)。
func SumDividendByRecipient(userId int) (int64, error) {
	var sum int64
	err := DB.Model(&DividendRecord{}).
		Where("user_id = ? AND type IN ?", userId, []int{DividendTypeDirect, DividendTypeIndirect}).
		Select("COALESCE(SUM(amount),0)").Scan(&sum).Error
	return sum, err
}

// SumDividendBySource 某下级(sourceUserId)为某上级(recipientId)产生的返利总额(quota)。
func SumDividendBySource(recipientId, sourceUserId int) (int64, error) {
	var sum int64
	err := DB.Model(&DividendRecord{}).
		Where("user_id = ? AND source_user_id = ? AND type IN ?", recipientId, sourceUserId, []int{DividendTypeDirect, DividendTypeIndirect}).
		Select("COALESCE(SUM(amount),0)").Scan(&sum).Error
	return sum, err
}
