package model

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"time"

	"gorm.io/gorm"
)

var ErrSecurityProof = errors.New("Security verification expired or invalid; please verify again")

// SecurityProof stores only opaque-token digests. The conditional update below
// makes proofs single-use across processes, including deployments without Redis.
type SecurityProof struct {
	ID          string `gorm:"primaryKey;size:64"`
	UserID      int    `gorm:"index;not null"`
	SessionHash string `gorm:"size:64;not null"`
	Action      string `gorm:"size:128;not null"`
	ContextHash string `gorm:"size:64;not null"`
	Method      string `gorm:"size:32"`
	Verified    bool   `gorm:"not null;default:false"`
	Consumed    bool   `gorm:"not null;default:false"`
	ExpiresAt   int64  `gorm:"index;not null"`
}

// Server-side ceremony state prevents replaying signed session cookies.
type SecurityCeremony struct {
	ID        string `gorm:"primaryKey;size:64"`
	Payload   string `gorm:"type:text;not null"`
	ExpiresAt int64  `gorm:"index;not null"`
}

func SaveSecurityCeremony(payload string) (string, error) {
	token, err := SecurityToken()
	if err != nil {
		return "", err
	}
	if err := DB.Create(&SecurityCeremony{ID: SecurityDigest(token), Payload: payload, ExpiresAt: time.Now().Unix() + 300}).Error; err != nil {
		return "", err
	}
	return token, nil
}

func PopSecurityCeremony(token string) (string, error) {
	var state SecurityCeremony
	err := DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("id = ? AND expires_at > ?", SecurityDigest(token), time.Now().Unix()).First(&state).Error; err != nil {
			return ErrSecurityProof
		}
		result := tx.Where("id = ?", state.ID).Delete(&SecurityCeremony{})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ErrSecurityProof
		}
		return nil
	})
	return state.Payload, err
}

func SecurityToken() (string, error) {
	var token [32]byte
	if _, err := rand.Read(token[:]); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(token[:]), nil
}

func SecurityDigest(value string) string {
	digest := sha256.Sum256([]byte(value))
	return hex.EncodeToString(digest[:])
}

func NewSecurityProof(userID int, sessionHash, action, contextHash string) (string, error) {
	token, err := SecurityToken()
	if err != nil {
		return "", err
	}
	proof := SecurityProof{ID: SecurityDigest(token), UserID: userID, SessionHash: sessionHash,
		Action: action, ContextHash: contextHash, ExpiresAt: time.Now().Unix() + 300}
	if err := DB.Create(&proof).Error; err != nil {
		return "", err
	}
	return token, nil
}

func securityProofQuery(token string, userID int, sessionHash string) *gorm.DB {
	return DB.Model(&SecurityProof{}).Where("id = ? AND user_id = ? AND session_hash = ? AND consumed = ? AND expires_at > ?",
		SecurityDigest(token), userID, sessionHash, false, time.Now().Unix())
}

func GetSecurityProof(token string, userID int, sessionHash string) (*SecurityProof, error) {
	if len(token) != 43 {
		return nil, ErrSecurityProof
	}
	var proof SecurityProof
	if err := securityProofQuery(token, userID, sessionHash).First(&proof).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrSecurityProof
		}
		return nil, err
	}
	return &proof, nil
}

func VerifySecurityProof(token string, userID int, sessionHash, method string) error {
	result := securityProofQuery(token, userID, sessionHash).Where("verified = ?", false).
		Updates(map[string]any{"verified": true, "method": method})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return ErrSecurityProof
	}
	return nil
}

func ConsumeSecurityProof(token string, userID int, sessionHash, action, contextHash string) error {
	result := securityProofQuery(token, userID, sessionHash).
		Where("verified = ? AND action = ? AND context_hash = ?", true, action, contextHash).
		Update("consumed", true)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return ErrSecurityProof
	}
	return nil
}

func InvalidateSecurityProofs(userID int) error {
	return DB.Where("user_id = ?", userID).Delete(&SecurityProof{}).Error
}
