// backend/db/db.go
package db

import (
	"context"
	"log"
	"time"

	"backend/internal/config"
	"backend/internal/models"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// MustConnect establishes a database connection with retry logic and proper connection pooling
func MustConnect(cfg config.Config) *gorm.DB {
	// Configure GORM with additional settings
	gormConfig := &gorm.Config{
		Logger: logger.Default,
		NowFunc: func() time.Time {
			return time.Now().UTC()
		},
		PrepareStmt:            true, // Enable prepared statements
		SkipDefaultTransaction: true, // Better performance for bulk operations
	}

	// Initialize database connection with retry logic
	var db *gorm.DB
	var err error
	maxRetries := 3

	for i := 0; i < maxRetries; i++ {
		db, err = gorm.Open(postgres.Open(cfg.PostgresDSN), gormConfig)
		if err == nil {
			break
		}

		if i == maxRetries-1 {
			log.Fatalf("failed to connect to database after %d attempts: %v", maxRetries, err)
		}

		time.Sleep(time.Second * time.Duration(1<<i)) // Exponential backoff
	}

	// Get underlying sql.DB for connection pool configuration
	sqlDB, err := db.DB()
	if err != nil {
		log.Fatalf("failed to get underlying sql.DB: %v", err)
	}

	// Set connection pool settings
	sqlDB.SetMaxOpenConns(25)                 // Maximum number of open connections
	sqlDB.SetMaxIdleConns(5)                  // Maximum number of idle connections
	sqlDB.SetConnMaxLifetime(5 * time.Minute) // Maximum connection lifetime
	sqlDB.SetConnMaxIdleTime(time.Hour)       // Maximum idle time for connections

	// Test connection with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := sqlDB.PingContext(ctx); err != nil {
		sqlDB.Close()
		log.Fatalf("database ping failed: %v", err)
	}

	// Run migrations
	if err := migrate(db); err != nil {
		sqlDB.Close()
		log.Fatalf("database migration failed: %v", err)
	}

	log.Println("PostgreSQL connected successfully with connection pooling")
	return db
}

// migrate runs database migrations
func migrate(db *gorm.DB) error {
	return db.AutoMigrate(
		&models.Tour{},
		&models.TourScene{},
		&models.Scene{},
		&models.Hotspot{},
		&models.Overlay{},
	)
}

// CloseDB closes the database connection
func CloseDB(db *gorm.DB) {
	if db == nil {
		return
	}

	sqlDB, err := db.DB()
	if err != nil {
		log.Printf("failed to get underlying sql.DB: %v", err)
		return
	}

	if err := sqlDB.Close(); err != nil {
		log.Printf("error closing database: %v", err)
	}
}
