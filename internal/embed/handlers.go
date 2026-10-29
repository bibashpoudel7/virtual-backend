// Package embed manages admin-issued keys that allow a tour to be embedded on other websites.
package embed

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"net/http"
	"regexp"
	"strings"
	"time"

	"backend/internal/auth"
	"backend/internal/models"
	"backend/internal/pkg/uuid"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const (
	maxKeysPerTour   = 20
	maxAllowedDomain = 20
	lastUsedInterval = time.Hour // throttle last_used_at writes
)

// Host with optional leading "*." wildcard and optional port, e.g. "example.com", "*.example.com", "localhost:5500"
var domainPattern = regexp.MustCompile(`^(\*\.)?[a-z0-9]([a-z0-9-]*[a-z0-9])?(\.[a-z0-9]([a-z0-9-]*[a-z0-9])?)*(:[0-9]{1,5})?$`)

var tokenPattern = regexp.MustCompile(`^vt_[A-Za-z0-9_-]{20,64}$`)

type Handler struct {
	db *gorm.DB
}

func NewHandler(db *gorm.DB) *Handler {
	return &Handler{db: db}
}

// RegisterRoutes adds the owner-only key management routes to the authenticated API group
func (h *Handler) RegisterRoutes(r *gin.RouterGroup) {
	r.GET("/tours/:id/embed-tokens", h.ListTokens)
	r.POST("/tours/:id/embed-tokens", h.CreateToken)
	r.DELETE("/tours/:id/embed-tokens/:tokenId", h.RevokeToken)
}

// RegisterPublicRoutes adds the key lookup used by the frontend's /embed/<key> route
func (h *Handler) RegisterPublicRoutes(r *gin.RouterGroup) {
	r.GET("/embed/:token", h.ResolveToken)
}

type createTokenRequest struct {
	Name           string   `json:"name"`
	AllowedDomains []string `json:"allowed_domains"`
}

// authorizeTour loads the tour and checks the caller owns it (or is superadmin)
func (h *Handler) authorizeTour(c *gin.Context) (*models.Tour, bool) {
	userID, ok := c.Get("user_id")
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return nil, false
	}
	role, _ := c.Get("role")

	var tour models.Tour
	if err := h.db.Select("id", "user_id", "is_published").First(&tour, "id = ?", c.Param("id")).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "tour not found"})
		return nil, false
	}

	roleStr, _ := role.(string)
	if roleStr != auth.RoleAdmin && tour.UserID != userID.(string) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return nil, false
	}
	return &tour, true
}

func (h *Handler) ListTokens(c *gin.Context) {
	tour, ok := h.authorizeTour(c)
	if !ok {
		return
	}

	var tokens []models.EmbedToken
	if err := h.db.Where("tour_id = ?", tour.ID).Order("created_at desc").Find(&tokens).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load embed keys"})
		return
	}
	c.JSON(http.StatusOK, tokens)
}

func (h *Handler) CreateToken(c *gin.Context) {
	tour, ok := h.authorizeTour(c)
	if !ok {
		return
	}

	var req createTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	domains, err := normalizeDomains(req.AllowedDomains)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = "Website embed"
	}
	if len(name) > 100 {
		name = name[:100]
	}

	var activeCount int64
	h.db.Model(&models.EmbedToken{}).Where("tour_id = ? AND revoked_at IS NULL", tour.ID).Count(&activeCount)
	if activeCount >= maxKeysPerTour {
		c.JSON(http.StatusBadRequest, gin.H{"error": "too many active embed keys for this tour; revoke one first"})
		return
	}

	token, err := generateToken()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate key"})
		return
	}

	userID := c.GetString("user_id")
	embedToken := models.EmbedToken{
		ID:             uuid.GenerateUUIDv7(),
		TourID:         tour.ID,
		Token:          token,
		Name:           name,
		AllowedDomains: strings.Join(domains, ","),
		BaseModel:      models.BaseModel{CreatedBy: &userID},
	}
	if err := h.db.Create(&embedToken).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create embed key"})
		return
	}
	c.JSON(http.StatusCreated, embedToken)
}

func (h *Handler) RevokeToken(c *gin.Context) {
	tour, ok := h.authorizeTour(c)
	if !ok {
		return
	}

	now := time.Now()
	result := h.db.Model(&models.EmbedToken{}).
		Where("id = ? AND tour_id = ? AND revoked_at IS NULL", c.Param("tokenId"), tour.ID).
		Update("revoked_at", &now)
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to revoke embed key"})
		return
	}
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "embed key not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "embed key revoked"})
}

// ResolveToken is public: it tells the frontend which tour a key opens and where it may be framed.
// Unknown, revoked or unpublished all return the same 404 so keys can't be probed.
func (h *Handler) ResolveToken(c *gin.Context) {
	token := c.Param("token")
	notFound := func() { c.JSON(http.StatusNotFound, gin.H{"error": "embed not available"}) }

	if !tokenPattern.MatchString(token) {
		notFound()
		return
	}

	var embedToken models.EmbedToken
	if err := h.db.Where("token = ? AND revoked_at IS NULL", token).First(&embedToken).Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "lookup failed"})
			return
		}
		notFound()
		return
	}

	var tour models.Tour
	if err := h.db.Select("id", "slug", "is_published").First(&tour, "id = ?", embedToken.TourID).Error; err != nil || !tour.IsPublished {
		notFound()
		return
	}

	if embedToken.LastUsedAt == nil || time.Since(*embedToken.LastUsedAt) > lastUsedInterval {
		now := time.Now()
		h.db.Model(&models.EmbedToken{}).Where("id = ?", embedToken.ID).Update("last_used_at", &now)
	}

	domains := []string{}
	if embedToken.AllowedDomains != "" {
		domains = strings.Split(embedToken.AllowedDomains, ",")
	}
	c.JSON(http.StatusOK, gin.H{
		"tour_id":         tour.ID,
		"slug":            tour.Slug,
		"allowed_domains": domains,
	})
}

func generateToken() (string, error) {
	buf := make([]byte, 24)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return "vt_" + base64.RawURLEncoding.EncodeToString(buf), nil
}

// normalizeDomains turns "https://www.Example.com/page" into "www.example.com" and validates each host
func normalizeDomains(input []string) ([]string, error) {
	seen := map[string]bool{}
	domains := []string{}
	for _, raw := range input {
		d := strings.ToLower(strings.TrimSpace(raw))
		d = strings.TrimPrefix(d, "https://")
		d = strings.TrimPrefix(d, "http://")
		if i := strings.IndexAny(d, "/?#"); i >= 0 {
			d = d[:i]
		}
		if d == "" || seen[d] {
			continue
		}
		if !domainPattern.MatchString(d) {
			return nil, errors.New("invalid domain: " + raw)
		}
		seen[d] = true
		domains = append(domains, d)
	}
	if len(domains) > maxAllowedDomain {
		return nil, errors.New("too many allowed domains")
	}
	return domains, nil
}
