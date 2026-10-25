// Package accounts handles sign-up, email verification, login and password reset.
package accounts

import (
	"errors"
	"log"
	"net/http"
	"net/mail"
	"os"
	"strings"
	"time"
	"unicode/utf8"

	"backend/internal/auth"
	"backend/internal/email"
	"backend/internal/models"
	"backend/internal/pkg/uuid"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

const (
	minPasswordLength = 8
	maxPasswordBytes  = 72 // bcrypt limit
)

// Compared against when the email is unknown, so login takes the same time either way
var dummyHash, _ = bcrypt.GenerateFromPassword([]byte("timing-equaliser-password"), bcrypt.DefaultCost)

type Mailer interface {
	SendOTP(to, code, purpose string) error
}

type Handler struct {
	db          *gorm.DB
	mailer      Mailer
	otps        *otpStore
	jwtSecret   string
	adminEmails map[string]bool
}

func NewHandler(db *gorm.DB, mailer Mailer, jwtSecret string) *Handler {
	admins := map[string]bool{}
	for _, e := range strings.Split(os.Getenv("ADMIN_EMAILS"), ",") {
		if e = normalizeEmail(e); e != "" {
			admins[e] = true
		}
	}
	return &Handler{
		db:          db,
		mailer:      mailer,
		otps:        &otpStore{db: db, secret: []byte(jwtSecret)},
		jwtSecret:   jwtSecret,
		adminEmails: admins,
	}
}

// RegisterRoutes: public auth endpoints plus /auth/me behind the auth middleware
func (h *Handler) RegisterRoutes(public *gin.RouterGroup, authMiddleware gin.HandlerFunc) {
	a := public.Group("/auth")
	a.POST("/register", h.Register)
	a.POST("/verify-email", h.VerifyEmail)
	a.POST("/resend-code", h.ResendCode)
	a.POST("/login", h.Login)
	a.POST("/forgot-password", h.ForgotPassword)
	a.POST("/reset-password", h.ResetPassword)
	a.GET("/me", authMiddleware, h.Me)
}

func normalizeEmail(raw string) string {
	return strings.ToLower(strings.TrimSpace(raw))
}

func validEmail(e string) bool {
	if len(e) > 254 {
		return false
	}
	addr, err := mail.ParseAddress(e)
	return err == nil && addr.Address == e && strings.Contains(e[strings.LastIndex(e, "@"):], ".")
}

func validatePassword(p string) error {
	if utf8.RuneCountInString(p) < minPasswordLength {
		return errors.New("password must be at least 8 characters")
	}
	if len(p) > maxPasswordBytes {
		return errors.New("password is too long (max 72 bytes)")
	}
	return nil
}

func (h *Handler) roleFor(email string) string {
	if h.adminEmails[email] {
		return models.RoleAdmin
	}
	return models.RoleUser
}

// sendCode issues and emails a code; returns an HTTP-ready error message on failure
func (h *Handler) sendCode(c *gin.Context, emailAddr, purpose string) bool {
	code, err := h.otps.issue(emailAddr, purpose)
	switch {
	case errors.Is(err, ErrCodeCooldown), errors.Is(err, ErrCodeHourlyLimit):
		c.JSON(http.StatusTooManyRequests, gin.H{"error": err.Error()})
		return false
	case err != nil:
		log.Printf("[accounts] issue code failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not create a verification code"})
		return false
	}
	if err := h.mailer.SendOTP(emailAddr, code, purpose); err != nil {
		log.Printf("[accounts] send code failed: %v", err)
		status := http.StatusBadGateway
		if errors.Is(err, email.ErrNotConfigured) {
			status = http.StatusServiceUnavailable
		}
		c.JSON(status, gin.H{"error": "could not send the email; please try again later"})
		return false
	}
	return true
}

func (h *Handler) respondWithToken(c *gin.Context, user *models.User) {
	now := time.Now()
	h.db.Model(user).Update("last_login_at", &now)
	token, err := auth.IssueToken(h.jwtSecret, user)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not sign in"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"token": token, "user": user})
}

type registerRequest struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (h *Handler) Register(c *gin.Context) {
	var req registerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	emailAddr := normalizeEmail(req.Email)
	if !validEmail(emailAddr) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "enter a valid email address"})
		return
	}
	if err := validatePassword(req.Password); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	name := strings.TrimSpace(req.Name)
	if utf8.RuneCountInString(name) > 100 {
		name = string([]rune(name)[:100])
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not create account"})
		return
	}

	var user models.User
	err = h.db.First(&user, "email = ?", emailAddr).Error
	switch {
	case err == nil && user.IsVerified():
		c.JSON(http.StatusConflict, gin.H{"error": "an account with this email already exists; sign in or reset your password"})
		return
	case err == nil:
		// Unverified sign-up retried: replace the details and send a fresh code
		user.Name = name
		user.PasswordHash = string(hash)
		if err := h.db.Save(&user).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "could not create account"})
			return
		}
	case errors.Is(err, gorm.ErrRecordNotFound):
		user = models.User{
			ID:           uuid.GenerateUUIDv7(),
			Email:        emailAddr,
			Name:         name,
			PasswordHash: string(hash),
			Role:         h.roleFor(emailAddr),
			Source:       "signup",
		}
		if err := h.db.Create(&user).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "could not create account"})
			return
		}
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not create account"})
		return
	}

	if !h.sendCode(c, emailAddr, PurposeVerifyEmail) {
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "verification_required", "email": emailAddr})
}

type codeRequest struct {
	Email string `json:"email"`
	Code  string `json:"code"`
}

func (h *Handler) VerifyEmail(c *gin.Context) {
	var req codeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	emailAddr := normalizeEmail(req.Email)

	var user models.User
	if err := h.db.First(&user, "email = ?", emailAddr).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": ErrCodeInvalid.Error()})
		return
	}
	if err := h.otps.verify(emailAddr, PurposeVerifyEmail, strings.TrimSpace(req.Code)); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if !user.IsVerified() {
		now := time.Now()
		user.EmailVerifiedAt = &now
		h.db.Model(&user).Update("email_verified_at", &now)
	}
	h.respondWithToken(c, &user)
}

type emailRequest struct {
	Email   string `json:"email"`
	Purpose string `json:"purpose"`
}

// ResendCode always answers the same way for unknown emails, so it can't be used to find accounts
func (h *Handler) ResendCode(c *gin.Context) {
	var req emailRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	emailAddr := normalizeEmail(req.Email)
	purpose := req.Purpose
	if purpose != PurposeResetPassword {
		purpose = PurposeVerifyEmail
	}

	var user models.User
	if err := h.db.First(&user, "email = ?", emailAddr).Error; err == nil {
		if purpose == PurposeVerifyEmail && user.IsVerified() {
			c.JSON(http.StatusOK, gin.H{"status": "sent"})
			return
		}
		if !h.sendCode(c, emailAddr, purpose) {
			return
		}
	}
	c.JSON(http.StatusOK, gin.H{"status": "sent"})
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (h *Handler) Login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	emailAddr := normalizeEmail(req.Email)
	wrong := func() { c.JSON(http.StatusUnauthorized, gin.H{"error": "incorrect email or password"}) }

	var user models.User
	if err := h.db.First(&user, "email = ?", emailAddr).Error; err != nil {
		bcrypt.CompareHashAndPassword(dummyHash, []byte(req.Password))
		wrong()
		return
	}
	if user.PasswordHash == "" {
		// Imported TheNimto account that hasn't set a password yet
		c.JSON(http.StatusForbidden, gin.H{"error": "set a password for your account using \"Forgot password\"", "code": "password_not_set"})
		return
	}
	if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)) != nil {
		wrong()
		return
	}
	if !user.IsVerified() {
		if !h.sendCode(c, emailAddr, PurposeVerifyEmail) {
			return
		}
		c.JSON(http.StatusForbidden, gin.H{"error": "verify your email first; we sent you a code", "code": "email_not_verified"})
		return
	}

	// Emails listed in ADMIN_EMAILS become admins on their next login
	if h.adminEmails[emailAddr] && user.Role != models.RoleAdmin {
		user.Role = models.RoleAdmin
		h.db.Model(&user).Update("role", models.RoleAdmin)
	}
	h.respondWithToken(c, &user)
}

func (h *Handler) ForgotPassword(c *gin.Context) {
	var req emailRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	emailAddr := normalizeEmail(req.Email)

	var user models.User
	if err := h.db.First(&user, "email = ?", emailAddr).Error; err == nil {
		if !h.sendCode(c, emailAddr, PurposeResetPassword) {
			return
		}
	}
	c.JSON(http.StatusOK, gin.H{"status": "sent"})
}

type resetRequest struct {
	Email    string `json:"email"`
	Code     string `json:"code"`
	Password string `json:"password"`
}

func (h *Handler) ResetPassword(c *gin.Context) {
	var req resetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	emailAddr := normalizeEmail(req.Email)
	if err := validatePassword(req.Password); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var user models.User
	if err := h.db.First(&user, "email = ?", emailAddr).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": ErrCodeInvalid.Error()})
		return
	}
	if err := h.otps.verify(emailAddr, PurposeResetPassword, strings.TrimSpace(req.Code)); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not reset password"})
		return
	}
	updates := map[string]interface{}{"password_hash": string(hash)}
	// The emailed code proves the address, so this also verifies it (imported accounts)
	if !user.IsVerified() {
		now := time.Now()
		user.EmailVerifiedAt = &now
		updates["email_verified_at"] = &now
	}
	if err := h.db.Model(&user).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not reset password"})
		return
	}
	h.respondWithToken(c, &user)
}

func (h *Handler) Me(c *gin.Context) {
	user, ok := auth.CurrentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Please sign in again"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"user": user})
}
