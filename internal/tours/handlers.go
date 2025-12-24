// backend/tours/handlers.go
package tours

import (
	"fmt"
	"net/http"

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
		toursGroup.GET("/:id", h.GetTour)
		toursGroup.PUT("/:id", h.UpdateTour)
		toursGroup.DELETE("/:id", h.DeleteTour)
		toursGroup.GET("", h.ListAllTours)
		toursGroup.POST("/:id/scenes", h.CreateScene)
		toursGroup.GET("/:id/scenes", h.ListScenes)
		toursGroup.GET("/by-property/:propertyID", h.ListTours)
	}

	// NEW: scene media + hotspots
	scenes := r.Group("/scenes")
	{
		scenes.GET("/:sceneId", h.GetScene)
		scenes.PUT("/:sceneId", h.UpdateScene)
		scenes.DELETE("/:sceneId", h.DeleteScene)
		
		// Image URLs update (from frontend after upload)
		scenes.POST("/:sceneId/update-images", h.UpdateSceneImages)

		// Hotspots
		scenes.POST("/:sceneId/hotspots", h.CreateHotspot) // step 4
		scenes.GET("/:sceneId/hotspots", h.ListHotspots)
		scenes.PUT("/:sceneId/hotspots/:hotspotId", h.UpdateHotspot)
		scenes.DELETE("/:sceneId/hotspots/:hotspotId", h.DeleteHotspot)
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

	tour, err := h.service.GetTour(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "tour not found"})
		return
	}

	// Check if the tour belongs to the authenticated user
	if tour.UserID != userID.(string) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	c.JSON(http.StatusOK, tour)
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
	
	// Get existing tour first to check ownership
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

	// Get tours with property information
	toursWithProperty, err := h.service.ListAllTours()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	
	// Filter tours to only show user's tours
	var userTours []models.TourWithProperty
	for _, tour := range toursWithProperty {
		if tour.UserID == userID.(string) {
			userTours = append(userTours, tour)
		}
	}

	c.JSON(http.StatusOK, userTours)
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

	scenes, err := h.service.ListScenes(tourID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, scenes)
}

func (h *Handler) GetScene(c *gin.Context) {
	sceneID := c.Param("sceneId")
	
	// Get user ID from context
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	scene, err := h.service.GetScene(sceneID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "scene not found"})
		return
	}

	// Verify tour ownership through scene's tour
	tour, err := h.service.GetTour(scene.TourID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "associated tour not found"})
		return
	}
	if tour.UserID != userID.(string) {
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

	// Get existing scene first
	existingScene, err := h.service.GetScene(sceneID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "scene not found"})
		return
	}

	// Verify tour ownership through scene's tour
	tour, err := h.service.GetTour(existingScene.TourID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "associated tour not found"})
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

	// Get existing scene first
	existingScene, err := h.service.GetScene(sceneID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "scene not found"})
		return
	}

	// Verify tour ownership through scene's tour
	tour, err := h.service.GetTour(existingScene.TourID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "associated tour not found"})
		return
	}
	if tour.UserID != userID.(string) {
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

	var hotspot models.Hotspot
	if err := c.ShouldBindJSON(&hotspot); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Verify tour ownership through hotspot's tour
	tour, err := h.service.GetTour(hotspot.TourID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "associated tour not found"})
		return
	}
	if tour.UserID != userID.(string) {
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

	// Get existing hotspot first
	existingHotspot, err := h.service.GetHotspot(hotspotID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "hotspot not found"})
		return
	}

	// Verify tour ownership through hotspot's tour
	tour, err := h.service.GetTour(existingHotspot.TourID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "associated tour not found"})
		return
	}
	if tour.UserID != userID.(string) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	if err := h.service.DeleteHotspot(hotspotID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusNoContent, nil)
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
