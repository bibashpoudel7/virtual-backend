package tours

import (
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
)

type CreateHotspotReq struct {
	Kind  string  `json:"kind" binding:"required"` // e.g., "icon", "text", "link", "video"
	Yaw   float64 `json:"yaw" binding:"required"`
	Pitch float64 `json:"pitch" binding:"required"`
	// Arbitrary JSON payload (iconUrl, text, link, etc.)
	Payload       map[string]any `json:"payload"`
	TourID        string         `json:"tour_id" binding:"required"`
	TargetSceneID string         `json:"target_scene_id" binding:"required"`
}

func (h *Handler) CreateHotspot(c *gin.Context) {
	sceneID := c.Param("sceneId")
	
	// Get user ID from context
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	var req CreateHotspotReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Verify tour ownership
	tour, err := h.service.GetTour(req.TourID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "tour not found"})
		return
	}
	if tour.UserID != userID.(string) {
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

	// Get the scene to verify ownership
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

	list, err := h.service.ListHotspots(sceneID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, list)
}

// UpdateSceneImagesReq receives image URLs from frontend after upload
type UpdateSceneImagesReq struct {
	MainImageURL  string          `json:"mainImageUrl" binding:"required"`
	TilesManifest json.RawMessage `json:"tilesManifest,omitempty"` // Store JSON manifest as-is
}

// UpdateSceneImages updates scene with URLs from frontend (no file upload)
func (h *Handler) UpdateSceneImages(c *gin.Context) {
	sceneID := c.Param("sceneId")

	// Get user ID from context
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	var req UpdateSceneImagesReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get the scene
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

	// Update scene with URLs from frontend
	scene.SrcOriginalURL = &req.MainImageURL

	// Store tiles manifest JSON in database
	if len(req.TilesManifest) > 0 {
		manifestStr := string(req.TilesManifest)
		scene.TilesManifest = &manifestStr
	}

	// Update scene in database
	if err := h.service.UpdateScene(scene); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update scene"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"scene":   scene,
	})
}
