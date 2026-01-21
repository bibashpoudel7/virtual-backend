package models

import (
	"time"
)

// Contact represents a contact form submission
type Contact struct {
	ID          string    `json:"id" gorm:"primaryKey;size:50;column:id"`
	FullName    string    `json:"full_name" gorm:"column:full_name;not null"`
	Email       string    `json:"email" gorm:"column:email;not null"`
	CountryCode *string   `json:"country_code,omitempty" gorm:"column:country_code"`
	PhoneNumber string    `json:"phone_number" gorm:"column:phone_number;not null"`
	Message     string    `json:"message" gorm:"column:message;not null"`
	Status      int       `json:"status" gorm:"column:status;default:1"` // 0=inactive, 1=active
	IP          *string   `json:"ip,omitempty" gorm:"column:ip"`
	CreatedAt   time.Time `json:"created_at" gorm:"column:created_at"`
	UpdatedAt   time.Time `json:"updated_at" gorm:"column:updated_at"`
}

// ContactRequest represents the request payload for contact form
type ContactRequest struct {
	FullName    string `json:"full_name" validate:"required"`
	Email       string `json:"email" validate:"required,email"`
	CountryCode string `json:"country_code"`
	PhoneNumber string `json:"phone_number" validate:"required"`
	Message     string `json:"message" validate:"required"`
}

// ContactResponse represents the response for contact form submission
type ContactResponse struct {
	ID      string `json:"id"`
	Message string `json:"message"`
}