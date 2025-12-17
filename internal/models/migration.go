package models

import "time"

// Migration tracks which database migrations have been executed
type Migration struct {
	ID        string    `json:"id" gorm:"primaryKey;size:255;column:id"`
	Version   string    `json:"version" gorm:"uniqueIndex;size:255;column:version"`
	Name      string    `json:"name" gorm:"size:255;column:name"`
	ExecutedAt time.Time `json:"executed_at" gorm:"autoCreateTime;column:executed_at"`
}