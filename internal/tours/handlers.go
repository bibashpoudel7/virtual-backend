// backend/tours/handlers.go
package tours

import (
	"fmt"
	"math"
	"net/http"
	"strconv"

	"backend/internal/models"
	"backend/internal/pkg/uuid"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	service Service
}

func NewHandler(service Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) RegisterRoutes(r *gin.RouterGroup) {
	toursGroup := r.Group("/tours")
	{
		toursGroup.POST("", h.CreateTour)
		toursGroup.GET("/featured", h.GetFeaturedTour)
		toursGroup.GET("/:id", h.GetTour)
		toursGroup.PUT("/:id", h.UpdateTour)
		toursGroup.DELETE("/:id", h.DeleteTour)
		toursGroup.GET("", h.ListAllTours)
		toursGroup.POST("/:id/scenes", h.CreateScene)
		toursGroup.GET("/:id/scenes", h.ListScenes)
		toursGroup.GET("/by-property/:propertyID", h.ListTours)

		// Superadmin endpoints
		toursGroup.PUT("/:id/publish", h.UpdateTourPublishStatus)
		toursGroup.PUT("/:id/feature", h.UpdateTourFeaturedStatus)

		// Tour-specific hotspot routes (alternative URL pattern)
		toursGroup.GET("/:id/scenes/:sceneId/hotspots", h.ListHotspotsByTourAndScene)
		toursGroup.POST("/:id/scenes/:sceneId/hotspots", h.CreateHotspotByTourAndScene)
		toursGroup.PUT("/:id/scenes/:sceneId/hotspots/:hotspotId", h.UpdateHotspotByTourAndScene)
		toursGroup.DELETE("/:id/scenes/:sceneId/hotspots/:hotspotId", h.DeleteHotspotByTourAndScene)

		// Play Tour endpoints
		toursGroup.POST("/:id/play-tours", h.CreatePlayTour)
		toursGroup.GET("/:id/play-tours", h.ListPlayTours)
	}

	playTours := r.Group("/play-tours")
	{
		playTours.GET("/:id", h.GetPlayTour)
		playTours.PUT("/:id", h.UpdatePlayTour)
		playTours.DELETE("/:id", h.DeletePlayTour)
	}

	// scene media + hotspots
	scenes := r.Group("/scenes")
	{
		scenes.GET("/:sceneId", h.GetScene)
		scenes.PUT("/:sceneId", h.UpdateScene)
		scenes.DELETE("/:sceneId", h.DeleteScene)

		// Image URLs update (from frontend after upload)
		scenes.POST("/:sceneId/update-images", h.UpdateSceneImages)

		// Hotspots
		scenes.POST("/:sceneId/hotspots", h.CreateHotspot)
		scenes.GET("/:sceneId/hotspots", h.ListHotspots)
		scenes.PUT("/:sceneId/hotspots/:hotspotId", h.UpdateHotspot)
		scenes.DELETE("/:sceneId/hotspots/:hotspotId", h.DeleteHotspot)

		// Overlays
		scenes.POST("/:sceneId/overlays", h.CreateOverlay)
		scenes.GET("/:sceneId/overlays", h.ListOverlays)
		scenes.PUT("/:sceneId/overlays/:overlayId", h.UpdateOverlay)
		scenes.DELETE("/:sceneId/overlays/:overlayId", h.DeleteOverlay)
	}
}

func (h *Handler) CreateTour(c *gin.Context) {
	// Use a map to handle JSON fields properly
	var requestData map[string]interface{}
	if err := c.ShouldBindJSON(&requestData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	println(uuid.GenerateUUIDv7())

	// Get user from context
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	role, exists := c.Get("role")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User role not found"})
		return
	}

	// Create tour with proper defaults
	tour := models.Tour{
		ID:                 uuid.GenerateUUIDv7(),
		Name:               getString(requestData, "name", ""),
		BackgroundAudioURL: getStringPtr(requestData, "background_audio_url"),
		DefaultFOV:         getFloat64(requestData, "default_fov", 75),
		DefaultYawSpeed:    getFloat64(requestData, "default_yaw_speed", 0.01),
		DefaultPitchSpeed:  getFloat64(requestData, "default_pitch_speed", 0.0),
		IsPublished:        getBool(requestData, "is_published", false),
		AutoplayEnabled:    getBool(requestData, "autoplay_enabled", false),
		Source:             "main_app", // Since this is from the virtual tour system
		UserID:             userID.(string),
	}

	// Handle property association based on role
	if propertyIDValue, exists := requestData["property_id"]; exists && propertyIDValue != nil {
		var propertyID string

		// Handle both string and number inputs
		switch v := propertyIDValue.(type) {
		case string:
			propertyID = v
		case float64:
			// Convert number to string (for backward compatibility)
			propertyID = fmt.Sprintf("%.0f", v)
		default:
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid property_id format"})
			return
		}

		// Convert role string to number
		roleStr := role.(string)
		var roleNum int

		switch roleStr {
		case "1", "SUPERADMIN":
			roleNum = 1
		case "3", "VENDOR":
			roleNum = 3
		default:
			if roleStr == "1" {
				roleNum = 1
			} else if roleStr == "3" {
				roleNum = 3
			} else {
				c.JSON(http.StatusForbidden, gin.H{"error": "Only vendors and superadmins can create property tours"})
				return
			}
		}

		// Validate property access based on role
		switch roleNum {
		case 1: // SUPERADMIN - can create tours for any approved venue property
		case 3: // VENDOR - can only create tours for their own approved properties
			// Validate that this property belongs to the vendor
			if err := h.service.ValidateVendorPropertyAccess(userID.(string), propertyID); err != nil {
				c.JSON(http.StatusForbidden, gin.H{"error": "You can only create tours for your own approved properties"})
				return
			}
		default:
			c.JSON(http.StatusForbidden, gin.H{"error": "Only vendors and superadmins can create property tours"})
			return
		}

		// Check if property already has a tour
		if existingTour, err := h.service.GetTourByPropertyID(propertyID); err == nil && existingTour != nil {
			c.JSON(http.StatusConflict, gin.H{"error": "This property already has a virtual tour"})
			return
		}

		tour.PropertyID = &propertyID
	} else {
		// No property_id provided - allow standalone tours for all roles
		roleStr := role.(string)

		// Set appropriate source based on role
		switch roleStr {
		case "1": // SUPERADMIN
			tour.Source = "standalone"
		case "2": // CUSTOMER
			tour.Source = "standalone"
		case "3": // VENDOR
			tour.Source = "standalone"
		default:
			tour.Source = "standalone"
		}
	}

	// Check if payment is required
	needsPayment, err := h.service.CheckIfPaymentRequired(tour.UserID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if needsPayment {
		// Create payment session
		session, err := h.service.CreatePaymentSession(&tour)
		if err != nil {
			c.JSON(http.StatusPaymentRequired, gin.H{
				"error":       "Payment required",
				"payment_url": session.URL,
				"session_id":  session.ID,
			})
			return
		}

		// Save tour as draft with payment pending
		tour.IsPublished = false
		tour.IsPaid = false
	} else {
		tour.IsPaid = true
	}

	if err := h.service.CreateTour(&tour); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, tour)
}

func (h *Handler) GetTour(c *gin.Context) {
	id := c.Param("id")
	println("bibash", id)

	// Get user ID from context
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	// Get user role from context
	role, roleExists := c.Get("role")
	if !roleExists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User role not found"})
		return
	}

	tour, err := h.service.GetTour(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "tour not found"})
		return
	}

	// Check if user is superadmin (role "1") or tour owner
	roleStr := role.(string)
	if roleStr != "1" && tour.UserID != userID.(string) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	c.JSON(http.StatusOK, tour)
}

func (h *Handler) GetFeaturedTour(c *gin.Context) {
	// Get user ID from context
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	// Get user role from context
	role, roleExists := c.Get("role")
	if !roleExists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User role not found"})
		return
	}

	// Get the featured tour
	featuredTour, err := h.service.GetFeaturedTour(userID.(string))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"message": "No featured tour found", "tour": nil})
		return
	}

	// Check if user is superadmin (role "1") or tour owner
	roleStr := role.(string)
	if roleStr != "1" && featuredTour.UserID != userID.(string) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	c.JSON(http.StatusOK, featuredTour)
}

func (h *Handler) ListTours(c *gin.Context) {
	propertyID := c.Param("propertyID")
	if propertyID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "property ID is required"})
		return
	}

	tours, err := h.service.ListTours(propertyID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, tours)
}

func (h *Handler) UpdateTour(c *gin.Context) {
	id := c.Param("id")

	// Get user ID from context
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	// Get existing tour first
	existingTour, err := h.service.GetTour(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "tour not found"})
		return
	}

	// Check if the tour belongs to the authenticated user
	if existingTour.UserID != userID.(string) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	// Use map for partial updates
	var updateData map[string]interface{}
	if err := c.ShouldBindJSON(&updateData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Update only provided fields
	if audioUrl, exists := updateData["background_audio_url"]; exists {
		if audioUrl == nil {
			existingTour.BackgroundAudioURL = nil
		} else if str, ok := audioUrl.(string); ok {
			existingTour.BackgroundAudioURL = &str
		}
	}

	if err := h.service.UpdateTour(existingTour); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, existingTour)
}

func (h *Handler) DeleteTour(c *gin.Context) {
	id := c.Param("id")

	// Get user ID from context
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	// Get user role from context
	role, roleExists := c.Get("role")
	if !roleExists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User role not found"})
		return
	}

	// Get existing tour first to check ownership
	existingTour, err := h.service.GetTour(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "tour not found"})
		return
	}

	// Check if user is superadmin (role "1") or tour owner
	roleStr := role.(string)
	if roleStr != "1" && existingTour.UserID != userID.(string) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	if err := h.service.DeleteTour(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusNoContent, nil)
}

func (h *Handler) ListAllTours(c *gin.Context) {
	// Get user ID from context
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	// Get user role from context
	role, roleExists := c.Get("role")
	if !roleExists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User role not found"})
		return
	}

	// Parse pagination parameters
	page := 1
	limit := 10 // Backend limit

	if pageStr := c.Query("page"); pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
	}

	if limitStr := c.Query("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 50 { // Max 50 per page
			limit = l
		}
	}

	// Check if user is superadmin (role "1")
	roleStr := role.(string)
	if roleStr == "1" {
		// Superadmin can see all tours with pagination
		toursWithProperty, total, err := h.service.ListAllTours(page, limit)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		// Calculate pagination metadata
		totalPages := int(math.Ceil(float64(total) / float64(limit)))

		response := gin.H{
			"data": toursWithProperty,
			"pagination": gin.H{
				"page":        page,
				"limit":       limit,
				"total":       total,
				"total_pages": totalPages,
			},
		}

		c.JSON(http.StatusOK, response)
		return
	}

	// For non-superadmin users, get user's tours with pagination
	userTours, total, err := h.service.ListUserTours(userID.(string), page, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Convert to TourWithProperty format for consistency
	var userToursWithProperty []models.TourWithProperty
	for _, tour := range userTours {
		userToursWithProperty = append(userToursWithProperty, models.TourWithProperty{Tour: tour})
	}

	// Calculate pagination metadata
	totalPages := int(math.Ceil(float64(total) / float64(limit)))

	response := gin.H{
		"data": userToursWithProperty,
		"pagination": gin.H{
			"page":        page,
			"limit":       limit,
			"total":       total,
			"total_pages": totalPages,
		},
	}

	c.JSON(http.StatusOK, response)
}

// UpdateTourPublishStatus updates the publication status of a tour (superadmin only)
func (h *Handler) UpdateTourPublishStatus(c *gin.Context) {
	tourID := c.Param("id")

	// Get user role from context
	role, roleExists := c.Get("role")
	if !roleExists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User role not found"})
		return
	}

	// Check if user is superadmin (role "1")
	roleStr := role.(string)
	if roleStr != "1" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Only superadmins can update tour publication status"})
		return
	}

	// Parse request body
	var request struct {
		IsPublished bool `json:"is_published"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get existing tour
	tour, err := h.service.GetTour(tourID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Tour not found"})
		return
	}

	// Update publication status
	tour.IsPublished = request.IsPublished

	if err := h.service.UpdateTour(tour); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":      "Tour publication status updated successfully",
		"tour_id":      tourID,
		"is_published": request.IsPublished,
	})
}

// UpdateTourFeaturedStatus updates the featured status of a tour (tour owner or superadmin)
func (h *Handler) UpdateTourFeaturedStatus(c *gin.Context) {
	tourID := c.Param("id")

	// Get user ID from context
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	// Get user role from context
	role, roleExists := c.Get("role")
	if !roleExists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User role not found"})
		return
	}

	// Parse request body
	var request struct {
		IsFeatured bool `json:"is_featured_on_homepage"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get existing tour
	tour, err := h.service.GetTour(tourID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Tour not found"})
		return
	}

	// Check if user is superadmin (role "1") or tour owner
	roleStr := role.(string)
	if roleStr != "1" && tour.UserID != userID.(string) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Only tour owners and superadmins can feature tours"})
		return
	}

	// If featuring this tour, unfeature all other tours first
	if request.IsFeatured {
		if err := h.service.UnfeatureAllTours(userID.(string)); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to unfeature other tours"})
			return
		}
	}

	// Update featured status
	tour.IsFeaturedOnHomepage = request.IsFeatured

	if err := h.service.UpdateTour(tour); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":                 "Tour featured status updated successfully",
		"tour_id":                 tourID,
		"is_featured_on_homepage": request.IsFeatured,
	})
}

func (h *Handler) CreateScene(c *gin.Context) {
	tourID := c.Param("id")

	// Get user ID from context
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	// Verify tour ownership
	tour, err := h.service.GetTour(tourID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "tour not found"})
		return
	}
	if tour.UserID != userID.(string) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	var scene models.Scene
	if err := c.ShouldBindJSON(&scene); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Validate required fields
	if scene.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "scene name is required"})
		return
	}
	if scene.Type == "" {
		scene.Type = "360" // Default to 360 if not specified
	}

	// Set default values if not provided
	if scene.FOV == 0 {
		scene.FOV = 75
	}
	if scene.Order == 0 {
		scene.Order = 1
	}
	if scene.Priority == 0 {
		scene.Priority = 1
	}

	scene.TourID = tourID
	if err := h.service.CreateScene(&scene); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, scene)
}

func (h *Handler) ListScenes(c *gin.Context) {
	tourID := c.Param("id")

	// Get user ID from context
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	// Get user role from context
	role, roleExists := c.Get("role")
	if !roleExists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User role not found"})
		return
	}

	// Verify tour ownership or superadmin access
	tour, err := h.service.GetTour(tourID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "tour not found"})
		return
	}

	// Check if user is superadmin (role "1") or tour owner
	roleStr := role.(string)
	if roleStr != "1" && tour.UserID != userID.(string) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))

	scenes, total, err := h.service.ListScenes(tourID, page, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":    "Scenes listed successfully",
		"datas":      scenes,
		"total":      total,
		"statusCode": http.StatusOK,
	})
}

func (h *Handler) GetScene(c *gin.Context) {
	sceneID := c.Param("sceneId")

	// Get user ID from context
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	// Get user role from context
	role, roleExists := c.Get("role")
	if !roleExists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User role not found"})
		return
	}

	scene, err := h.service.GetScene(sceneID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "scene not found"})
		return
	}

	// Verify tour ownership through scene's tour or superadmin access
	tour, err := h.service.GetTour(scene.TourID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "associated tour not found"})
		return
	}

	// Check if user is superadmin (role "1") or tour owner
	roleStr := role.(string)
	if roleStr != "1" && tour.UserID != userID.(string) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	c.JSON(http.StatusOK, scene)
}

func (h *Handler) UpdateScene(c *gin.Context) {
	sceneID := c.Param("sceneId")

	// Get user ID from context
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	// Get user role from context
	role, roleExists := c.Get("role")
	if !roleExists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User role not found"})
		return
	}

	// Get existing scene first
	existingScene, err := h.service.GetScene(sceneID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "scene not found"})
		return
	}

	// Verify tour ownership through scene's tour or superadmin access
	tour, err := h.service.GetTour(existingScene.TourID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "associated tour not found"})
		return
	}

	// Check if user is superadmin (role "1") or tour owner
	roleStr := role.(string)
	if roleStr != "1" && tour.UserID != userID.(string) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	var scene models.Scene
	if err := c.ShouldBindJSON(&scene); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	scene.ID = sceneID
	if err := h.service.UpdateScene(&scene); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, scene)
}

func (h *Handler) DeleteScene(c *gin.Context) {
	sceneID := c.Param("sceneId")

	// Get user ID from context
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	// Get user role from context
	role, roleExists := c.Get("role")
	if !roleExists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User role not found"})
		return
	}

	// Get existing scene first
	existingScene, err := h.service.GetScene(sceneID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "scene not found"})
		return
	}

	// Verify tour ownership through scene's tour or superadmin access
	tour, err := h.service.GetTour(existingScene.TourID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "associated tour not found"})
		return
	}

	// Check if user is superadmin (role "1") or tour owner
	roleStr := role.(string)
	if roleStr != "1" && tour.UserID != userID.(string) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	if err := h.service.DeleteScene(sceneID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusNoContent, nil)
}

func (h *Handler) UpdateHotspot(c *gin.Context) {
	hotspotID := c.Param("hotspotId")

	// Get user ID from context
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	// Get user role from context
	role, roleExists := c.Get("role")
	if !roleExists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User role not found"})
		return
	}

	var hotspot models.Hotspot
	if err := c.ShouldBindJSON(&hotspot); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Verify tour ownership through hotspot's tour or superadmin access
	tour, err := h.service.GetTour(hotspot.TourID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "associated tour not found"})
		return
	}

	// Check if user is superadmin (role "1") or tour owner
	roleStr := role.(string)
	if roleStr != "1" && tour.UserID != userID.(string) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	hotspot.ID = hotspotID
	if err := h.service.UpdateHotspot(&hotspot); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, hotspot)
}

func (h *Handler) DeleteHotspot(c *gin.Context) {
	hotspotID := c.Param("hotspotId")

	// Get user ID from context
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	// Get user role from context
	role, roleExists := c.Get("role")
	if !roleExists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User role not found"})
		return
	}

	// Get existing hotspot first
	existingHotspot, err := h.service.GetHotspot(hotspotID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "hotspot not found"})
		return
	}

	// Verify tour ownership through hotspot's tour or superadmin access
	tour, err := h.service.GetTour(existingHotspot.TourID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "associated tour not found"})
		return
	}

	// Check if user is superadmin (role "1") or tour owner
	roleStr := role.(string)
	if roleStr != "1" && tour.UserID != userID.(string) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	if err := h.service.DeleteHotspot(hotspotID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusNoContent, nil)
}

type CreateHotspotReq struct {
	Kind  string  `json:"kind" binding:"required"` // e.g., "icon", "text", "link", "video"
	Yaw   float64 `json:"yaw" binding:"required"`
	Pitch float64 `json:"pitch" binding:"required"`
	// Arbitrary JSON payload (iconUrl, text, link, etc.)
	Payload       map[string]any `json:"payload"`
	TourID        string         `json:"tour_id" binding:"required"`
	TargetSceneID string         `json:"target_scene_id"`
}

func (h *Handler) CreateHotspot(c *gin.Context) {
	sceneID := c.Param("sceneId")

	// Get user ID from context
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	// Get user role from context
	role, roleExists := c.Get("role")
	if !roleExists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User role not found"})
		return
	}

	var req CreateHotspotReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Custom validation: navigation hotspots require target_scene_id
	if req.Kind == "navigation" && req.TargetSceneID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "target_scene_id is required for navigation hotspots"})
		return
	}

	// Verify tour ownership or superadmin access
	tour, err := h.service.GetTour(req.TourID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "tour not found"})
		return
	}

	// Check if user is superadmin (role "1") or tour owner
	roleStr := role.(string)
	if roleStr != "1" && tour.UserID != userID.(string) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	hs, err := h.service.CreateHotspot(sceneID, req.TourID, req.TargetSceneID, req.Kind, req.Yaw, req.Pitch, req.Payload)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, hs)
}

func (h *Handler) ListHotspots(c *gin.Context) {
	sceneID := c.Param("sceneId")

	// Get user ID from context
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	// Get user role from context
	role, roleExists := c.Get("role")
	if !roleExists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User role not found"})
		return
	}

	// Debug logging
	println("ListHotspots - UserID:", userID.(string))
	println("ListHotspots - Role:", role.(string))

	// Get the scene to verify ownership
	scene, err := h.service.GetScene(sceneID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "scene not found"})
		return
	}

	println("ListHotspots - SceneID:", sceneID)
	println("ListHotspots - Scene.TourID:", scene.TourID)

	// Verify tour ownership through scene's tour or superadmin access
	tour, err := h.service.GetTour(scene.TourID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "associated tour not found"})
		return
	}

	println("ListHotspots - Tour.UserID:", tour.UserID)

	// Check if user is superadmin (role "1") or tour owner
	roleStr := role.(string)
	if roleStr != "1" && tour.UserID != userID.(string) {
		println("ListHotspots - Access denied: roleStr =", roleStr, ", tour.UserID =", tour.UserID, ", userID =", userID.(string))
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	println("ListHotspots - Access granted")

	list, err := h.service.ListHotspots(sceneID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, list)
}

// GetPublicTour returns tour data for public viewing (no authentication required)
func (h *Handler) GetPublicTour(c *gin.Context) {
	id := c.Param("id")

	tour, err := h.service.GetTour(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "tour not found"})
		return
	}

	// Only return published tours for public viewing
	if !tour.IsPublished {
		c.JSON(http.StatusNotFound, gin.H{"error": "tour not available for public viewing"})
		return
	}

	c.JSON(http.StatusOK, tour)
}

func (h *Handler) GetPublicTours(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))

	tours, total, err := h.service.ListAllPublicTours(page, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	totalPages := int(math.Ceil(float64(total) / float64(limit)))

	c.JSON(http.StatusOK, gin.H{
		"datas": tours,
		"pagination": gin.H{
			"page":        page,
			"limit":       limit,
			"total":       total,
			"total_pages": totalPages,
		},
		"statusCode": http.StatusOK,
	})
}

// GetPublicTourScenes returns scenes for a tour for public viewing (no authentication required)
func (h *Handler) GetPublicTourScenes(c *gin.Context) {
	tourID := c.Param("id")

	// Verify tour exists and is published
	tour, err := h.service.GetTour(tourID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "tour not found"})
		return
	}

	if !tour.IsPublished {
		c.JSON(http.StatusNotFound, gin.H{"error": "tour not available for public viewing"})
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))

	scenes, total, err := h.service.ListScenes(tourID, page, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":    "Scenes listed successfully",
		"datas":      scenes,
		"total":      total,
		"statusCode": http.StatusOK,
	})
}

// GetPublicSceneHotspots returns hotspots for a scene for public viewing (no authentication required)
func (h *Handler) GetPublicSceneHotspots(c *gin.Context) {
	sceneID := c.Param("sceneId")

	// Get scene to verify it exists and belongs to a published tour
	scene, err := h.service.GetScene(sceneID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "scene not found"})
		return
	}

	// Verify the tour is published
	tour, err := h.service.GetTour(scene.TourID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "associated tour not found"})
		return
	}

	if !tour.IsPublished {
		c.JSON(http.StatusNotFound, gin.H{"error": "tour not available for public viewing"})
		return
	}

	hotspots, err := h.service.ListHotspots(sceneID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, hotspots)
}

// GetPublicSceneOverlays returns overlays for a scene for public viewing (no authentication required)
func (h *Handler) GetPublicSceneOverlays(c *gin.Context) {
	sceneID := c.Param("sceneId")

	// Get scene to verify it exists and belongs to a published tour
	scene, err := h.service.GetScene(sceneID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "scene not found"})
		return
	}

	// Verify the tour is published
	tour, err := h.service.GetTour(scene.TourID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "associated tour not found"})
		return
	}

	if !tour.IsPublished {
		c.JSON(http.StatusNotFound, gin.H{"error": "tour not available for public viewing"})
		return
	}

	overlays, err := h.service.ListOverlays(sceneID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, overlays)
}

// Helper functions for safe type extraction from map
func getString(m map[string]interface{}, key string, defaultValue string) string {
	if val, exists := m[key]; exists {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return defaultValue
}

func getStringPtr(m map[string]interface{}, key string) *string {
	if val, exists := m[key]; exists {
		if str, ok := val.(string); ok && str != "" {
			return &str
		}
	}
	return nil
}

func getFloat64(m map[string]interface{}, key string, defaultValue float64) float64 {
	if val, exists := m[key]; exists {
		switch v := val.(type) {
		case float64:
			return v
		case float32:
			return float64(v)
		case int:
			return float64(v)
		case int64:
			return float64(v)
		}
	}
	return defaultValue
}

func getBool(m map[string]interface{}, key string, defaultValue bool) bool {
	if val, exists := m[key]; exists {
		if b, ok := val.(bool); ok {
			return b
		}
	}
	return defaultValue
}

// Tour-specific hotspot handlers (alternative URL pattern for frontend compatibility)

func (h *Handler) ListHotspotsByTourAndScene(c *gin.Context) {
	tourID := c.Param("id")
	sceneID := c.Param("sceneId")

	// Get user ID from context
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	// Get user role from context
	role, roleExists := c.Get("role")
	if !roleExists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User role not found"})
		return
	}

	// Verify tour ownership or superadmin access
	tour, err := h.service.GetTour(tourID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "tour not found"})
		return
	}

	// Check if user is superadmin (role "1") or tour owner
	roleStr := role.(string)
	if roleStr != "1" && tour.UserID != userID.(string) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	// Verify scene belongs to tour
	scene, err := h.service.GetScene(sceneID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "scene not found"})
		return
	}

	if scene.TourID != tourID {
		c.JSON(http.StatusBadRequest, gin.H{"error": "scene does not belong to specified tour"})
		return
	}

	list, err := h.service.ListHotspots(sceneID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, list)
}

func (h *Handler) CreateHotspotByTourAndScene(c *gin.Context) {
	tourID := c.Param("id")
	sceneID := c.Param("sceneId")

	// Get user ID from context
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	// Get user role from context
	role, roleExists := c.Get("role")
	if !roleExists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User role not found"})
		return
	}

	var req CreateHotspotReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Custom validation: navigation hotspots require target_scene_id
	if req.Kind == "navigation" && req.TargetSceneID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "target_scene_id is required for navigation hotspots"})
		return
	}

	// Verify tour ownership or superadmin access
	tour, err := h.service.GetTour(tourID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "tour not found"})
		return
	}

	// Check if user is superadmin (role "1") or tour owner
	roleStr := role.(string)
	if roleStr != "1" && tour.UserID != userID.(string) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	// Verify scene belongs to tour
	scene, err := h.service.GetScene(sceneID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "scene not found"})
		return
	}

	if scene.TourID != tourID {
		c.JSON(http.StatusBadRequest, gin.H{"error": "scene does not belong to specified tour"})
		return
	}

	// Override tour ID from URL parameter
	req.TourID = tourID

	hs, err := h.service.CreateHotspot(sceneID, req.TourID, req.TargetSceneID, req.Kind, req.Yaw, req.Pitch, req.Payload)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, hs)
}

func (h *Handler) UpdateHotspotByTourAndScene(c *gin.Context) {
	tourID := c.Param("id")
	sceneID := c.Param("sceneId")
	hotspotID := c.Param("hotspotId")

	// Get user ID from context
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	// Get user role from context
	role, roleExists := c.Get("role")
	if !roleExists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User role not found"})
		return
	}

	var hotspot models.Hotspot
	if err := c.ShouldBindJSON(&hotspot); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Verify tour ownership or superadmin access
	tour, err := h.service.GetTour(tourID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "tour not found"})
		return
	}

	// Check if user is superadmin (role "1") or tour owner
	roleStr := role.(string)
	if roleStr != "1" && tour.UserID != userID.(string) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	// Verify scene belongs to tour
	scene, err := h.service.GetScene(sceneID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "scene not found"})
		return
	}

	if scene.TourID != tourID {
		c.JSON(http.StatusBadRequest, gin.H{"error": "scene does not belong to specified tour"})
		return
	}

	hotspot.ID = hotspotID
	hotspot.TourID = tourID
	if err := h.service.UpdateHotspot(&hotspot); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, hotspot)
}

func (h *Handler) DeleteHotspotByTourAndScene(c *gin.Context) {
	tourID := c.Param("id")
	sceneID := c.Param("sceneId")
	hotspotID := c.Param("hotspotId")

	// Get user ID from context
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	// Get user role from context
	role, roleExists := c.Get("role")
	if !roleExists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User role not found"})
		return
	}

	// Verify tour ownership or superadmin access
	tour, err := h.service.GetTour(tourID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "tour not found"})
		return
	}

	// Check if user is superadmin (role "1") or tour owner
	roleStr := role.(string)
	if roleStr != "1" && tour.UserID != userID.(string) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	// Verify scene belongs to tour
	scene, err := h.service.GetScene(sceneID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "scene not found"})
		return
	}

	if scene.TourID != tourID {
		c.JSON(http.StatusBadRequest, gin.H{"error": "scene does not belong to specified tour"})
		return
	}

	// Get existing hotspot to verify it belongs to the scene
	existingHotspot, err := h.service.GetHotspot(hotspotID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "hotspot not found"})
		return
	}

	if existingHotspot.SceneID != sceneID {
		c.JSON(http.StatusBadRequest, gin.H{"error": "hotspot does not belong to specified scene"})
		return
	}

	if err := h.service.DeleteHotspot(hotspotID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusNoContent, nil)
}

// Overlay handlers
type CreateOverlayReq struct {
	Kind    string         `json:"kind" binding:"required"` // e.g., "text", "image", "video", "html", "badge", "tooltip"
	Yaw     float64        `json:"yaw"`
	Pitch   float64        `json:"pitch"`
	Payload map[string]any `json:"payload"`
	TourID  string         `json:"tour_id" binding:"required"`
}

func (h *Handler) CreateOverlay(c *gin.Context) {
	sceneID := c.Param("sceneId")

	// Get user ID from context
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	// Get user role from context
	role, roleExists := c.Get("role")
	if !roleExists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User role not found"})
		return
	}

	var req CreateOverlayReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Verify tour ownership or superadmin access
	tour, err := h.service.GetTour(req.TourID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "tour not found"})
		return
	}

	// Check if user is superadmin (role "1") or tour owner
	roleStr := role.(string)
	if roleStr != "1" && tour.UserID != userID.(string) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	overlay, err := h.service.CreateOverlay(sceneID, req.TourID, req.Kind, req.Yaw, req.Pitch, req.Payload)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, overlay)
}

func (h *Handler) ListOverlays(c *gin.Context) {
	sceneID := c.Param("sceneId")

	// Get user ID from context
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	// Get user role from context
	role, roleExists := c.Get("role")
	if !roleExists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User role not found"})
		return
	}

	// Get the scene to verify ownership
	scene, err := h.service.GetScene(sceneID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "scene not found"})
		return
	}

	// Verify tour ownership through scene's tour or superadmin access
	tour, err := h.service.GetTour(scene.TourID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "associated tour not found"})
		return
	}

	// Check if user is superadmin (role "1") or tour owner
	roleStr := role.(string)
	if roleStr != "1" && tour.UserID != userID.(string) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	overlays, err := h.service.ListOverlays(sceneID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, overlays)
}

func (h *Handler) UpdateOverlay(c *gin.Context) {
	overlayID := c.Param("overlayId")

	// Get user ID from context
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	// Get user role from context
	role, roleExists := c.Get("role")
	if !roleExists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User role not found"})
		return
	}

	var overlay models.Overlay
	if err := c.ShouldBindJSON(&overlay); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Verify tour ownership through overlay's tour or superadmin access
	tour, err := h.service.GetTour(overlay.TourID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "associated tour not found"})
		return
	}

	// Check if user is superadmin (role "1") or tour owner
	roleStr := role.(string)
	if roleStr != "1" && tour.UserID != userID.(string) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	overlay.ID = overlayID
	if err := h.service.UpdateOverlay(&overlay); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, overlay)
}

func (h *Handler) DeleteOverlay(c *gin.Context) {
	overlayID := c.Param("overlayId")

	// Get user ID from context
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	// Get user role from context
	role, roleExists := c.Get("role")
	if !roleExists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User role not found"})
		return
	}

	// Get existing overlay first
	existingOverlay, err := h.service.GetOverlay(overlayID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "overlay not found"})
		return
	}

	// Verify tour ownership through overlay's tour or superadmin access
	tour, err := h.service.GetTour(existingOverlay.TourID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "associated tour not found"})
		return
	}

	// Check if user is superadmin (role "1") or tour owner
	roleStr := role.(string)
	if roleStr != "1" && tour.UserID != userID.(string) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	if err := h.service.DeleteOverlay(overlayID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusNoContent, nil)
}

// PlayTour handlers

func (h *Handler) CreatePlayTour(c *gin.Context) {
	tourID := c.Param("id")

	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	var playTour models.PlayTour
	if err := c.ShouldBindJSON(&playTour); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	playTour.TourID = tourID
	playTour.UserID = userID.(string)

	if err := h.service.CreatePlayTour(&playTour); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, playTour)
}

func (h *Handler) GetPlayTour(c *gin.Context) {
	id := c.Param("id")
	playTour, err := h.service.GetPlayTour(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Play tour not found"})
		return
	}
	c.JSON(http.StatusOK, playTour)
}

func (h *Handler) UpdatePlayTour(c *gin.Context) {
	id := c.Param("id")

	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	var playTour models.PlayTour
	if err := c.ShouldBindJSON(&playTour); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	playTour.ID = id
	playTour.UserID = userID.(string)

	if err := h.service.UpdatePlayTour(&playTour); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, playTour)
}

func (h *Handler) DeletePlayTour(c *gin.Context) {
	id := c.Param("id")
	if err := h.service.DeletePlayTour(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusNoContent, nil)
}

func (h *Handler) ListPlayTours(c *gin.Context) {
	tourID := c.Param("id")
	playTours, err := h.service.ListPlayTours(tourID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, playTours)
}

// GetPublicPlayTours returns play tours for a tour for public viewing (no authentication required)
func (h *Handler) GetPublicPlayTours(c *gin.Context) {
	tourID := c.Param("id")

	// Verify tour exists and is published
	tour, err := h.service.GetTour(tourID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "tour not found"})
		return
	}

	if !tour.IsPublished {
		c.JSON(http.StatusNotFound, gin.H{"error": "tour not available for public viewing"})
		return
	}

	playTours, err := h.service.ListPlayTours(tourID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, playTours)
}
