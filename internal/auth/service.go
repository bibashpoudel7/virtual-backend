package auth

import (
	"errors"
	"strings"
	"time"

	"backend/internal/models"

	"backend/internal/pkg/uuid"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// ErrInvalidCredentials is returned for both unknown emails and wrong
// passwords, so the response never reveals which accounts exist.
var ErrInvalidCredentials = errors.New("invalid email or password")

// ErrEmailTaken is returned when registering an email that already exists.
var ErrEmailTaken = errors.New("an account with that email already exists")

// TokenTTL is how long an issued access token stays valid.
const TokenTTL = 24 * time.Hour

type Service struct {
	db        *gorm.DB
	jwtSecret []byte
}

func NewService(db *gorm.DB, jwtSecret string) *Service {
	return &Service{db: db, jwtSecret: []byte(jwtSecret)}
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

// Login verifies credentials and returns a signed token plus the user.
func (s *Service) Login(email, password string) (string, *models.User, error) {
	var user models.User
	err := s.db.Where("email = ?", normalizeEmail(email)).First(&user).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		// Hash a dummy value so a missing account costs the same time as a
		// wrong password, avoiding a timing oracle on which emails exist.
		bcrypt.CompareHashAndPassword(
			[]byte("$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy"),
			[]byte(password),
		)
		return "", nil, ErrInvalidCredentials
	}
	if err != nil {
		return "", nil, err
	}

	if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)) != nil {
		return "", nil, ErrInvalidCredentials
	}

	token, err := s.IssueToken(&user)
	if err != nil {
		return "", nil, err
	}

	now := time.Now().UTC()
	s.db.Model(&user).UpdateColumn("last_login_at", now)
	user.LastLoginAt = &now

	return token, &user, nil
}

// Register creates an account and returns a token for it.
func (s *Service) Register(email, password, name string) (string, *models.User, error) {
	email = normalizeEmail(email)

	var count int64
	if err := s.db.Model(&models.User{}).Where("email = ?", email).Count(&count).Error; err != nil {
		return "", nil, err
	}
	if count > 0 {
		return "", nil, ErrEmailTaken
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", nil, err
	}

	user := models.User{
		ID:           uuid.GenerateUUIDv7(),
		Email:        email,
		PasswordHash: string(hash),
		Name:         strings.TrimSpace(name),
		Role:         "user",
	}
	if err := s.db.Create(&user).Error; err != nil {
		return "", nil, err
	}

	token, err := s.IssueToken(&user)
	if err != nil {
		return "", nil, err
	}
	return token, &user, nil
}

// IssueToken signs a JWT whose claims match what AuthMiddleware reads.
func (s *Service) IssueToken(user *models.User) (string, error) {
	now := time.Now().UTC()
	claims := jwt.MapClaims{
		"user_id": user.ID,
		"sub":     user.ID,
		"email":   user.Email,
		"role":    user.Role,
		"iat":     now.Unix(),
		"nbf":     now.Unix(),
		"exp":     now.Add(TokenTTL).Unix(),
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(s.jwtSecret)
}
