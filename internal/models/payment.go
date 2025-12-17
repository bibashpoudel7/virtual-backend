package models

import "time"

// PaymentSession represents a temporary payment session
type PaymentSession struct {
	ID     string `json:"id"`
	URL    string `json:"url"`
	TourID string `json:"tour_id"`
	UserID string `json:"user_id"`
	Amount int64  `json:"amount"`
	Status string `json:"status"`
}

// Payment represents a payment transaction
type Payment struct {
	ID              string     `json:"id" gorm:"primaryKey;size:50"`
	UserID          string     `json:"user_id" gorm:"index"`
	TourID          *string    `json:"tour_id,omitempty" gorm:"index"`
	PropertyID      *int64     `json:"property_id,omitempty"`
	Amount          int64      `json:"amount"` // in cents
	Currency        string     `json:"currency"`
	Status          string     `json:"status"`       // pending, succeeded, failed, refunded
	PaymentType     string     `json:"payment_type"` // tour_creation, subscription, addon
	StripeSessionID string     `json:"stripe_session_id,omitempty"`
	StripePaymentID string     `json:"stripe_payment_id,omitempty"`
	Description     string     `json:"description,omitempty"`
	Metadata        string     `json:"metadata,omitempty" gorm:"type:jsonb"`
	RefundedAt      *time.Time `json:"refunded_at,omitempty"`
	FailureReason   string     `json:"failure_reason,omitempty"`

	BaseModel
}

// Subscription represents a user subscription
type Subscription struct {
	ID                   string     `json:"id" gorm:"primaryKey;size:50"`
	UserID               string     `json:"user_id" gorm:"index;unique"`
	PlanType             string     `json:"plan_type"` // basic, premium, enterprise
	Status               string     `json:"status"`    // active, cancelled, expired, paused
	StripeSubscriptionID string     `json:"stripe_subscription_id"`
	StartedAt            time.Time  `json:"started_at"`
	ExpiresAt            time.Time  `json:"expires_at"`
	CancelledAt          *time.Time `json:"cancelled_at,omitempty"`
	Features             string     `json:"features" gorm:"type:jsonb"` // JSON of feature flags

	BaseModel
}

// User represents a user from the main backend
type User struct {
	ID                string `json:"id" gorm:"primaryKey;size:50"`
	Email             string `json:"email" gorm:"unique"`
	Name              string `json:"name"`
	StripeCustomerID  string `json:"stripe_customer_id,omitempty"`
	PropertyID        *int64 `json:"property_id,omitempty"`
	Role              string `json:"role"` // user, agent, admin
	FreeTourUsed      int    `json:"free_tours_used" gorm:"default:0"`
	TotalToursCreated int    `json:"total_tours_created" gorm:"default:0"`

	BaseModel
}

// TourAccess represents access control for tours
type TourAccess struct {
	ID         string     `json:"id" gorm:"primaryKey;size:50"`
	TourID     string     `json:"tour_id" gorm:"index"`
	UserID     string     `json:"user_id" gorm:"index"`
	AccessType string     `json:"access_type"` // owner, editor, viewer
	GrantedBy  string     `json:"granted_by"`
	ExpiresAt  *time.Time `json:"expires_at,omitempty"`

	BaseModel
}
