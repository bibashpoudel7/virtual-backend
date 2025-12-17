package main

import (
	"flag"
	"log"
	"path/filepath"

	"backend/internal/config"
	"backend/internal/db"
	"backend/internal/migrate"
)

func main() {
	// Parse command line flags
	var (
		migrationsPath = flag.String("path", "migrations", "Path to migrations directory")
		checkOnly      = flag.Bool("check", false, "Only check for pending migrations without executing")
		rollback       = flag.Int("rollback", 0, "Rollback the last n migrations")
	)
	flag.Parse()

	// Load configuration and connect to database
	cfg := config.Load()
	database := db.MustConnect(cfg)
	if database == nil {
		log.Fatal("Failed to connect to database")
	}

	// Create migrator
	migrator := migrate.NewMigrator(database)

	// Convert relative path to absolute
	absPath, err := filepath.Abs(*migrationsPath)
	if err != nil {
		log.Fatalf("Failed to resolve migrations path: %v", err)
	}

	if *rollback > 0 {
		// Rollback migrations
		log.Printf("Rolling back %d migration(s)...", *rollback)
		if err := migrator.Rollback(absPath, *rollback); err != nil {
			log.Fatalf("Failed to rollback migrations: %v", err)
		}
	} else if *checkOnly {
		// Only check for pending migrations
		pending, err := migrator.GetPendingMigrations(absPath)
		if err != nil {
			log.Fatalf("Failed to check pending migrations: %v", err)
		}

		if len(pending) == 0 {
			log.Println("No pending migrations")
		} else {
			log.Printf("Found %d pending migration(s):", len(pending))
			for _, migration := range pending {
				log.Printf("  - %s", migration)
			}
		}
	} else {
		// Run migrations
		log.Println("Checking for pending migrations...")
		if err := migrator.Run(absPath); err != nil {
			log.Fatalf("Failed to run migrations: %v", err)
		}
	}
}