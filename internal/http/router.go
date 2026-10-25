// backend/http/router.go

package http

import (
	"backend/internal/auth"
	"backend/internal/config"
	"backend/internal/payment"
	"backend/internal/tours"
	"log"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func NewRouter(cfg config.Config, db *gorm.DB) *gin.Engine {
	r := gin.Default()

	// JWT secret comes from config (JWT_SECRET env var), so token issuing
	// and validation always use the same key.
	jwtSecret := cfg.JWTSecret

	// Add global middlewares
	r.Use(
		DefaultCORSConfig(),
		LoggingMiddleware(),
	)

	// Initialize local payment service
	paymentSvc := payment.NewLocalPaymentService(db)

	// Initialize services
	tourSvc, err := tours.NewService(db, cfg)
	if err != nil {
		log.Fatalf("Failed to initialize tour service: %v", err)
	}

	tourHandler := tours.NewHandler(tourSvc)

	// Health check endpoint (no auth)
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "healthy"})
	})

	// Auth validation endpoint
	r.POST("/api/auth/validate", auth.OptionalAuthMiddleware(jwtSecret), func(c *gin.Context) {
		if user, exists := auth.GetUserFromContext(c); exists {
			c.JSON(200, gin.H{
				"valid": true,
				"user":  user,
			})
		} else {
			c.JSON(401, gin.H{"valid": false})
		}
	})

	// Auth endpoints - public (no token required yet)
	authHandler := auth.NewHandler(auth.NewService(db, jwtSecret))
	authHandler.RegisterRoutes(r.Group("/api/auth"))

	// Payment endpoints - protected
	paymentAPI := r.Group("/api/payment")
	paymentAPI.Use(auth.AuthMiddleware(jwtSecret))
	{
		paymentAPI.GET("/check-limit", func(c *gin.Context) {
			userID := c.GetString("user_id")
			// Check if payment is required
			canCreate, remainingFree, _ := paymentSvc.CheckTourLimit(userID)
			c.JSON(200, gin.H{
				"paymentRequired": !canCreate,
				"remainingFree":   remainingFree,
			})
		})

		paymentAPI.POST("/create-payment", func(c *gin.Context) {
			var req struct {
				TourID string `json:"tourId"`
				Amount int64  `json:"amount"`
			}
			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(400, gin.H{"error": err.Error()})
				return
			}

			userID := c.GetString("user_id")

			payment, err := paymentSvc.CreateLocalPayment(userID, req.TourID, req.Amount)
			if err != nil {
				c.JSON(500, gin.H{"error": err.Error()})
				return
			}

			c.JSON(200, gin.H{
				"paymentId": payment.ID,
				"status":    payment.Status,
				"amount":    payment.Amount,
			})
		})

		paymentAPI.POST("/process/:paymentId", func(c *gin.Context) {
			paymentID := c.Param("paymentId")

			err := paymentSvc.ProcessLocalPayment(paymentID)
			if err != nil {
				c.JSON(500, gin.H{"error": err.Error()})
				return
			}

			c.JSON(200, gin.H{"success": true})
		})
	}

	// API routes - protected by auth
	api := r.Group("/api")
	api.Use(auth.AuthMiddleware(jwtSecret))

	// Register tour routes
	tourHandler.RegisterRoutes(api)

	return r
}
