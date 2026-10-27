// Package auth issues and verifies this service's own login tokens.
package auth

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"backend/internal/models"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"gorm.io/gorm"
)

const (
	RoleAdmin = models.RoleAdmin
	RoleUser  = models.RoleUser

	tokenIssuer   = "virtual-tours"
	tokenLifetime = 7 * 24 * time.Hour
)

// Claims carried by our login tokens. The role is NOT trusted from the token;
// it is re-read from the database on every request.
type Claims struct {
	UserID string `json:"uid"`
	jwt.RegisteredClaims
}

// IssueToken signs a login token for the user
func IssueToken(jwtSecret string, user *models.User) (string, error) {
	now := time.Now()
	claims := Claims{
		UserID: user.ID,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    tokenIssuer,
			Subject:   user.ID,
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(tokenLifetime)),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(jwtSecret))
}

// parseToken verifies signature, algorithm, issuer and expiry
func parseToken(jwtSecret, tokenString string) (*Claims, error) {
	claims := &Claims{}
	_, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		return []byte(jwtSecret), nil
	},
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithIssuer(tokenIssuer),
		jwt.WithExpirationRequired(),
	)
	if err != nil {
		return nil, err
	}
	if claims.UserID == "" {
		return nil, errors.New("token has no user")
	}
	return claims, nil
}

func bearerToken(c *gin.Context) string {
	header := c.GetHeader("Authorization")
	if len(header) > 7 && strings.EqualFold(header[:7], "Bearer ") {
		return strings.TrimSpace(header[7:])
	}
	return ""
}

// authenticate resolves the request's user, or returns nil
func authenticate(c *gin.Context, jwtSecret string, db *gorm.DB) *models.User {
	tokenString := bearerToken(c)
	if tokenString == "" {
		return nil
	}
	claims, err := parseToken(jwtSecret, tokenString)
	if err != nil {
		return nil
	}
	var user models.User
	if err := db.First(&user, "id = ?", claims.UserID).Error; err != nil || !user.IsVerified() {
		return nil
	}
	return &user
}

func setUser(c *gin.Context, user *models.User) {
	c.Set("user", user)
	c.Set("user_id", user.ID)
	c.Set("role", user.Role)
	c.Set("email", user.Email)
}

// AuthMiddleware requires a valid login token for a verified account
func AuthMiddleware(jwtSecret string, db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		user := authenticate(c, jwtSecret, db)
		if user == nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Please sign in again"})
			return
		}
		setUser(c, user)
		c.Next()
	}
}

// OptionalAuthMiddleware sets the user when a valid token is present, but never blocks
func OptionalAuthMiddleware(jwtSecret string, db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		if user := authenticate(c, jwtSecret, db); user != nil {
			setUser(c, user)
		}
		c.Next()
	}
}

// RequireAdmin must run after AuthMiddleware
func RequireAdmin() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !IsAdmin(c) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "Admin access required"})
			return
		}
		c.Next()
	}
}

// IsAdmin reports whether the authenticated user is an admin
func IsAdmin(c *gin.Context) bool {
	return c.GetString("role") == RoleAdmin
}

// CurrentUser returns the authenticated user set by the middleware
func CurrentUser(c *gin.Context) (*models.User, bool) {
	value, ok := c.Get("user")
	if !ok {
		return nil, false
	}
	user, ok := value.(*models.User)
	return user, ok
}
