package migrate

import (
	"fmt"
	"io/ioutil"
	"log"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"backend/internal/models"
	"gorm.io/gorm"
)

// splitSQLStatements splits SQL content into individual statements
// This is a simple implementation that splits by semicolon
// but ignores semicolons within quotes
func splitSQLStatements(sql string) []string {
	var statements []string
	var current strings.Builder
	inSingleQuote := false
	inDoubleQuote := false
	
	for i := 0; i < len(sql); i++ {
		ch := sql[i]
		
		// Handle quotes
		if ch == '\'' && !inDoubleQuote && (i == 0 || sql[i-1] != '\\') {
			inSingleQuote = !inSingleQuote
		} else if ch == '"' && !inSingleQuote && (i == 0 || sql[i-1] != '\\') {
			inDoubleQuote = !inDoubleQuote
		}
		
		// Check for statement separator
		if ch == ';' && !inSingleQuote && !inDoubleQuote {
			stmt := strings.TrimSpace(current.String())
			if stmt != "" && !strings.HasPrefix(stmt, "--") {
				statements = append(statements, stmt)
			}
			current.Reset()
		} else {
			current.WriteByte(ch)
		}
	}
	
	// Add any remaining statement
	stmt := strings.TrimSpace(current.String())
	if stmt != "" && !strings.HasPrefix(stmt, "--") {
		statements = append(statements, stmt)
	}
	
	return statements
}

type Migrator struct {
	db *gorm.DB
}

func NewMigrator(db *gorm.DB) *Migrator {
	return &Migrator{db: db}
}

// Run executes all pending migrations
func (m *Migrator) Run(migrationsPath string) error {
	// Ensure migrations table exists
	if err := m.db.AutoMigrate(&models.Migration{}); err != nil {
		return fmt.Errorf("failed to create migrations table: %v", err)
	}

	// Get list of migration files
	files, err := ioutil.ReadDir(migrationsPath)
	if err != nil {
		return fmt.Errorf("failed to read migrations directory: %v", err)
	}

	// Filter and sort UP migration files only
	var migrationFiles []string
	for _, file := range files {
		if !file.IsDir() && strings.HasSuffix(file.Name(), ".up.sql") {
			migrationFiles = append(migrationFiles, file.Name())
		}
	}
	sort.Strings(migrationFiles)

	if len(migrationFiles) == 0 {
		log.Println("No migration files found")
		return nil
	}

	// Check which migrations have already been executed
	var executedMigrations []models.Migration
	if err := m.db.Find(&executedMigrations).Error; err != nil {
		return fmt.Errorf("failed to get executed migrations: %v", err)
	}

	// Create a map for faster lookup
	executed := make(map[string]bool)
	for _, migration := range executedMigrations {
		executed[migration.Version] = true
	}

	// Execute pending migrations
	pendingCount := 0
	for _, filename := range migrationFiles {
		version := strings.TrimSuffix(strings.TrimSuffix(filename, ".up.sql"), ".sql")
		
		if executed[version] {
			log.Printf("Migration %s already executed, skipping", version)
			continue
		}

		// Read migration file
		filePath := filepath.Join(migrationsPath, filename)
		content, err := ioutil.ReadFile(filePath)
		if err != nil {
			return fmt.Errorf("failed to read migration file %s: %v", filename, err)
		}

		// Execute migration in a transaction
		err = m.db.Transaction(func(tx *gorm.DB) error {
			// Split SQL content by semicolon to handle multiple statements
			statements := splitSQLStatements(string(content))
			
			for i, stmt := range statements {
				stmt = strings.TrimSpace(stmt)
				if stmt == "" {
					continue
				}
				
				// Execute each SQL statement
				if err := tx.Exec(stmt).Error; err != nil {
					return fmt.Errorf("failed to execute statement %d in migration %s: %v", i+1, version, err)
				}
			}

			// Record the migration
			migration := models.Migration{
				ID:         fmt.Sprintf("%s_%d", version, time.Now().UnixNano()),
				Version:    version,
				Name:       filename,
				ExecutedAt: time.Now(),
			}
			if err := tx.Create(&migration).Error; err != nil {
				return fmt.Errorf("failed to record migration %s: %v", version, err)
			}

			log.Printf("Successfully executed migration: %s", version)
			return nil
		})

		if err != nil {
			return err
		}
		pendingCount++
	}

	if pendingCount == 0 {
		log.Println("No pending migrations to execute")
	} else {
		log.Printf("Successfully executed %d migration(s)", pendingCount)
	}

	return nil
}

// Rollback rolls back the last n migrations
func (m *Migrator) Rollback(migrationsPath string, steps int) error {
	// Ensure migrations table exists
	if err := m.db.AutoMigrate(&models.Migration{}); err != nil {
		return fmt.Errorf("failed to create migrations table: %v", err)
	}

	// Get the last n executed migrations
	var executedMigrations []models.Migration
	if err := m.db.Order("executed_at DESC").Limit(steps).Find(&executedMigrations).Error; err != nil {
		return fmt.Errorf("failed to get executed migrations: %v", err)
	}

	if len(executedMigrations) == 0 {
		log.Println("No migrations to rollback")
		return nil
	}

	// Rollback in reverse order
	for _, migration := range executedMigrations {
		downFile := strings.Replace(migration.Name, ".up.sql", ".down.sql", 1)
		if downFile == migration.Name {
			// Handle old format migrations
			downFile = strings.Replace(migration.Name, ".sql", ".down.sql", 1)
		}
		
		filePath := filepath.Join(migrationsPath, downFile)
		
		// Check if down migration exists
		if _, err := ioutil.ReadFile(filePath); err != nil {
			log.Printf("No rollback file found for migration %s, skipping", migration.Version)
			continue
		}

		// Read down migration file
		content, err := ioutil.ReadFile(filePath)
		if err != nil {
			return fmt.Errorf("failed to read rollback file %s: %v", downFile, err)
		}

		// Execute rollback in a transaction
		err = m.db.Transaction(func(tx *gorm.DB) error {
			// Split SQL content by semicolon to handle multiple statements
			statements := splitSQLStatements(string(content))
			
			for i, stmt := range statements {
				stmt = strings.TrimSpace(stmt)
				if stmt == "" {
					continue
				}
				
				// Execute each SQL statement
				if err := tx.Exec(stmt).Error; err != nil {
					return fmt.Errorf("failed to execute rollback statement %d in migration %s: %v", i+1, migration.Version, err)
				}
			}

			// Remove the migration record
			if err := tx.Where("version = ?", migration.Version).Delete(&models.Migration{}).Error; err != nil {
				return fmt.Errorf("failed to remove migration record %s: %v", migration.Version, err)
			}

			log.Printf("Successfully rolled back migration: %s", migration.Version)
			return nil
		})

		if err != nil {
			return err
		}
	}

	log.Printf("Successfully rolled back %d migration(s)", len(executedMigrations))
	return nil
}

// GetPendingMigrations returns a list of migrations that haven't been executed yet
func (m *Migrator) GetPendingMigrations(migrationsPath string) ([]string, error) {
	// Ensure migrations table exists
	if err := m.db.AutoMigrate(&models.Migration{}); err != nil {
		return nil, fmt.Errorf("failed to create migrations table: %v", err)
	}

	// Get list of migration files
	files, err := ioutil.ReadDir(migrationsPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read migrations directory: %v", err)
	}

	// Filter UP migration files
	var migrationFiles []string
	for _, file := range files {
		if !file.IsDir() && strings.HasSuffix(file.Name(), ".up.sql") {
			migrationFiles = append(migrationFiles, strings.TrimSuffix(file.Name(), ".up.sql"))
		} else if !file.IsDir() && strings.HasSuffix(file.Name(), ".sql") && !strings.Contains(file.Name(), ".down.sql") && !strings.Contains(file.Name(), ".up.sql") {
			// Support old format without .up/.down
			migrationFiles = append(migrationFiles, strings.TrimSuffix(file.Name(), ".sql"))
		}
	}

	// Get executed migrations
	var executedMigrations []models.Migration
	if err := m.db.Find(&executedMigrations).Error; err != nil {
		return nil, fmt.Errorf("failed to get executed migrations: %v", err)
	}

	// Create a map for faster lookup
	executed := make(map[string]bool)
	for _, migration := range executedMigrations {
		executed[migration.Version] = true
	}

	// Find pending migrations
	var pending []string
	for _, version := range migrationFiles {
		if !executed[version] {
			pending = append(pending, version)
		}
	}

	sort.Strings(pending)
	return pending, nil
}