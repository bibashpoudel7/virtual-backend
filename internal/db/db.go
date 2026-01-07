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

type Databases struct {
	Virtual *gorm.DB // For tours, scenes, hotspots
	Main    *gorm.DB // For users, companies, properties (read-only)
}

// MustConnect establishes connections to both databases
func MustConnect(cfg config.Config) *Databases {
	// Configure GORM with additional settings
	gormConfig := &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent), // Disable all SQL logging
		NowFunc: func() time.Time {
			return time.Now().UTC()
		},
		PrepareStmt:            true, // Enable prepared statements
		SkipDefaultTransaction: true, // Better performance for bulk operations
	}

	// Connect to virtual database (primary)
	virtualDB := connectWithRetry(cfg.PostgresDSN, gormConfig, "virtual")
	
	// Connect to main (nimto) database (for reading users, companies, properties)
	mainDB := connectWithRetry(cfg.MainPostgresDSN, gormConfig, "nimto")

	// Run migrations only on virtual database
	if err := migrate(virtualDB); err != nil {
		log.Fatalf("virtual database migration failed: %v", err)
	}

	log.Println("Both databases connected successfully")
	return &Databases{
		Virtual: virtualDB,
		Main:    mainDB,
	}
}

// connectWithRetry establishes a database connection with retry logic
func connectWithRetry(dsn string, gormConfig *gorm.Config, dbName string) *gorm.DB {
	var db *gorm.DB
	var err error
	maxRetries := 3

	for i := 0; i < maxRetries; i++ {
		db, err = gorm.Open(postgres.Open(dsn), gormConfig)
		if err == nil {
			break
		}

		if i == maxRetries-1 {
			log.Fatalf("failed to connect to %s database after %d attempts: %v", dbName, maxRetries, err)
		}

		time.Sleep(time.Second * time.Duration(1<<i)) // Exponential backoff
	}

	// Get underlying sql.DB for connection pool configuration
	sqlDB, err := db.DB()
	if err != nil {
		log.Fatalf("failed to get underlying sql.DB for %s: %v", dbName, err)
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
		log.Fatalf("%s database ping failed: %v", dbName, err)
	}

	log.Printf("%s database connected successfully", dbName)
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
		&models.PlayTour{},
		&models.PlayTourScene{},
	)
}

// CloseDBs closes both database connections
func CloseDBs(dbs *Databases) {
	if dbs.Virtual != nil {
		closeDB(dbs.Virtual, "virtual")
	}
	if dbs.Main != nil {
		closeDB(dbs.Main, "nimto")
	}
}

// closeDB closes a single database connection
func closeDB(db *gorm.DB, name string) {
	sqlDB, err := db.DB()
	if err != nil {
		log.Printf("failed to get underlying sql.DB for %s: %v", name, err)
		return
	}

	if err := sqlDB.Close(); err != nil {
		log.Printf("error closing %s database: %v", name, err)
	}
}
