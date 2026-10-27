package accounts

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"backend/internal/auth"
	"backend/internal/models"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Integration tests: run with TEST_DATABASE_URL pointing at a throwaway Postgres database.
const testSecret = "test-secret-0123456789-0123456789-abcdef"

type fakeMailer struct{ codes map[string]string }

func (m *fakeMailer) SendOTP(to, code, purpose string) error {
	m.codes[to+"|"+purpose] = code
	return nil
}

type testEnv struct {
	db     *gorm.DB
	mailer *fakeMailer
	router *gin.Engine
}

func setup(t *testing.T) *testEnv {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Migrator().DropTable(&models.User{}, &models.EmailOTP{}); err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&models.User{}, &models.EmailOTP{}); err != nil {
		t.Fatal(err)
	}

	t.Setenv("ADMIN_EMAILS", "boss@example.com")
	gin.SetMode(gin.TestMode)
	mailer := &fakeMailer{codes: map[string]string{}}
	h := NewHandler(db, mailer, testSecret)
	r := gin.New()
	requireAuth := auth.AuthMiddleware(testSecret, db)
	h.RegisterRoutes(r.Group("/api"), requireAuth)
	r.GET("/api/admin-only", requireAuth, auth.RequireAdmin(), func(c *gin.Context) { c.JSON(200, gin.H{"ok": true}) })
	return &testEnv{db: db, mailer: mailer, router: r}
}

func (e *testEnv) do(t *testing.T, method, path string, body any, headers ...string) (int, map[string]any) {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		json.NewEncoder(&buf).Encode(body)
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	for i := 0; i+1 < len(headers); i += 2 {
		req.Header.Set(headers[i], headers[i+1])
	}
	w := httptest.NewRecorder()
	e.router.ServeHTTP(w, req)
	out := map[string]any{}
	json.Unmarshal(w.Body.Bytes(), &out)
	return w.Code, out
}

// allowResend lets a test request a second code without waiting out the cooldown
func (e *testEnv) allowResend(email string) {
	e.db.Model(&models.EmailOTP{}).Where("email = ?", email).Update("created_at", time.Now().Add(-2*time.Minute))
}

func TestSignupVerifyLoginFlow(t *testing.T) {
	e := setup(t)
	email := "Alice@Example.com"

	if code, _ := e.do(t, "POST", "/api/auth/register", map[string]string{"email": email, "password": "short"}); code != 400 {
		t.Fatalf("short password: got %d", code)
	}
	if code, _ := e.do(t, "POST", "/api/auth/register", map[string]string{"email": "not-an-email", "password": "longenough1"}); code != 400 {
		t.Fatalf("bad email: got %d", code)
	}

	code, body := e.do(t, "POST", "/api/auth/register", map[string]string{"name": "Alice", "email": email, "password": "correct-horse"})
	if code != 200 || body["status"] != "verification_required" {
		t.Fatalf("register: %d %v", code, body)
	}
	sent := e.mailer.codes["alice@example.com|verify_email"]
	if len(sent) != 6 {
		t.Fatalf("expected 6-digit code, got %q", sent)
	}

	// Login before verifying is refused and resends a code
	e.allowResend("alice@example.com")
	if code, body := e.do(t, "POST", "/api/auth/login", map[string]string{"email": email, "password": "correct-horse"}); code != 403 || body["code"] != "email_not_verified" {
		t.Fatalf("unverified login: %d %v", code, body)
	}
	sent = e.mailer.codes["alice@example.com|verify_email"]

	if code, _ := e.do(t, "POST", "/api/auth/verify-email", map[string]string{"email": email, "code": "000000"}); code != 400 {
		t.Fatalf("wrong code: got %d", code)
	}
	code, body = e.do(t, "POST", "/api/auth/verify-email", map[string]string{"email": email, "code": sent})
	if code != 200 || body["token"] == "" {
		t.Fatalf("verify: %d %v", code, body)
	}
	if code, _ := e.do(t, "POST", "/api/auth/verify-email", map[string]string{"email": email, "code": sent}); code != 400 {
		t.Fatalf("code reused: got %d", code)
	}

	if code, _ := e.do(t, "POST", "/api/auth/login", map[string]string{"email": email, "password": "wrong-password"}); code != 401 {
		t.Fatalf("wrong password: got %d", code)
	}
	code, body = e.do(t, "POST", "/api/auth/login", map[string]string{"email": "  alice@example.COM ", "password": "correct-horse"})
	if code != 200 {
		t.Fatalf("login: %d %v", code, body)
	}
	token := body["token"].(string)

	code, body = e.do(t, "GET", "/api/auth/me", nil, "Authorization", "Bearer "+token)
	user := body["user"].(map[string]any)
	if code != 200 || user["email"] != "alice@example.com" || user["role"] != models.RoleUser {
		t.Fatalf("me: %d %v", code, body)
	}
	if _, leaked := user["password_hash"]; leaked {
		t.Fatal("password hash returned to client")
	}

	// A spoofed role header must not grant admin
	if code, _ := e.do(t, "GET", "/api/admin-only", nil, "Authorization", "Bearer "+token, "X-User-Role", "1"); code != 403 {
		t.Fatalf("role header spoofing: got %d", code)
	}

	// Signing up again with a verified email is refused
	if code, _ := e.do(t, "POST", "/api/auth/register", map[string]string{"email": email, "password": "another-pass"}); code != 409 {
		t.Fatalf("duplicate signup: got %d", code)
	}
}

func TestTooManyWrongCodes(t *testing.T) {
	e := setup(t)
	e.do(t, "POST", "/api/auth/register", map[string]string{"email": "bob@example.com", "password": "bob-password"})
	right := e.mailer.codes["bob@example.com|verify_email"]
	for i := 0; i < otpMaxAttempts; i++ {
		e.do(t, "POST", "/api/auth/verify-email", map[string]string{"email": "bob@example.com", "code": "999999"})
	}
	if code, _ := e.do(t, "POST", "/api/auth/verify-email", map[string]string{"email": "bob@example.com", "code": right}); code != 400 {
		t.Fatalf("correct code after lockout should fail, got %d", code)
	}
	if code, _ := e.do(t, "POST", "/api/auth/resend-code", map[string]string{"email": "bob@example.com"}); code != 429 {
		t.Fatalf("resend within cooldown should be 429, got %d", code)
	}
}

func TestImportedUserSetsPasswordViaReset(t *testing.T) {
	e := setup(t)
	// Imported from TheNimto: keeps old ID, no password, not verified
	e.db.Create(&models.User{ID: "legacy-nimto-id", Email: "carol@example.com", Role: models.RoleUser, Source: "nimto_import"})

	if code, body := e.do(t, "POST", "/api/auth/login", map[string]string{"email": "carol@example.com", "password": "anything123"}); code != 403 || body["code"] != "password_not_set" {
		t.Fatalf("imported login: %d %v", code, body)
	}

	if code, _ := e.do(t, "POST", "/api/auth/forgot-password", map[string]string{"email": "nobody@example.com"}); code != 200 {
		t.Fatalf("unknown email must look the same: %d", code)
	}
	e.do(t, "POST", "/api/auth/forgot-password", map[string]string{"email": "carol@example.com"})
	resetCode := e.mailer.codes["carol@example.com|reset_password"]

	code, body := e.do(t, "POST", "/api/auth/reset-password", map[string]string{"email": "carol@example.com", "code": resetCode, "password": "new-password-1"})
	if code != 200 {
		t.Fatalf("reset: %d %v", code, body)
	}
	user := body["user"].(map[string]any)
	if user["id"] != "legacy-nimto-id" || user["email_verified_at"] == nil {
		t.Fatalf("reset should keep id and verify email: %v", user)
	}
	if code, _ := e.do(t, "POST", "/api/auth/login", map[string]string{"email": "carol@example.com", "password": "new-password-1"}); code != 200 {
		t.Fatalf("login after reset: %d", code)
	}
}

func TestAdminEmailAndForeignTokens(t *testing.T) {
	e := setup(t)
	e.do(t, "POST", "/api/auth/register", map[string]string{"email": "boss@example.com", "password": "boss-password"})
	_, body := e.do(t, "POST", "/api/auth/verify-email", map[string]string{"email": "boss@example.com", "code": e.mailer.codes["boss@example.com|verify_email"]})
	token := body["token"].(string)
	if code, _ := e.do(t, "GET", "/api/admin-only", nil, "Authorization", "Bearer "+token); code != 200 {
		t.Fatalf("ADMIN_EMAILS user should be admin, got %d", code)
	}

	// A TheNimto-style token signed with the same secret (no issuer, user_id claim) is rejected
	var adminID string
	e.db.Model(&models.User{}).Select("id").Where("email = ?", "boss@example.com").Scan(&adminID)
	foreign, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id": adminID, "uid": adminID, "exp": time.Now().Add(time.Hour).Unix(),
	}).SignedString([]byte(testSecret))
	if code, _ := e.do(t, "GET", "/api/auth/me", nil, "Authorization", "Bearer "+foreign); code != 401 {
		t.Fatalf("token without our issuer must be rejected, got %d", code)
	}
	if code, _ := e.do(t, "GET", "/api/auth/me", nil, "Authorization", "Bearer "+strings.Repeat("x", 40)); code != 401 {
		t.Fatalf("garbage token: got %d", code)
	}
}
