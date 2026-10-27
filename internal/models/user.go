package models

import "time"

const (
	RoleAdmin = "admin"
	RoleUser  = "user"
)

// User is an account in the virtual tour system (replaces the TheNimto login).
// Users imported from TheNimto keep their old user ID so existing tours stay linked.
type User struct {
	ID              string     `json:"id" gorm:"primaryKey;size:50;column:id"`
	Email           string     `json:"email" gorm:"uniqueIndex;size:255;column:email"` // always lower-case
	Name            string     `json:"name" gorm:"size:100;column:name"`
	PasswordHash    string     `json:"-" gorm:"column:password_hash"` // empty until the user sets one (imported users)
	Role            string     `json:"role" gorm:"size:20;default:user;column:role"`
	EmailVerifiedAt *time.Time `json:"email_verified_at,omitempty" gorm:"column:email_verified_at"`
	LastLoginAt     *time.Time `json:"last_login_at,omitempty" gorm:"column:last_login_at"`
	Source          string     `json:"-" gorm:"size:20;default:signup;column:source"` // signup | nimto_import
	BaseModel
}

func (u *User) IsVerified() bool {
	return u.EmailVerifiedAt != nil
}

// EmailOTP is a one-time code emailed to verify an address or reset a password.
// Only a hash of the code is stored.
type EmailOTP struct {
	ID         string     `gorm:"primaryKey;size:50;column:id"`
	Email      string     `gorm:"index;size:255;column:email"`
	Purpose    string     `gorm:"size:30;column:purpose"` // verify_email | reset_password
	CodeHash   string     `gorm:"size:64;column:code_hash"`
	ExpiresAt  time.Time  `gorm:"column:expires_at"`
	Attempts   int        `gorm:"default:0;column:attempts"`
	ConsumedAt *time.Time `gorm:"column:consumed_at"`
	CreatedAt  time.Time  `gorm:"autoCreateTime;column:created_at"`
}

func (EmailOTP) TableName() string {
	return "email_otps"
}
