package accounts

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"time"

	"backend/internal/models"
	"backend/internal/pkg/uuid"

	"gorm.io/gorm"
)

const (
	PurposeVerifyEmail   = "verify_email"
	PurposeResetPassword = "reset_password"

	otpLifetime    = 10 * time.Minute
	otpMaxAttempts = 5
	otpCooldown    = 60 * time.Second
	otpMaxPerHour  = 5
)

var (
	ErrCodeInvalid     = errors.New("the code is incorrect or has expired")
	ErrCodeTooMany     = errors.New("too many wrong attempts; request a new code")
	ErrCodeCooldown    = errors.New("please wait a minute before requesting another code")
	ErrCodeHourlyLimit = errors.New("too many codes requested; try again in an hour")
)

type otpStore struct {
	db     *gorm.DB
	secret []byte // HMAC key so stored hashes can't be brute-forced offline
}

func newCode() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(1000000))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%06d", n.Int64()), nil
}

func (s *otpStore) hash(email, purpose, code string) string {
	mac := hmac.New(sha256.New, s.secret)
	mac.Write([]byte(email + "|" + purpose + "|" + code))
	return hex.EncodeToString(mac.Sum(nil))
}

// issue creates a new code (invalidating older ones) and returns it for emailing
func (s *otpStore) issue(email, purpose string) (string, error) {
	now := time.Now()

	var last models.EmailOTP
	if err := s.db.Where("email = ? AND purpose = ?", email, purpose).Order("created_at desc").First(&last).Error; err == nil {
		if now.Sub(last.CreatedAt) < otpCooldown {
			return "", ErrCodeCooldown
		}
	}

	var recent int64
	s.db.Model(&models.EmailOTP{}).Where("email = ? AND created_at > ?", email, now.Add(-time.Hour)).Count(&recent)
	if recent >= otpMaxPerHour {
		return "", ErrCodeHourlyLimit
	}

	code, err := newCode()
	if err != nil {
		return "", err
	}

	err = s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&models.EmailOTP{}).
			Where("email = ? AND purpose = ? AND consumed_at IS NULL", email, purpose).
			Update("consumed_at", now).Error; err != nil {
			return err
		}
		return tx.Create(&models.EmailOTP{
			ID:        uuid.GenerateUUIDv7(),
			Email:     email,
			Purpose:   purpose,
			CodeHash:  s.hash(email, purpose, code),
			ExpiresAt: now.Add(otpLifetime),
		}).Error
	})
	if err != nil {
		return "", err
	}
	return code, nil
}

// verify checks a code and consumes it on success
func (s *otpStore) verify(email, purpose, code string) error {
	var otp models.EmailOTP
	err := s.db.Where("email = ? AND purpose = ? AND consumed_at IS NULL AND expires_at > ?", email, purpose, time.Now()).
		Order("created_at desc").First(&otp).Error
	if err != nil {
		return ErrCodeInvalid
	}
	if otp.Attempts >= otpMaxAttempts {
		return ErrCodeTooMany
	}

	if !hmac.Equal([]byte(otp.CodeHash), []byte(s.hash(email, purpose, code))) {
		s.db.Model(&models.EmailOTP{}).Where("id = ?", otp.ID).Update("attempts", gorm.Expr("attempts + 1"))
		if otp.Attempts+1 >= otpMaxAttempts {
			return ErrCodeTooMany
		}
		return ErrCodeInvalid
	}

	// Consume atomically so the same code can't be used twice in parallel
	result := s.db.Model(&models.EmailOTP{}).Where("id = ? AND consumed_at IS NULL", otp.ID).Update("consumed_at", time.Now())
	if result.Error != nil || result.RowsAffected == 0 {
		return ErrCodeInvalid
	}
	return nil
}
