// Command create-user creates a verified account directly (no email code), optionally taking over
// tours whose owner has no account (e.g. tours left from the TheNimto login).
//
// The password is read from the CREATE_USER_PASSWORD environment variable so it doesn't end up
// in shell history as a flag.
//
//	CREATE_USER_PASSWORD='…' go run ./cmd/create-user -email you@example.com -name "You" -role admin -claim-orphaned-tours
package main

import (
	"flag"
	"log"
	"os"
	"strings"
	"time"

	"backend/internal/models"
	"backend/internal/pkg/uuid"

	"github.com/joho/godotenv"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func main() {
	email := flag.String("email", "", "account email (required)")
	name := flag.String("name", "", "display name")
	role := flag.String("role", models.RoleUser, "admin or user")
	claim := flag.Bool("claim-orphaned-tours", false, "give this account every tour whose owner has no account")
	flag.Parse()
	_ = godotenv.Load()

	addr := strings.ToLower(strings.TrimSpace(*email))
	password := os.Getenv("CREATE_USER_PASSWORD")
	switch {
	case addr == "" || !strings.Contains(addr, "@"):
		log.Fatal("-email is required")
	case len(password) < 8 || len(password) > 72:
		log.Fatal("CREATE_USER_PASSWORD must be 8-72 characters")
	case *role != models.RoleAdmin && *role != models.RoleUser:
		log.Fatal("-role must be admin or user")
	}

	db, err := gorm.Open(postgres.Open(os.Getenv("POSTGRES_DSN")), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		log.Fatalf("database: %v", err)
	}
	if err := db.AutoMigrate(&models.User{}, &models.EmailOTP{}); err != nil {
		log.Fatalf("migrate users: %v", err)
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		log.Fatal(err)
	}

	err = db.Transaction(func(tx *gorm.DB) error {
		now := time.Now()
		var user models.User
		if err := tx.Where("email = ?", addr).First(&user).Error; err == nil {
			log.Printf("account %s exists (%s): updating password, role and verifying", addr, user.ID)
			if err := tx.Model(&user).Updates(map[string]interface{}{
				"password_hash": string(hash), "role": *role, "email_verified_at": &now,
			}).Error; err != nil {
				return err
			}
		} else {
			user = models.User{
				ID: uuid.GenerateUUIDv7(), Email: addr, Name: strings.TrimSpace(*name),
				PasswordHash: string(hash), Role: *role, EmailVerifiedAt: &now, Source: "admin_cli",
			}
			if err := tx.Create(&user).Error; err != nil {
				return err
			}
			log.Printf("created %s account %s (%s)", *role, addr, user.ID)
		}

		if !*claim {
			return nil
		}
		orphaned := tx.Table("tours").Select("id").Where("user_id NOT IN (?)", tx.Table("users").Select("id"))
		var names []string
		tx.Table("tours").Where("id IN (?)", orphaned).Pluck("name", &names)
		result := tx.Exec("UPDATE play_tours SET user_id = ? WHERE tour_id IN (?)", user.ID, orphaned)
		if result.Error != nil {
			return result.Error
		}
		playTours := result.RowsAffected
		result = tx.Exec("UPDATE tours SET user_id = ? WHERE id IN (?)", user.ID, orphaned)
		if result.Error != nil {
			return result.Error
		}
		log.Printf("moved %d tour(s) to %s: %s (and %d play tour(s))", result.RowsAffected, addr, strings.Join(names, ", "), playTours)
		return nil
	})
	if err != nil {
		log.Fatalf("failed, nothing changed: %v", err)
	}
}
