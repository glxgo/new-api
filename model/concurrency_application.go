package model

import (
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	ConcurrencyApplicationPending  = "pending"
	ConcurrencyApplicationApproved = "approved"
	ConcurrencyApplicationRejected = "rejected"
)

// ConcurrencyApplication stores a user's request to increase their account
// concurrency. Contact details are visible only to the applicant and admins.
type ConcurrencyApplication struct {
	Id             int    `json:"id"`
	UserId         int    `json:"user_id" gorm:"not null;index;column:user_id"`
	Username       string `json:"username" gorm:"column:username;->;-:migration"`
	CurrentLimit   int    `json:"current_limit" gorm:"not null;column:current_limit"`
	RequestedLimit int    `json:"requested_limit" gorm:"not null;column:requested_limit"`
	Reason         string `json:"reason" gorm:"type:text;not null"`
	Contact        string `json:"contact" gorm:"type:varchar(120);not null"`
	Status         string `json:"status" gorm:"type:varchar(16);not null;default:'pending';index"`
	AdminNote      string `json:"admin_note" gorm:"type:varchar(500);not null;default:'';column:admin_note"`
	ReviewerId     int    `json:"reviewer_id" gorm:"not null;default:0;column:reviewer_id"`
	CreatedAt      int64  `json:"created_at" gorm:"not null;autoCreateTime;column:created_at"`
	UpdatedAt      int64  `json:"updated_at" gorm:"not null;autoUpdateTime;column:updated_at"`
	ReviewedAt     int64  `json:"reviewed_at" gorm:"not null;default:0;column:reviewed_at"`
}

func CreateConcurrencyApplication(userId, requestedLimit int, reason, contact string) (*ConcurrencyApplication, error) {
	reason = strings.TrimSpace(reason)
	contact = strings.TrimSpace(contact)
	var application *ConcurrencyApplication
	err := DB.Transaction(func(tx *gorm.DB) error {
		var user User
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&user, userId).Error; err != nil {
			return err
		}
		currentLimit := user.EffectiveConcurrencyLimit()
		if requestedLimit <= currentLimit {
			return errors.New("requested concurrency must be greater than current limit")
		}
		var count int64
		if err := tx.Model(&ConcurrencyApplication{}).
			Where("user_id = ? AND status = ?", userId, ConcurrencyApplicationPending).
			Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			return errors.New("a concurrency application is already pending")
		}
		application = &ConcurrencyApplication{
			UserId:         userId,
			CurrentLimit:   currentLimit,
			RequestedLimit: requestedLimit,
			Reason:         reason,
			Contact:        contact,
			Status:         ConcurrencyApplicationPending,
		}
		return tx.Create(application).Error
	})
	return application, err
}

func ListConcurrencyApplications(userId int, status string, offset, limit int) ([]ConcurrencyApplication, int64, error) {
	query := DB.Model(&ConcurrencyApplication{})
	if userId > 0 {
		query = query.Where("concurrency_applications.user_id = ?", userId)
	}
	if status != "" {
		query = query.Where("concurrency_applications.status = ?", status)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var applications []ConcurrencyApplication
	err := query.Select("concurrency_applications.*, users.username AS username").
		Joins("LEFT JOIN users ON users.id = concurrency_applications.user_id").
		Order("concurrency_applications.id DESC").Offset(offset).Limit(limit).Scan(&applications).Error
	return applications, total, err
}

func ReviewConcurrencyApplication(id, reviewerId int, approve bool, approvedLimit int, adminNote string) (*ConcurrencyApplication, error) {
	var application ConcurrencyApplication
	err := DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&application, id).Error; err != nil {
			return err
		}
		if application.Status != ConcurrencyApplicationPending {
			return errors.New("application has already been reviewed")
		}
		status := ConcurrencyApplicationRejected
		if approve {
			status = ConcurrencyApplicationApproved
			if approvedLimit == 0 {
				approvedLimit = application.RequestedLimit
			}
			if approvedLimit < 1 || approvedLimit > 10000 {
				return errors.New("approved concurrency is out of range")
			}
			if err := tx.Model(&User{}).Where("id = ?", application.UserId).
				Updates(map[string]interface{}{
					"concurrency_limit":          approvedLimit,
					"concurrency_limit_override": true,
				}).Error; err != nil {
				return err
			}
		}
		now := time.Now().Unix()
		if err := tx.Model(&application).Updates(map[string]interface{}{
			"status": status, "admin_note": strings.TrimSpace(adminNote),
			"reviewer_id": reviewerId, "reviewed_at": now, "updated_at": now,
		}).Error; err != nil {
			return err
		}
		application.Status = status
		application.AdminNote = strings.TrimSpace(adminNote)
		application.ReviewerId = reviewerId
		application.ReviewedAt = now
		return nil
	})
	if err == nil && approve {
		_ = InvalidateUserCache(application.UserId)
	}
	return &application, err
}
