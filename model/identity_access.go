package model

import (
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	"gorm.io/gorm"
)

// IdentityType is the small, mutually-exclusive set of operator-assigned
// identities used by the mainland web access exception.  It is deliberately
// kept separate from User so that the identity cannot be changed by the
// ordinary user update payload.
const (
	IdentityTypeNone       = "none"
	IdentityTypeEnterprise = "enterprise"
	IdentityTypeEducation  = "education"
)

const (
	MainlandIPAllowlistStatusActive  = "active"
	MainlandIPAllowlistStatusRevoked = "revoked"
	MainlandIPAllowlistSourceSelf    = "self"
	MainlandIPAllowlistSourceAdmin   = "admin"
	MainlandIPAllowlistMaxPerUser    = 10
)

var (
	ErrInvalidIdentityType = errors.New("invalid identity type")
	ErrIdentityRequired    = errors.New("enterprise or education identity is required")
	ErrWhitelistLimit      = errors.New("too many active mainland IP allowlist entries")
)

// UserIdentity stores the operator-granted identity and its audit metadata.
// A separate table prevents mass-assignment through model.User JSON updates.
type UserIdentity struct {
	ID                 int    `json:"id"`
	UserID             int    `json:"user_id" gorm:"not null;uniqueIndex"`
	IdentityType       string `json:"identity_type" gorm:"type:varchar(20);not null;default:'none';index"`
	IdentityVerifiedAt int64  `json:"identity_verified_at" gorm:"not null;default:0"`
	IdentityVerifiedBy int    `json:"identity_verified_by" gorm:"not null;default:0"`
	CreatedAt          int64  `json:"created_at" gorm:"not null;autoCreateTime"`
	UpdatedAt          int64  `json:"updated_at" gorm:"not null;autoUpdateTime"`
}

// MainlandIPAllowlist is an exact-address exception to the CN web block.
// PrefixLength is always 32 or 128; users cannot submit a CIDR or arbitrary
// address, and the source IP is resolved server-side from the trusted proxy
// chain.
type MainlandIPAllowlist struct {
	ID                   int    `json:"id"`
	UserID               int    `json:"user_id" gorm:"not null;index;uniqueIndex:idx_mainland_allowlist_address"`
	IdentityTypeSnapshot string `json:"identity_type_snapshot" gorm:"type:varchar(20);not null;index"`
	IP                   string `json:"ip" gorm:"type:varchar(64);not null;index;uniqueIndex:idx_mainland_allowlist_address"`
	AddressFamily        string `json:"address_family" gorm:"type:varchar(4);not null"`
	PrefixLength         int    `json:"prefix_length" gorm:"not null;uniqueIndex:idx_mainland_allowlist_address"`
	Source               string `json:"source" gorm:"type:varchar(20);not null;index"`
	Status               string `json:"status" gorm:"type:varchar(20);not null;index;uniqueIndex:idx_mainland_allowlist_address"`
	CreatedAt            int64  `json:"created_at" gorm:"not null;autoCreateTime"`
	LastSeenAt           int64  `json:"last_seen_at" gorm:"not null;default:0"`
	ExpiresAt            int64  `json:"expires_at" gorm:"not null;default:0;index"`
	RevokedAt            int64  `json:"revoked_at" gorm:"not null;default:0"`
	CreatedBy            int    `json:"created_by" gorm:"not null;default:0"`
	RevokeReason         string `json:"revoke_reason" gorm:"type:varchar(255)"`
}

func normalizeIdentityType(identityType string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(identityType)) {
	case "", IdentityTypeNone:
		return IdentityTypeNone, nil
	case IdentityTypeEnterprise:
		return IdentityTypeEnterprise, nil
	case IdentityTypeEducation, "student":
		return IdentityTypeEducation, nil
	default:
		return "", ErrInvalidIdentityType
	}
}

// NormalizeIdentityType validates and canonicalizes an identity value for
// controller/frontend boundaries.
func NormalizeIdentityType(identityType string) (string, error) {
	return normalizeIdentityType(identityType)
}

func GetUserIdentity(userID int) (*UserIdentity, error) {
	if userID <= 0 {
		return nil, fmt.Errorf("invalid user id")
	}
	if DB == nil {
		return nil, fmt.Errorf("database is not initialized")
	}
	var identity UserIdentity
	err := DB.Where("user_id = ?", userID).First(&identity).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return &UserIdentity{UserID: userID, IdentityType: IdentityTypeNone}, nil
	}
	if err != nil {
		return nil, err
	}
	if identity.IdentityType == "" {
		identity.IdentityType = IdentityTypeNone
	}
	return &identity, nil
}

func GetUserIdentityType(userID int) (string, error) {
	identity, err := GetUserIdentity(userID)
	if err != nil {
		return "", err
	}
	return identity.IdentityType, nil
}

// PopulateUserIdentityFields enriches user response objects without exposing
// the identity table to mass-assignment or requiring a schema join in every
// existing user query.
func PopulateUserIdentityFields(users []*User) error {
	if len(users) == 0 {
		return nil
	}
	if DB == nil {
		return fmt.Errorf("database is not initialized")
	}
	ids := make([]int, 0, len(users))
	for _, user := range users {
		if user != nil && user.Id > 0 {
			ids = append(ids, user.Id)
			user.IdentityType = IdentityTypeNone
			user.IdentityLabel = ""
		}
	}
	if len(ids) == 0 {
		return nil
	}
	var identities []UserIdentity
	if err := DB.Where("user_id IN ?", ids).Find(&identities).Error; err != nil {
		return err
	}
	byUser := make(map[int]string, len(identities))
	for _, identity := range identities {
		canonical, err := normalizeIdentityType(identity.IdentityType)
		if err == nil {
			byUser[identity.UserID] = canonical
		}
	}
	for _, user := range users {
		if user == nil {
			continue
		}
		identityType := byUser[user.Id]
		if identityType == "" {
			identityType = IdentityTypeNone
		}
		user.IdentityType = identityType
		user.IdentityLabel = IdentityLabel(identityType)
	}
	return nil
}

// SetUserIdentity updates the operator-granted identity atomically.  Changing
// or removing an identity revokes all active IP exceptions for that user so a
// stale enterprise/education grant cannot survive an operator decision.
func SetUserIdentity(userID, operatorID int, identityType string) (string, string, error) {
	canonical, err := normalizeIdentityType(identityType)
	if err != nil {
		return "", "", err
	}
	if userID <= 0 || operatorID <= 0 {
		return "", "", fmt.Errorf("invalid identity principal")
	}
	if DB == nil {
		return "", "", fmt.Errorf("database is not initialized")
	}
	previous := IdentityTypeNone
	err = DB.Transaction(func(tx *gorm.DB) error {
		var identity UserIdentity
		result := tx.Where("user_id = ?", userID).First(&identity)
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			identity = UserIdentity{UserID: userID, IdentityType: canonical}
			if canonical != IdentityTypeNone {
				identity.IdentityVerifiedAt = time.Now().Unix()
				identity.IdentityVerifiedBy = operatorID
			}
			if err := tx.Create(&identity).Error; err != nil {
				return err
			}
		} else if result.Error != nil {
			return result.Error
		} else {
			previous = identity.IdentityType
			if previous == "" {
				previous = IdentityTypeNone
			}
			updates := map[string]interface{}{"identity_type": canonical}
			if canonical == IdentityTypeNone {
				updates["identity_verified_at"] = 0
				updates["identity_verified_by"] = 0
			} else if previous != canonical {
				updates["identity_verified_at"] = time.Now().Unix()
				updates["identity_verified_by"] = operatorID
			}
			if err := tx.Model(&identity).Updates(updates).Error; err != nil {
				return err
			}
		}
		if previous != canonical {
			if err := tx.Model(&MainlandIPAllowlist{}).
				Where("user_id = ? AND status = ?", userID, MainlandIPAllowlistStatusActive).
				Updates(map[string]interface{}{
					"status":        MainlandIPAllowlistStatusRevoked,
					"revoked_at":    time.Now().Unix(),
					"revoke_reason": "identity_changed",
				}).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return "", "", err
	}
	_ = InvalidateUserCache(userID)
	return previous, canonical, nil
}

func normalizeAllowlistIP(ip net.IP) (value, family string, prefixLength int, ok bool) {
	if ip == nil {
		return "", "", 0, false
	}
	if v4 := ip.To4(); v4 != nil {
		return v4.String(), "ipv4", 32, true
	}
	if v6 := ip.To16(); v6 != nil {
		return v6.String(), "ipv6", 128, true
	}
	return "", "", 0, false
}

// AddMainlandIPWhitelist records exactly the supplied server-resolved address
// for an identity-bearing user.  Repeated requests are idempotent and refresh
// the last-seen timestamp rather than creating duplicate rows.
func AddMainlandIPWhitelist(userID, creatorID int, ip net.IP, source string) (*MainlandIPAllowlist, error) {
	if DB == nil {
		return nil, fmt.Errorf("database is not initialized")
	}
	if userID <= 0 || creatorID <= 0 {
		return nil, fmt.Errorf("invalid whitelist principal")
	}
	identity, err := GetUserIdentity(userID)
	if err != nil {
		return nil, err
	}
	canonicalIdentity, err := normalizeIdentityType(identity.IdentityType)
	if err != nil {
		return nil, err
	}
	if canonicalIdentity == IdentityTypeNone {
		return nil, ErrIdentityRequired
	}
	ipValue, family, prefixLength, ok := normalizeAllowlistIP(ip)
	if !ok {
		return nil, fmt.Errorf("invalid client IP")
	}
	source = strings.TrimSpace(source)
	if source != MainlandIPAllowlistSourceAdmin {
		source = MainlandIPAllowlistSourceSelf
	}
	now := time.Now().Unix()
	var row MainlandIPAllowlist
	query := DB.Where("user_id = ? AND ip = ? AND prefix_length = ? AND status = ?", userID, ipValue, prefixLength, MainlandIPAllowlistStatusActive)
	if err := query.First(&row).Error; err == nil {
		if err := DB.Model(&row).Updates(map[string]interface{}{"last_seen_at": now}).Error; err != nil {
			return nil, err
		}
		row.LastSeenAt = now
		return &row, nil
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	var activeCount int64
	if err := DB.Model(&MainlandIPAllowlist{}).
		Where("user_id = ? AND status = ? AND (expires_at = 0 OR expires_at > ?)", userID, MainlandIPAllowlistStatusActive, now).
		Count(&activeCount).Error; err != nil {
		return nil, err
	}
	if activeCount >= MainlandIPAllowlistMaxPerUser {
		return nil, ErrWhitelistLimit
	}
	row = MainlandIPAllowlist{
		UserID:               userID,
		IdentityTypeSnapshot: canonicalIdentity,
		IP:                   ipValue,
		AddressFamily:        family,
		PrefixLength:         prefixLength,
		Source:               source,
		Status:               MainlandIPAllowlistStatusActive,
		CreatedAt:            now,
		LastSeenAt:           now,
		CreatedBy:            creatorID,
	}
	if err := DB.Create(&row).Error; err != nil {
		// Another request may have inserted the same exact address between the
		// lookup and Create. Re-read the active row so the endpoint remains
		// idempotent under concurrent clicks.
		if lookupErr := DB.Where("user_id = ? AND ip = ? AND prefix_length = ? AND status = ?", userID, ipValue, prefixLength, MainlandIPAllowlistStatusActive).First(&row).Error; lookupErr == nil {
			return &row, nil
		}
		return nil, err
	}
	return &row, nil
}

// IsMainlandIPWhitelisted performs an exact active-address lookup.  It is safe
// during early startup/tests when the database is not initialized.
func IsMainlandIPWhitelisted(ip net.IP) bool {
	if DB == nil {
		return false
	}
	ipValue, _, prefixLength, ok := normalizeAllowlistIP(ip)
	if !ok {
		return false
	}
	var row MainlandIPAllowlist
	now := time.Now().Unix()
	err := DB.Model(&MainlandIPAllowlist{}).
		Where("ip = ? AND prefix_length = ? AND status = ? AND (expires_at = 0 OR expires_at > ?)", ipValue, prefixLength, MainlandIPAllowlistStatusActive, now).
		Order("id desc").First(&row).Error
	if err != nil {
		return false
	}
	// Re-check the current operator-granted identity as a defence in depth
	// measure. SetUserIdentity revokes rows transactionally, but this also
	// closes the window if an identity is removed by a migration or direct
	// administrative repair.
	identity, err := GetUserIdentity(row.UserID)
	if err != nil {
		return false
	}
	canonical, err := normalizeIdentityType(identity.IdentityType)
	return err == nil && canonical != IdentityTypeNone && canonical == row.IdentityTypeSnapshot
}

func ListMainlandIPAllowlists(userID int, includeRevoked bool) ([]MainlandIPAllowlist, error) {
	if DB == nil {
		return nil, fmt.Errorf("database is not initialized")
	}
	query := DB.Model(&MainlandIPAllowlist{}).Order("id desc")
	if userID > 0 {
		query = query.Where("user_id = ?", userID)
	}
	if !includeRevoked {
		query = query.Where("status = ?", MainlandIPAllowlistStatusActive)
	}
	var rows []MainlandIPAllowlist
	if err := query.Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func RevokeMainlandIPAllowlist(id, operatorID int, reason string) error {
	if DB == nil {
		return fmt.Errorf("database is not initialized")
	}
	if id <= 0 || operatorID <= 0 {
		return fmt.Errorf("invalid whitelist principal")
	}
	updates := map[string]interface{}{
		"status":        MainlandIPAllowlistStatusRevoked,
		"revoked_at":    time.Now().Unix(),
		"revoke_reason": strings.TrimSpace(reason),
	}
	result := DB.Model(&MainlandIPAllowlist{}).
		Where("id = ? AND status = ?", id, MainlandIPAllowlistStatusActive).
		Updates(updates)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// IdentityLabel is a stable user-facing label used by both themes.
func IdentityLabel(identityType string) string {
	switch identityType {
	case IdentityTypeEnterprise:
		return "ENTERPRISE"
	case IdentityTypeEducation:
		return "student"
	default:
		return ""
	}
}
