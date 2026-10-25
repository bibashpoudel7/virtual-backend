package models

import "time"

// EmbedToken is an admin-issued key that allows a published tour to be embedded on other websites.
// The key is publishable (it ends up in the embedding site's HTML); access is controlled by
// revoking it and by AllowedDomains, which the frontend enforces with CSP frame-ancestors.
type EmbedToken struct {
	ID             string     `json:"id" gorm:"primaryKey;size:50;column:id"`
	TourID         string     `json:"tour_id" gorm:"index;size:50;column:tour_id"`
	Token          string     `json:"token" gorm:"uniqueIndex;size:80;column:token"`
	Name           string     `json:"name" gorm:"size:100;column:name"`
	AllowedDomains string     `json:"allowed_domains" gorm:"column:allowed_domains"` // comma-separated hosts; empty = any site
	RevokedAt      *time.Time `json:"revoked_at,omitempty" gorm:"column:revoked_at"`
	LastUsedAt     *time.Time `json:"last_used_at,omitempty" gorm:"column:last_used_at"`
	BaseModel
}

func (EmbedToken) TableName() string {
	return "tour_embed_tokens"
}
