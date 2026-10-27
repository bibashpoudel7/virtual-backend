package floorplan

import (
	"encoding/json"
	"math"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// BuildingScan is the room layout measured with the Android capture app (ARCore).
// Coordinates are meters on the floor in the AR session's world frame: x right, z toward
// the user at the session start (same axes as three.js), shared by every room of the scan.
type BuildingScan struct {
	Version    int        `json:"version"`
	CapturedAt time.Time  `json:"captured_at"`
	Device     string     `json:"device"`
	Rooms      []ScanRoom `json:"rooms"`
}

type ScanRoom struct {
	Name string `json:"name"`
	// The tour scene photographed in this room, or ""
	SceneID string `json:"scene_id"`
	// Floor corners [x, z], in walking order
	Corners [][2]float64 `json:"corners"`
	// 0 when not measured
	CeilingHeight float64 `json:"ceiling_height"`
	// Where the 360° photo was taken [x, z], or null
	PhotoSpot *[2]float64 `json:"photo_spot"`
}

const (
	maxScanRooms   = 100
	maxRoomCorners = 64
	maxCoordinate  = 500.0 // meters from the scan origin
)

func (h *Handler) RegisterScanRoutes(r *gin.RouterGroup) {
	r.PUT("/tours/:id/building-scan", h.SaveBuildingScan)
	r.DELETE("/tours/:id/building-scan", h.DeleteBuildingScan)
}

// SaveBuildingScan stores a scan uploaded by the capture app on the tour.
func (h *Handler) SaveBuildingScan(c *gin.Context) {
	tour, ok := h.ownedTour(c)
	if !ok {
		return
	}

	var scan BuildingScan
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 1<<20)
	if err := c.ShouldBindJSON(&scan); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid scan data"})
		return
	}
	if msg := validateScan(&scan); msg != "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": msg})
		return
	}

	// Only link rooms to photos that belong to this tour
	scenes, _, err := h.tours.ListScenes(tour.ID, 1, maxSceneOptions)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not load the tour's photos."})
		return
	}
	valid := map[string]bool{}
	for _, scene := range scenes {
		valid[scene.ID] = true
	}
	used := map[string]bool{}
	for i := range scan.Rooms {
		id := scan.Rooms[i].SceneID
		if !valid[id] || used[id] {
			scan.Rooms[i].SceneID = ""
		} else {
			used[id] = true
		}
	}

	scan.Version = 1
	if scan.CapturedAt.IsZero() {
		scan.CapturedAt = time.Now().UTC()
	}
	raw, err := json.Marshal(scan)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not save the scan."})
		return
	}
	tour.BuildingScan = raw
	if err := h.tours.UpdateTour(tour); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not save the scan."})
		return
	}
	c.JSON(http.StatusOK, scan)
}

func (h *Handler) DeleteBuildingScan(c *gin.Context) {
	tour, ok := h.ownedTour(c)
	if !ok {
		return
	}
	tour.BuildingScan = nil
	if err := h.tours.UpdateTour(tour); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not remove the scan."})
		return
	}
	c.Status(http.StatusNoContent)
}

func validateScan(scan *BuildingScan) string {
	if len(scan.Rooms) == 0 {
		return "The scan has no rooms."
	}
	if len(scan.Rooms) > maxScanRooms {
		return "The scan has too many rooms."
	}
	finite := func(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) && math.Abs(v) <= maxCoordinate }
	for i := range scan.Rooms {
		room := &scan.Rooms[i]
		room.Name = strings.TrimSpace(room.Name)
		if len(room.Name) > 80 {
			room.Name = room.Name[:80]
		}
		if len(room.Corners) < 3 || len(room.Corners) > maxRoomCorners {
			return "Each room needs between 3 and 64 corners."
		}
		for _, p := range room.Corners {
			if !finite(p[0]) || !finite(p[1]) {
				return "The scan has an invalid corner."
			}
		}
		if room.PhotoSpot != nil && (!finite(room.PhotoSpot[0]) || !finite(room.PhotoSpot[1])) {
			room.PhotoSpot = nil
		}
		if room.CeilingHeight < 1.8 || room.CeilingHeight > 8 {
			room.CeilingHeight = 0
		}
	}
	if len(scan.Device) > 120 {
		scan.Device = scan.Device[:120]
	}
	return ""
}
