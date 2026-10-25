package auth

import (
	"crypto/rand"
	"errors"
	"math/big"
	"strings"
	"time"

	"backend/internal/mailer"
	"backend/internal/models"
	"backend/internal/pkg/uuid"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

const (
	// CodeTTL is how long a verification code stays valid.
	CodeTTL = 10 * time.Minute
	// ResendCooldown throttles how often a new code can be requested.
	ResendCooldown = 60 * time.Second
	// MaxCodeAttempts caps wrong guesses before the code is invalidated.
	MaxCodeAttempts = 5
)

var (
	ErrNoPendingSignup = errors.New("no pending signup for that email; start again")
	ErrCodeExpired     = errors.New("that code has expired; request a new one")
	ErrCodeIncorrect   = errors.New("that code is not correct")
	ErrTooManyAttempts = errors.New("too many incorrect attempts; request a new code")
	ErrResendTooSoon   = errors.New("a code was just sent; please wait before requesting another")
)

// generateCode returns a cryptographically random 6-digit code.
func generateCode() (string, error) {
	var sb strings.Builder
	for i := 0; i < 6; i++ {
		n, err := rand.Int(rand.Reader, big.NewInt(10))
		if err != nil {
			return "", err
		}
		sb.WriteString(n.String())
	}
	return sb.String(), nil
}

// StartSignup records an unverified signup and emails a verification code.
// Re-running it for the same email replaces the pending record, which lets a
// user correct a typo without being locked out.
func (s *Service) StartSignup(email, password, name string, mail *mailer.Mailer) error {
	email = normalizeEmail(email)

	// Reject emails that already have a real account.
	var existing int64
	if err := s.db.Model(&models.User{}).Where("email = ?", email).Count(&existing).Error; err != nil {
		return err
	}
	if existing > 0 {
		return ErrEmailTaken
	}

	now := time.Now().UTC()

	// Throttle resends for an in-flight signup.
	var prior models.PendingSignup
	err := s.db.Where("email = ?", email).First(&prior).Error
	if err == nil && now.Sub(prior.LastSentAt) < ResendCooldown {
		return ErrResendTooSoon
	}
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}

	code, err := generateCode()
	if err != nil {
		return err
	}
	codeHash, err := bcrypt.GenerateFromPassword([]byte(code), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	passHash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	pending := models.PendingSignup{
		ID:           uuid.GenerateUUIDv7(),
		Email:        email,
		Name:         strings.TrimSpace(name),
		PasswordHash: string(passHash),
		CodeHash:     string(codeHash),
		ExpiresAt:    now.Add(CodeTTL),
		Attempts:     0,
		LastSentAt:   now,
	}

	// One pending record per email: replace any earlier attempt.
	if err := s.db.Where("email = ?", email).Delete(&models.PendingSignup{}).Error; err != nil {
		return err
	}
	if err := s.db.Create(&pending).Error; err != nil {
		return err
	}

	// If the email cannot be delivered, drop the pending row so the user is
	// not stuck waiting for a code that will never arrive.
	if err := mail.SendSignupCode(email, code, int(CodeTTL.Minutes())); err != nil {
		s.db.Where("email = ?", email).Delete(&models.PendingSignup{})
		return err
	}
	return nil
}

// VerifySignup checks the code and, on success, creates the real account and
// returns a token for it.
func (s *Service) VerifySignup(email, code string) (string, *models.User, error) {
	email = normalizeEmail(email)

	var pending models.PendingSignup
	err := s.db.Where("email = ?", email).First(&pending).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "", nil, ErrNoPendingSignup
	}
	if err != nil {
		return "", nil, err
	}

	now := time.Now().UTC()
	if now.After(pending.ExpiresAt) {
		s.db.Delete(&pending)
		return "", nil, ErrCodeExpired
	}
	if pending.Attempts >= MaxCodeAttempts {
		s.db.Delete(&pending)
		return "", nil, ErrTooManyAttempts
	}

	if bcrypt.CompareHashAndPassword([]byte(pending.CodeHash), []byte(strings.TrimSpace(code))) != nil {
		s.db.Model(&pending).UpdateColumn("attempts", pending.Attempts+1)
		if pending.Attempts+1 >= MaxCodeAttempts {
			s.db.Delete(&pending)
			return "", nil, ErrTooManyAttempts
		}
		return "", nil, ErrCodeIncorrect
	}

	// Code is good: promote the pending record to a real account. The
	// password hash carries over, so it is never re-hashed or re-sent.
	user := models.User{
		ID:           uuid.GenerateUUIDv7(),
		Email:        pending.Email,
		Name:         pending.Name,
		PasswordHash: pending.PasswordHash,
		Role:         "user",
		LastLoginAt:  &now,
	}

	if err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&user).Error; err != nil {
			return err
		}
		return tx.Delete(&pending).Error
	}); err != nil {
		return "", nil, err
	}

	token, err := s.IssueToken(&user)
	if err != nil {
		return "", nil, err
	}
	return token, &user, nil
}
