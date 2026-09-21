package model

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

const (
	InvoiceStatusPending  = "pending"
	InvoiceStatusApproved = "approved"
	InvoiceStatusRejected = "rejected"
)

// InvoiceApplication is a user request for a manually issued invoice. The
// platform never generates an invoice file; it records the request and lets
// the operator review it offline.
type InvoiceApplication struct {
	Id          int                       `json:"id"`
	UserId      int                       `json:"user_id" gorm:"index;not null"`
	SubjectType string                    `json:"subject_type" gorm:"type:varchar(32);not null"`
	Title       string                    `json:"title" gorm:"type:varchar(255);not null"`
	TaxpayerId  string                    `json:"taxpayer_id" gorm:"type:varchar(64);not null;default:''"`
	Email       string                    `json:"email" gorm:"type:varchar(255);not null"`
	TotalAmount float64                   `json:"total_amount" gorm:"not null;default:0"`
	Currency    string                    `json:"currency" gorm:"type:varchar(8);not null;default:'CNY'"`
	Status      string                    `json:"status" gorm:"type:varchar(16);index;not null"`
	Remark      string                    `json:"remark" gorm:"type:varchar(255);not null;default:''"`
	HandlerId   int                       `json:"handler_id"`
	HandlerName string                    `json:"handler_name" gorm:"type:varchar(64);not null;default:''"`
	HandledAt   int64                     `json:"handled_at" gorm:"bigint"`
	CreatedAt   int64                     `json:"created_at" gorm:"bigint;index"`
	UpdatedAt   int64                     `json:"updated_at" gorm:"bigint"`
	Orders      []InvoiceApplicationOrder `json:"orders,omitempty" gorm:"foreignKey:InvoiceApplicationId"`
}

// InvoiceApplicationOrder snapshots the amount/currency at application time
// so later payment/order edits cannot change the requested invoice total.
// TopUpId is unique: a successful recharge order can only be invoiced once.
type InvoiceApplicationOrder struct {
	Id                   int     `json:"id"`
	InvoiceApplicationId int     `json:"invoice_application_id" gorm:"index;not null"`
	TopUpId              int     `json:"top_up_id" gorm:"uniqueIndex;not null"`
	TradeNo              string  `json:"trade_no" gorm:"type:varchar(255);not null"`
	Amount               float64 `json:"amount" gorm:"not null;default:0"`
	Currency             string  `json:"currency" gorm:"type:varchar(8);not null;default:'CNY'"`
	PaymentMethod        string  `json:"payment_method" gorm:"type:varchar(64);not null;default:''"`
	PaymentTime          int64   `json:"payment_time" gorm:"bigint"`
}

type InvoiceEligibleTopUp struct {
	Id            int     `json:"id"`
	TradeNo       string  `json:"trade_no"`
	Amount        float64 `json:"amount"`
	Currency      string  `json:"currency"`
	PaymentMethod string  `json:"payment_method"`
	PaymentTime   int64   `json:"payment_time"`
}

func (InvoiceApplication) TableName() string { return "invoice_applications" }

func (InvoiceApplicationOrder) TableName() string { return "invoice_application_orders" }

func invoiceTopUpAmount(topUp *TopUp) (float64, string, bool) {
	if topUp == nil {
		return 0, "", false
	}
	if topUp.ActualPaymentAmountMinor > 0 && strings.TrimSpace(topUp.ActualPaymentCurrency) != "" {
		amount := decimal.NewFromInt(topUp.ActualPaymentAmountMinor).Div(decimal.NewFromInt(100)).InexactFloat64()
		return amount, strings.ToUpper(strings.TrimSpace(topUp.ActualPaymentCurrency)), amount > 0
	}
	if topUp.ExpectedPaymentAmountMinor > 0 && strings.TrimSpace(topUp.ExpectedPaymentCurrency) != "" {
		amount := decimal.NewFromInt(topUp.ExpectedPaymentAmountMinor).Div(decimal.NewFromInt(100)).InexactFloat64()
		return amount, strings.ToUpper(strings.TrimSpace(topUp.ExpectedPaymentCurrency)), amount > 0
	}
	if topUp.Money <= 0 {
		return 0, "", false
	}
	currency := "CNY"
	switch topUp.PaymentProvider {
	case PaymentProviderStripe, PaymentProviderCreem, PaymentProviderWaffo, PaymentProviderWaffoPancake:
		currency = "USD"
	}
	return topUp.Money, currency, true
}

// ListEligibleInvoiceTopUps returns all successful externally paid top-ups
// that have not been claimed by any invoice application. It deliberately does
// not reuse the wallet's 30-day history window: invoice requests may concern
// older paid orders.
func ListEligibleInvoiceTopUps(userId int) ([]*InvoiceEligibleTopUp, error) {
	if userId <= 0 {
		return nil, errors.New("无效的用户")
	}
	var topUps []TopUp
	if err := DB.Where("user_id = ? AND status = ? AND amount > 0 AND money > 0 AND LOWER(TRIM(COALESCE(payment_provider, ''))) <> ? AND LOWER(TRIM(COALESCE(payment_method, ''))) <> ?", userId, common.TopUpStatusSuccess, PaymentProviderBalance, PaymentMethodBalance).
		Order("complete_time desc, id desc").Limit(5000).Find(&topUps).Error; err != nil {
		return nil, err
	}
	if len(topUps) == 0 {
		return []*InvoiceEligibleTopUp{}, nil
	}
	ids := make([]int, 0, len(topUps))
	for _, topUp := range topUps {
		ids = append(ids, topUp.Id)
	}
	var used []int
	if err := DB.Model(&InvoiceApplicationOrder{}).Where("top_up_id IN ?", ids).Pluck("top_up_id", &used).Error; err != nil {
		return nil, err
	}
	usedSet := make(map[int]struct{}, len(used))
	for _, id := range used {
		usedSet[id] = struct{}{}
	}
	result := make([]*InvoiceEligibleTopUp, 0, len(topUps))
	for index := range topUps {
		topUp := &topUps[index]
		if _, exists := usedSet[topUp.Id]; exists {
			continue
		}
		amount, currency, ok := invoiceTopUpAmount(topUp)
		if !ok {
			continue
		}
		paymentTime := topUp.CompleteTime
		if paymentTime == 0 {
			paymentTime = topUp.CreateTime
		}
		result = append(result, &InvoiceEligibleTopUp{Id: topUp.Id, TradeNo: topUp.TradeNo, Amount: amount, Currency: currency, PaymentMethod: topUp.PaymentMethod, PaymentTime: paymentTime})
	}
	return result, nil
}

func GetUserInvoiceApplications(userId int) ([]*InvoiceApplication, error) {
	var applications []*InvoiceApplication
	err := DB.Preload("Orders").Where("user_id = ?", userId).Order("id desc").Find(&applications).Error
	return applications, err
}

func GetInvoiceApplicationById(id int) (*InvoiceApplication, error) {
	var application InvoiceApplication
	if err := DB.Preload("Orders").First(&application, id).Error; err != nil {
		return nil, err
	}
	return &application, nil
}

func GetAllInvoiceApplications(status string, page, pageSize int) ([]*InvoiceApplication, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	query := DB.Model(&InvoiceApplication{})
	if strings.TrimSpace(status) != "" && status != "all" {
		query = query.Where("status = ?", status)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var applications []*InvoiceApplication
	if err := query.Preload("Orders").Order("id desc").Offset((page - 1) * pageSize).Limit(pageSize).Find(&applications).Error; err != nil {
		return nil, 0, err
	}
	return applications, total, nil
}

// CreateInvoiceApplication atomically validates and claims every selected
// top-up. The row locks prevent two concurrent browser tabs from invoicing
// the same order.
func CreateInvoiceApplication(userId int, subjectType, title, taxpayerId, email string, topUpIds []int) (*InvoiceApplication, error) {
	if userId <= 0 || len(topUpIds) == 0 {
		return nil, errors.New("请选择至少一笔充值订单")
	}
	if len(topUpIds) > 100 {
		return nil, errors.New("一次最多选择 100 笔充值订单")
	}
	cleanIds := make([]int, 0, len(topUpIds))
	seen := make(map[int]struct{}, len(topUpIds))
	for _, id := range topUpIds {
		if id <= 0 {
			return nil, errors.New("充值订单参数无效")
		}
		if _, exists := seen[id]; exists {
			return nil, errors.New("充值订单不能重复选择")
		}
		seen[id] = struct{}{}
		cleanIds = append(cleanIds, id)
	}
	application := &InvoiceApplication{UserId: userId, SubjectType: subjectType, Title: title, TaxpayerId: taxpayerId, Email: email, Status: InvoiceStatusPending}
	err := DB.Transaction(func(tx *gorm.DB) error {
		var topUps []TopUp
		if err := tx.Set("gorm:query_option", "FOR UPDATE").Where("id IN ? AND user_id = ? AND status = ? AND amount > 0 AND money > 0 AND LOWER(TRIM(COALESCE(payment_provider, ''))) <> ? AND LOWER(TRIM(COALESCE(payment_method, ''))) <> ?", cleanIds, userId, common.TopUpStatusSuccess, PaymentProviderBalance, PaymentMethodBalance).Find(&topUps).Error; err != nil {
			return err
		}
		if len(topUps) != len(cleanIds) {
			return errors.New("存在不可开票或不属于当前用户的充值订单")
		}
		orders := make([]InvoiceApplicationOrder, 0, len(topUps))
		currency := ""
		total := decimal.Zero
		for index := range topUps {
			topUp := &topUps[index]
			amount, orderCurrency, ok := invoiceTopUpAmount(topUp)
			if !ok {
				return errors.New("充值订单缺少有效支付金额")
			}
			if currency == "" {
				currency = orderCurrency
			} else if currency != orderCurrency {
				return errors.New("不同币种的充值订单不能合并开票")
			}
			var claimed int64
			if err := tx.Model(&InvoiceApplicationOrder{}).Where("top_up_id = ?", topUp.Id).Count(&claimed).Error; err != nil {
				return err
			}
			if claimed > 0 {
				return errors.New("所选充值订单已有发票申请")
			}
			paymentTime := topUp.CompleteTime
			if paymentTime == 0 {
				paymentTime = topUp.CreateTime
			}
			orders = append(orders, InvoiceApplicationOrder{TopUpId: topUp.Id, TradeNo: topUp.TradeNo, Amount: amount, Currency: orderCurrency, PaymentMethod: topUp.PaymentMethod, PaymentTime: paymentTime})
			total = total.Add(decimal.NewFromFloat(amount))
		}
		application.TotalAmount = total.Round(2).InexactFloat64()
		application.Currency = currency
		application.CreatedAt = common.GetTimestamp()
		application.UpdatedAt = application.CreatedAt
		if err := tx.Create(application).Error; err != nil {
			return err
		}
		for index := range orders {
			orders[index].InvoiceApplicationId = application.Id
		}
		if err := tx.Create(&orders).Error; err != nil {
			return err
		}
		application.Orders = orders
		return nil
	})
	if err != nil {
		return nil, err
	}
	return application, nil
}

func ReviewInvoiceApplication(id int, status string, handlerId int, handlerName, remark string) (*InvoiceApplication, error) {
	if id <= 0 {
		return nil, errors.New("发票申请不存在")
	}
	if status != InvoiceStatusApproved && status != InvoiceStatusRejected {
		return nil, errors.New("无效的发票审核状态")
	}
	var application InvoiceApplication
	err := DB.Transaction(func(tx *gorm.DB) error {
		now := common.GetTimestamp()
		result := tx.Model(&InvoiceApplication{}).
			Where("id = ? AND status = ?", id, InvoiceStatusPending).
			Updates(map[string]interface{}{
				"status":       status,
				"handler_id":   handlerId,
				"handler_name": handlerName,
				"remark":       strings.TrimSpace(remark),
				"handled_at":   now,
				"updated_at":   now,
			})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			var current InvoiceApplication
			if err := tx.First(&current, id).Error; err != nil {
				return err
			}
			return errors.New("该发票申请已处理")
		}
		return tx.First(&application, id).Error
	})
	if err != nil {
		return nil, err
	}
	return GetInvoiceApplicationById(application.Id)
}

func FormatInvoiceApplicationSummary(application *InvoiceApplication) string {
	if application == nil {
		return ""
	}
	parts := make([]string, 0, len(application.Orders))
	for _, order := range application.Orders {
		parts = append(parts, fmt.Sprintf("%s %.2f %s", order.TradeNo, order.Amount, order.Currency))
	}
	return strings.Join(parts, "; ")
}

func invoiceTimestamp(value int64) time.Time {
	return time.Unix(value, 0)
}
