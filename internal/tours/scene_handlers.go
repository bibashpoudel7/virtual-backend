package tours

import (
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
)

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

	// Get user role from context
	role, roleExists := c.Get("role")
	if !roleExists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User role not found"})
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
