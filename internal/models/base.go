package models

import "time"

type BaseModel struct {
	CreatedAt time.Time `json:"created_at" gorm:"autoCreateTime;column:created_at"`
	UpdatedAt time.Time `json:"updated_at" gorm:"autoUpdateTime;column:updated_at"`
	UpdatedBy *string   `json:"updated_by,omitempty" gorm:"column:updated_by;size:50"`
	CreatedBy *string   `json:"created_by,omitempty" gorm:"column:created_by;size:50"`
}