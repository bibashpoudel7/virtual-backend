package models

import "time"

// PendingSignup is an unverified signup attempt. The account is only written
// to the users table once the emailed code is confirmed, so unverified
// records never become real accounts.
//
// Both the password and the verification code are stored as bcrypt hashes.
type PendingSignup struct {
	ID           string    `json:"id" gorm:"primaryKey;size:50;column:id"`
	Email        string    `json:"email" gorm:"column:email;size:255;uniqueIndex;not null"`
	Name         string    `json:"name" gorm:"column:name;size:255"`
	PasswordHash string    `json:"-" gorm:"column:password_hash;size:255;not null"`
	CodeHash     string    `json:"-" gorm:"column:code_hash;size:255;not null"`
	ExpiresAt    time.Time `json:"expires_at" gorm:"column:expires_at;not null"`
	Attempts     int       `json:"attempts" gorm:"column:attempts;default:0"`
	LastSentAt   time.Time `json:"last_sent_at" gorm:"column:last_sent_at"`

	BaseModel
}

func (PendingSignup) TableName() string { return "pending_signups" }
