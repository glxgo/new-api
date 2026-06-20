package model

// DividendRecord 分润发放明细(T+1 结算审计用)。每笔分润一条记录, 可追溯谁从谁的消费中分得多少。
type DividendRecord struct {
	Id           int    `json:"id" gorm:"primaryKey;autoIncrement"`
	BatchId      string `json:"batch_id" gorm:"type:varchar(40);index;not null"` // 结算批次, 如 "2026-06-16"
	UserId       int    `json:"user_id" gorm:"index;not null"`                   // 收款用户(分润归属)
	SourceUserId int    `json:"source_user_id" gorm:"not null"`                  // 产生消费的用户
	LogId        int    `json:"log_id" gorm:"index;not null"`                    // 消费日志 id
	Type         int    `json:"type" gorm:"not null"`                            // 见 DividendType* 常量
	GrossProfit  int    `json:"gross_profit" gorm:"not null"`                    // 该笔毛利(quota 单位)
	Amount       int    `json:"amount" gorm:"not null"`                          // 分得金额(quota 单位)
	CreatedAt    int64  `json:"created_at" gorm:"bigint"`
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

// GetDividendRecordsByRecipient 用户收到的返利明细(仅 type=1,2 拉新返利), 分页倒序。
func GetDividendRecordsByRecipient(userId int, page, pageSize int) ([]*DividendRecord, int64, error) {
	var records []*DividendRecord
	var total int64
	tx := DB.Model(&DividendRecord{}).Where("user_id = ? AND type IN ?", userId, []int{DividendTypeDirect, DividendTypeIndirect})
	tx.Count(&total)
	err := tx.Order("id desc").Offset((page - 1) * pageSize).Limit(pageSize).Find(&records).Error
	return records, total, err
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
