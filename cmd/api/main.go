// backend/cmd/api/main.go
package main

import (
	"backend/internal/config"
	vdb "backend/internal/db"
	httpapi "backend/internal/http"
	"log"

	"github.com/joho/godotenv"
)

func main() {
	// Load .env file
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using environment variables")
	}

	// 1️⃣ Load configuration
	cfg := config.Load()

	// 2️⃣ Connect to PostgreSQL
	gdb := vdb.MustConnect(cfg)

	// 6️⃣ Create Gin router with all dependencies
	router := httpapi.NewRouter(cfg, gdb)

	// 7️⃣ Start server
	log.Printf("virtual-tour-service listening on :%s", cfg.Port)
	if err := router.Run(":" + cfg.Port); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
