// backend/http/router.go

package http

import (
	"backend/internal/accounts"
	"backend/internal/auth"
	"backend/internal/config"
	"backend/internal/contact"
	"backend/internal/db"
	"backend/internal/email"
	"backend/internal/embed"
	"backend/internal/payment"
	"backend/internal/floorplan"
	"backend/internal/tours"
	"log"
	"os"

	"github.com/gin-gonic/gin"
)

func NewRouter(cfg config.Config, dbs *db.Databases) *gin.Engine {
	r := gin.Default()

	// Secret for signing this service's own login tokens. No fallback: a guessable default would let
	// anyone forge tokens. Use a value NOT shared with any other system.
	jwtSecret := os.Getenv("JWT_SECRET")
	if len(jwtSecret) < 32 {
		log.Fatal("JWT_SECRET must be set to a random value of at least 32 characters")
	}

	// Liveness check for Docker/compose healthchecks
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	// Add global middlewares
	r.Use(
		DefaultCORSConfig(),
		LoggingMiddleware(),
	)

	// Initialize local payment service
	paymentSvc := payment.NewLocalPaymentService(dbs.Virtual)

	// Initialize services
	tourSvc, err := tours.NewService(dbs.Virtual, cfg)
	if err != nil {
		log.Fatalf("Failed to initialize tour service: %v", err)
	}

	contactSvc := contact.NewService(dbs.Virtual)

	tourHandler := tours.NewHandler(tourSvc)
	contactHandler := contact.NewHandler(contactSvc)
	embedHandler := embed.NewHandler(dbs.Virtual)
	accountHandler := accounts.NewHandler(dbs.Virtual, email.NewService(), jwtSecret)
	requireAuth := auth.AuthMiddleware(jwtSecret, dbs.Virtual)

	// Payment endpoints - protected
	paymentAPI := r.Group("/api/payment")
	paymentAPI.Use(requireAuth)
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

		// Sign-up, email verification (OTP), login and password reset
		accountHandler.RegisterRoutes(publicAPI, requireAuth)

		// Public tour viewing endpoints for end users
		publicAPI.GET("/tours/public", tourHandler.GetPublicTours)
		publicAPI.GET("/tours/:id/public", tourHandler.GetPublicTour)
		publicAPI.GET("/tours/:id/scenes/public", tourHandler.GetPublicTourScenes)

		// Public hotspots and overlays endpoints for tour viewing
		publicAPI.GET("/scenes/:sceneId/hotspots/public", tourHandler.GetPublicSceneHotspots)
		publicAPI.GET("/scenes/:sceneId/overlays/public", tourHandler.GetPublicSceneOverlays)
		publicAPI.GET("/tours/:id/play-tours/public", tourHandler.GetPublicPlayTours)

		// Embed key lookup used by the frontend /embed/<key> route
		embedHandler.RegisterPublicRoutes(publicAPI)
	}

	// API routes - protected by auth
	api := r.Group("/api")
	api.Use(requireAuth)

	// Register tour routes
	tourHandler.RegisterRoutes(api)

	// Floor plan reading and phone building scans for the dollhouse builder (tour owner or admin)
	floorplanHandler := floorplan.NewHandler(tourSvc, cfg.AnthropicAPIKey)
	floorplanHandler.RegisterRoutes(api)
	floorplanHandler.RegisterScanRoutes(api)

	// Embed key management (tour owner or superadmin)
	embedHandler.RegisterRoutes(api)

	// Contact form submissions: admins only
	contactAPI := api.Group("/contacts", auth.RequireAdmin())
	{
		contactAPI.GET("", contactHandler.GetContacts)
		contactAPI.GET("/:id", contactHandler.GetContact)
		contactAPI.PATCH("/:id/status", contactHandler.UpdateContactStatus)
	}

	return r
}
