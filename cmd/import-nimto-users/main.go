// Command import-nimto-users copies the owners of existing tours from the TheNimto database into
// this service's users table, keeping their old user ID so their tours stay linked.
// Imported users have no password: they set one with "Forgot password" (an emailed code),
// which also verifies their email.
//
// Usage (one-time):
//
//	MAIN_POSTGRES_DSN=postgres://... go run ./cmd/import-nimto-users           # dry run
//	MAIN_POSTGRES_DSN=postgres://... go run ./cmd/import-nimto-users -apply    # write
package main

import (
	"flag"
	"log"
	"os"
	"strings"

	"backend/internal/models"

	"github.com/joho/godotenv"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type nimtoUser struct {
	ID        string
	Email     string
	FirstName string
	LastName  string
	Roles     string
}

func main() {
	apply := flag.Bool("apply", false, "write users (default is a dry run)")
	flag.Parse()
	_ = godotenv.Load()

	virtualDSN, mainDSN := os.Getenv("POSTGRES_DSN"), os.Getenv("MAIN_POSTGRES_DSN")
	if virtualDSN == "" || mainDSN == "" {
		log.Fatal("POSTGRES_DSN and MAIN_POSTGRES_DSN must both be set")
	}
	cfg := &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)}
	virtualDB, err := gorm.Open(postgres.Open(virtualDSN), cfg)
	if err != nil {
		log.Fatalf("virtual database: %v", err)
	}
	mainDB, err := gorm.Open(postgres.Open(mainDSN), cfg)
	if err != nil {
		log.Fatalf("TheNimto database: %v", err)
	}
	if err := virtualDB.AutoMigrate(&models.User{}, &models.EmailOTP{}); err != nil {
		log.Fatalf("migrate users: %v", err)
	}

	var ownerIDs []string
	if err := virtualDB.Table("tours").Distinct("user_id").Where("user_id <> ''").Pluck("user_id", &ownerIDs).Error; err != nil {
		log.Fatalf("read tour owners: %v", err)
	}
	log.Printf("%d distinct tour owners", len(ownerIDs))
	if len(ownerIDs) == 0 {
		return
	}

	var nimtoUsers []nimtoUser
	if err := mainDB.Table("users").
		Select("id::text AS id, email, first_name, last_name, roles::text AS roles").
		Where("id::text IN ?", ownerIDs).
		Scan(&nimtoUsers).Error; err != nil {
		log.Fatalf("read TheNimto users: %v", err)
	}

	admins := map[string]bool{}
	for _, e := range strings.Split(os.Getenv("ADMIN_EMAILS"), ",") {
		if e = strings.ToLower(strings.TrimSpace(e)); e != "" {
			admins[e] = true
		}
	}

	found := map[string]bool{}
	created, skipped := 0, 0
	for _, nu := range nimtoUsers {
		found[nu.ID] = true
		email := strings.ToLower(strings.TrimSpace(nu.Email))
		if email == "" {
			log.Printf("skip %s: no email", nu.ID)
			skipped++
			continue
		}

		var existing int64
		virtualDB.Model(&models.User{}).Where("id = ? OR email = ?", nu.ID, email).Count(&existing)
		if existing > 0 {
			log.Printf("skip %s (%s): already imported", nu.ID, email)
			skipped++
			continue
		}

		role := models.RoleUser
		if nu.Roles == "1" || admins[email] { // TheNimto SUPERADMIN = 1
			role = models.RoleAdmin
		}
		user := models.User{
			ID:     nu.ID,
			Email:  email,
			Name:   strings.TrimSpace(nu.FirstName + " " + nu.LastName),
			Role:   role,
			Source: "nimto_import",
		}
		log.Printf("%s %s <%s> as %s", map[bool]string{true: "import", false: "would import"}[*apply], user.ID, email, role)
		if *apply {
			if err := virtualDB.Create(&user).Error; err != nil {
				log.Printf("  failed: %v", err)
				continue
			}
		}
		created++
	}

	for _, id := range ownerIDs {
		if !found[id] {
			log.Printf("warning: tour owner %s not found in TheNimto users; their tours will have no owner account", id)
		}
	}
	log.Printf("done: %d %s, %d skipped (dry run: %v)", created, map[bool]string{true: "imported", false: "to import"}[*apply], skipped, !*apply)
}
