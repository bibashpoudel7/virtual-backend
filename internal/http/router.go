// backend/http/router.go

package http

import (
	"backend/internal/auth"
	"backend/internal/config"
	"backend/internal/contact"
	"backend/internal/db"
	"backend/internal/payment"
	"backend/internal/properties"
	"backend/internal/tours"
	"backend/internal/users"
	"log"
	"os"
	"strings"

	"github.com/dgrijalva/jwt-go"
	"github.com/gin-gonic/gin"
)

func NewRouter(cfg config.Config, dbs *db.Databases) *gin.Engine {
	r := gin.Default()

	// Get JWT secret from environment
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		jwtSecret = "8db1b8d5158b97eb7b54bf002765323c04a5c1be86d34b44548637f6e7a358e160a078390957e5481b069761e8b8c1dcd0489cb0fa2ab302071d>"
	}

	// Add global middlewares
	r.Use(
		DefaultCORSConfig(),
		LoggingMiddleware(),
	)

	// Health check endpoint
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "healthy"})
	})

	// Initialize local payment service
	paymentSvc := payment.NewLocalPaymentService(dbs.Virtual)

	// Initialize services
	userSvc := users.NewService(dbs.Main)
	tourSvc, err := tours.NewService(dbs.Virtual, dbs.Main, cfg)
	if err != nil {
		log.Fatalf("Failed to initialize tour service: %v", err)
	}

	propertySvc := properties.NewService(dbs.Virtual, dbs.Main)
	contactSvc := contact.NewService(dbs.Virtual)

	tourHandler := tours.NewHandler(tourSvc)
	propertyHandler := properties.NewHandler(propertySvc)
	contactHandler := contact.NewHandler(contactSvc)

	// Test endpoint to debug JWT tokens
	r.POST("/api/debug/token", func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(400, gin.H{"error": "No authorization header"})
			return
		}

		tokenString := strings.TrimPrefix(authHeader, "Bearer ")

		// Try to decode without verification first to see the structure
		token, _, err := new(jwt.Parser).ParseUnverified(tokenString, &auth.UserClaims{})
		if err != nil {
			c.JSON(400, gin.H{"error": "Failed to parse token", "details": err.Error()})
			return
		}

		if claims, ok := token.Claims.(*auth.UserClaims); ok {
			c.JSON(200, gin.H{
				"header":    token.Header,
				"claims":    claims,
				"raw_token": tokenString[:50] + "...", // First 50 chars for debugging
			})
		} else {
			c.JSON(400, gin.H{"error": "Invalid claims format"})
		}
	})

	// Auth validation endpoint
	r.POST("/api/auth/validate", auth.OptionalAuthMiddleware(jwtSecret, userSvc), func(c *gin.Context) {
		if user, exists := auth.GetUserFromContext(c); exists {
			c.JSON(200, gin.H{
				"valid": true,
				"user":  user,
			})
		} else {
			c.JSON(401, gin.H{"valid": false})
		}
	})

	// Payment endpoints - protected
	paymentAPI := r.Group("/api/payment")
	paymentAPI.Use(auth.AuthMiddleware(jwtSecret, userSvc))
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

	// Public API routes (no auth required) - for integration with TheNimto backend
	publicAPI := r.Group("/api")
	{
		// Contact form endpoint (public)
		publicAPI.POST("/contact", contactHandler.CreateContact)

		// Public property tour check endpoint for TheNimto backend integration
		publicAPI.GET("/properties/:propertyId/tour", propertyHandler.GetPropertyTour)
		publicAPI.GET("/properties/:propertyId/tour-details", propertyHandler.GetPropertyTourDetails)

		// Public tour viewing endpoints for end users
		publicAPI.GET("/tours/public", tourHandler.GetPublicTours)
		publicAPI.GET("/tours/:id/public", tourHandler.GetPublicTour)
		publicAPI.GET("/tours/:id/scenes/public", tourHandler.GetPublicTourScenes)

		// Public hotspots and overlays endpoints for tour viewing
		publicAPI.GET("/scenes/:sceneId/hotspots/public", tourHandler.GetPublicSceneHotspots)
		publicAPI.GET("/scenes/:sceneId/overlays/public", tourHandler.GetPublicSceneOverlays)
		publicAPI.GET("/tours/:id/play-tours/public", tourHandler.GetPublicPlayTours)
	}

	// API routes - protected by auth
	api := r.Group("/api")
	api.Use(auth.AuthMiddleware(jwtSecret, userSvc))

	// Register tour routes
	tourHandler.RegisterRoutes(api)

	// Register property routes
	propertyHandler.RegisterRoutes(api)

	// Admin contact routes (protected)
	contactAPI := api.Group("/contacts")
	{
		contactAPI.GET("", contactHandler.GetContacts)
		contactAPI.GET("/:id", contactHandler.GetContact)
		contactAPI.PATCH("/:id/status", contactHandler.UpdateContactStatus)
	}

	return r
}
