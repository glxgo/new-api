package model

import (
	"github.com/QuantumNous/new-api/common"

	"github.com/shopspring/decimal"
)

// Withdraw 提现申请记录。普通用户提现本金(Type=1), 管理员/超管提现分红(Type=2)。
type Withdraw struct {
	Id            int    `json:"id" gorm:"primaryKey;autoIncrement"`
	UserId        int    `json:"user_id" gorm:"index;not null"`
	Type          int    `json:"type" gorm:"not null"` // 1 本金 2 分红
	Amount        int    `json:"amount" gorm:"not null"`        // 申请提现金额(quota 单位)
	Fee           int    `json:"fee" gorm:"not null"`           // 手续费(quota 单位)
	ActualAmount  int    `json:"actual_amount" gorm:"not null"` // 实际到账 = Amount - Fee
	Status        int    `json:"status" gorm:"index;not null"`  // 0 待审 1 通过 2 拒绝
	HandlerId     int    `json:"handler_id"`
	HandlerName   string `json:"handler_name" gorm:"type:varchar(64)"`
	AlipayName    string `json:"alipay_name" gorm:"type:varchar(64)"`     // 普通用户本金提现必填
	AlipayAccount string `json:"alipay_account" gorm:"type:varchar(128)"` // 普通用户本金提现必填
	WechatQrcode  string `json:"wechat_qrcode" gorm:"type:longtext"`      // base64, 备用收款码(longtext 容纳压缩后图片)
	Remark        string `json:"remark" gorm:"type:varchar(255)"`        // 审核备注
	CreatedAt     int64  `json:"created_at" gorm:"bigint;index"`
	HandledAt     int64  `json:"handled_at" gorm:"bigint"`
}

func (Withdraw) TableName() string {
	return "withdraws"
}

const (
	WithdrawStatusPending  = 0
	WithdrawStatusApproved = 1
	WithdrawStatusRejected = 2
	WithdrawTypePrincipal  = 1 // 本金(可提)
	WithdrawTypeDividend   = 2 // 分红(管理员/超管)
)

func CreateWithdraw(w *Withdraw) error {
	if w.CreatedAt == 0 {
		w.CreatedAt = common.GetTimestamp()
	}
	return DB.Create(w).Error
}

func GetWithdrawById(id int) (*Withdraw, error) {
	var w Withdraw
	err := DB.Where("id = ?", id).First(&w).Error
	if err != nil {
		return nil, err
	}
	return &w, nil
}

func GetUserWithdraws(userId int, page, pageSize int) ([]*Withdraw, int64, error) {
	var ws []*Withdraw
	var total int64
	DB.Model(&Withdraw{}).Where("user_id = ?", userId).Count(&total)
	err := DB.Where("user_id = ?", userId).Order("id desc").Offset((page - 1) * pageSize).Limit(pageSize).Find(&ws).Error
	return ws, total, err
}

func GetAllWithdraws(status int, page, pageSize int) ([]*Withdraw, int64, error) {
	var ws []*Withdraw
	var total int64
	tx := DB.Model(&Withdraw{})
	if status >= 0 {
		tx = tx.Where("status = ?", status)
	}
	tx.Count(&total)
	err := tx.Order("id desc").Offset((page - 1) * pageSize).Limit(pageSize).Find(&ws).Error
	return ws, total, err
}

// FinishWithdraw 审核完成(通过/拒绝), 原子更新(WHERE status=Pending 防并发重复审核), 记录审核人+备注+时间。
func FinishWithdraw(id, status, handlerId int, handlerName, remark string) error {
	return DB.Model(&Withdraw{}).Where("id = ? AND status = ?", id, WithdrawStatusPending).
		Updates(map[string]interface{}{
			"status":       status,
			"handler_id":   handlerId,
			"handler_name": handlerName,
			"remark":       remark,
			"handled_at":   common.GetTimestamp(),
		}).Error
}

// CalcWithdrawFee 手续费 = amount × feeRate(默认 0.05), decimal 精算取整。
func CalcWithdrawFee(amount int, feeRate float64) int {
	if amount <= 0 || feeRate <= 0 {
		return 0
	}
	d := decimal.NewFromInt(int64(amount)).Mul(decimal.NewFromFloat(feeRate))
	return int(d.Round(0).IntPart())
}
