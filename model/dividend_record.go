package model

// DividendRecord is the commission audit ledger. PolicyVersion=0 rows are
// immutable historical profit settlements; PolicyVersion=1 rows are generated
// from real paid recharge credits and always leave legacy profit fields at 0.
type DividendRecord struct {
	Id                  int     `json:"id" gorm:"primaryKey;autoIncrement"`
	BatchId             string  `json:"batch_id" gorm:"type:varchar(40);index;not null"`              // 结算批次, 如 "2026-06-16"
	UserId              int     `json:"user_id" gorm:"index;not null"`                                // 收款用户(分润归属)
	SourceUserId        int     `json:"source_user_id" gorm:"index;not null"`                         // 产生消费的用户
	LogId               int     `json:"log_id" gorm:"index;not null"`                                 // 兼容旧记录；按日聚合记录为 0
	Type                int     `json:"type" gorm:"not null"`                                         // 见 DividendType* 常量
	GrossProfit         int     `json:"gross_profit" gorm:"not null"`                                 // 历史兼容字段；新策略固定为 0
	Amount              int     `json:"amount" gorm:"not null"`                                       // 发放返利/分红(quota 单位)
	SourceUsage         int     `json:"source_usage" gorm:"not null;default:0"`                       // 历史兼容字段；新策略固定为 0
	SourceRechargeCents int64   `json:"source_recharge_cents" gorm:"not null;default:0"`              // 当日该用户实际充值金额(分)
	RequestCount        int     `json:"request_count" gorm:"not null;default:0"`                      // 聚合的消费请求数
	SourceRef           string  `json:"source_ref" gorm:"type:varchar(64);index;not null;default:''"` // 幂等/聚合键
	CommissionKey       *string `json:"-" gorm:"type:varchar(160);uniqueIndex"`                       // 新策略幂等键；历史记录为 NULL
	PolicyVersion       int     `json:"policy_version" gorm:"not null;default:0;index"`               // 0=历史利润结算, 1=固定充值比例
	CreatedAt           int64   `json:"created_at" gorm:"bigint"`
}

func (DividendRecord) TableName() string {
	return "dividend_records"
}

// 分润类型常量
const (
	DividendTypeDirect   = 1 // 普通用户直属 5%；代理直属 8%
	DividendTypeIndirect = 2 // 普通用户二级 2%；代理二级 4%
	DividendTypeAdmin    = 3 // 管理员直属 15%；管理员二级 5%
	DividendTypeRoot     = 4 // 超级管理员固定 5%
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
	tx := DB.Model(&DividendRecord{}).Where("user_id = ? AND type IN ? AND policy_version = ?", userId, []int{DividendTypeDirect, DividendTypeIndirect}, RechargeCommissionPolicyV1)
	tx.Count(&total)
	err := tx.Order("id desc").Offset((page - 1) * pageSize).Limit(pageSize).Find(&records).Error
	return records, total, err
}

type AffiliateSourceSummary struct {
	SourceUserId  int
	RechargeCents int64
	Rebate        int64
}

// GetAffiliateSourceSummaries 返回指定下级为某个收款用户累计产生的有效充值与返利。
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
		Where("user_id IN ? AND source_type IN ? AND commission_state = ? AND commission_policy_version = ?", sourceUserIds, []string{RechargeSourceWalletTopUp, RechargeSourceSubscription, RechargeSourceVirtualMembership, RechargeSourceAdmin}, RechargeCommissionDone, RechargeCommissionPolicyV1).
		Group("user_id").Scan(&recharges).Error; err != nil {
		return nil, err
	}
	for _, row := range recharges {
		summary := result[row.UserId]
		summary.RechargeCents = row.Amount
		result[row.UserId] = summary
	}

	type dividendRow struct {
		SourceUserId int
		Rebate       int64
	}
	var dividends []dividendRow
	if err := DB.Model(&DividendRecord{}).
		Select("source_user_id, COALESCE(SUM(amount), 0) AS rebate").
		Where("user_id = ? AND source_user_id IN ? AND type IN ? AND policy_version = ?", recipientId, sourceUserIds, []int{DividendTypeDirect, DividendTypeIndirect}, RechargeCommissionPolicyV1).
		Group("source_user_id").Scan(&dividends).Error; err != nil {
		return nil, err
	}
	for _, row := range dividends {
		summary := result[row.SourceUserId]
		summary.Rebate = row.Rebate
		result[row.SourceUserId] = summary
	}
	return result, nil
}

// GetRechargeCentsByUsersBetween 返回指定时间窗内计入累充的充值汇总。
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
		Where("user_id IN ? AND source_type IN ? AND commission_state = ? AND commission_policy_version = ? AND created_at >= ? AND created_at < ?", userIds, []string{RechargeSourceWalletTopUp, RechargeSourceSubscription, RechargeSourceVirtualMembership, RechargeSourceAdmin}, RechargeCommissionDone, RechargeCommissionPolicyV1, start, end).
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
		Where("user_id = ? AND type IN ? AND policy_version = ?", userId, []int{DividendTypeDirect, DividendTypeIndirect}, RechargeCommissionPolicyV1).
		Select("COALESCE(SUM(amount),0)").Scan(&sum).Error
	return sum, err
}

// SumDividendBySource 某下级(sourceUserId)为某上级(recipientId)产生的返利总额(quota)。
func SumDividendBySource(recipientId, sourceUserId int) (int64, error) {
	var sum int64
	err := DB.Model(&DividendRecord{}).
		Where("user_id = ? AND source_user_id = ? AND type IN ? AND policy_version = ?", recipientId, sourceUserId, []int{DividendTypeDirect, DividendTypeIndirect}, RechargeCommissionPolicyV1).
		Select("COALESCE(SUM(amount),0)").Scan(&sum).Error
	return sum, err
}
