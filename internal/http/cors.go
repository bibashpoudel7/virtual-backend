package http

import (
	"strings"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

// ConfigureCORS sets up CORS middleware for microservice
func ConfigureCORS(allowedOrigins []string) gin.HandlerFunc {
	config := cors.Config{
		AllowOriginFunc: func(origin string) bool {
			// Allow all thenimto.com subdomains and apex
			if origin == "https://thenimto.com" || strings.HasSuffix(origin, ".thenimto.com") {
				return true
			}
			// Allow local development
			if strings.HasPrefix(origin, "http://localhost:") || strings.HasPrefix(origin, "http://127.0.0.1:") {
				return true
			}
			// Check configured origins
			for _, o := range allowedOrigins {
				if o == origin || o == "*" {
					return true
				}
			}
			return false
		},
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
	return ConfigureCORS(nil)
}
