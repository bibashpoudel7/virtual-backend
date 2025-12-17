// backend/http/middleware.go

package http

import (
	"log"
	"os"
	"strings"
	"time"

	"backend/utils/jwt"

	"github.com/gin-gonic/gin"
)

// LoggingMiddleware is a Gin middleware that logs the HTTP request details
func LoggingMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		latency := time.Since(start)
		log.Printf("%s %s %s %s", c.Request.Method, c.Request.URL.Path, c.Writer.Status(), latency)
	}
}

// CORSMiddleware handles CORS for the application
func CORSMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		allowed := os.Getenv("CORS_ALLOWED_ORIGINS")
		if allowed == "" {
			allowed = "*"
		}

		origins := strings.Split(allowed, ",")
		origin := c.GetHeader("Origin")

		for _, o := range origins {
			o = strings.TrimSpace(o)
			if o == "*" || o == origin {
				c.Writer.Header().Set("Access-Control-Allow-Origin", o)
				break
			}
		}

		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, PATCH, DELETE")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}

// AuthMiddleware handles JWT authentication
func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		println(authHeader)
		if authHeader == "" {
			c.AbortWithStatusJSON(401, gin.H{"error": "missing authorization header"})
			return
		}
		tokenStr := strings.TrimPrefix(authHeader, "Bearer ")

		token, err := jwt.VerifyToken(tokenStr)
		if err != nil {
			c.AbortWithStatusJSON(401, gin.H{"error": "invalid tokens"})
			return
		}

		claims := token.Claims.(jwt.MapClaims)
		c.Set("user", claims) // pass claims to handlers
		c.Next()
	}
}
