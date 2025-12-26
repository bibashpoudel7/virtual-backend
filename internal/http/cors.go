package http

import (
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

// ConfigureCORS sets up CORS middleware for microservice
func ConfigureCORS(allowedOrigins []string) gin.HandlerFunc {
	config := cors.Config{
		AllowOrigins: allowedOrigins,
		AllowMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS", "PATCH"},
		AllowHeaders: []string{
			"Origin",
			"Content-Type",
			"Accept",
			"Authorization",
			"X-Requested-With",
			"X-Property-ID",
			"X-Source",
			"X-User-ID",
			"X-User-Role",
		},
		ExposeHeaders: []string{
			"Content-Length",
			"Content-Type",
			"X-Request-ID",
			"X-Payment-Required",
		},
		AllowCredentials: true,
		MaxAge:          12 * time.Hour,
	}

	return cors.New(config)
}

// DefaultCORSConfig returns default CORS configuration
func DefaultCORSConfig() gin.HandlerFunc {
	defaultOrigins := []string{
		"http://localhost:3000",  // Virtual frontend (Next.js)
		"http://localhost:3001",  // Main frontend (TheNimto web app)
		"http://localhost:3002",  // Microfrontend dev
		"http://localhost:8080",  // Microservice (NestJS backend - main TheNimto)
		"https://yourdomain.com", // Production domain
	}

	return ConfigureCORS(defaultOrigins)
}