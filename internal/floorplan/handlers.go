// Package floorplan reads an uploaded floor plan image with Claude and returns its rooms,
// which the frontend dollhouse builder turns into a 3D model of the property.
package floorplan

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/anthropics/anthropic-sdk-go/shared/constant"
	"github.com/gin-gonic/gin"

	"backend/internal/auth"
	"backend/internal/models"
	"backend/internal/tours"
)

const (
	model = "claude-opus-5-5"
	// The API accepts images up to 5 MB; the frontend downsizes plans before uploading
	maxImageBytes   = 5 * 1024 * 1024
	analyzeTimeout  = 4 * time.Minute
	maxSceneOptions = 200
)

var allowedImageTypes = map[string]anthropic.BetaBase64ImageSourceMediaType{
	"image/png":  anthropic.BetaBase64ImageSourceMediaTypeImagePNG,
	"image/jpeg": anthropic.BetaBase64ImageSourceMediaTypeImageJPEG,
	"image/webp": anthropic.BetaBase64ImageSourceMediaTypeImageWebP,
	"image/gif":  anthropic.BetaBase64ImageSourceMediaTypeImageGIF,
}

const systemPrompt = `You read architectural floor plans and return the rooms as polygons for building a 3D model.

Coordinates: give every corner as normalized image coordinates, x from 0 (left edge of the image) to 1 (right edge), y from 0 (top edge) to 1 (bottom edge). Measure from the whole image, including margins, title blocks and legends.

Rooms:
- Return every enclosed space a visitor could stand in: bedrooms, living areas, kitchens, bathrooms, halls, corridors, closets, balconies.
- Trace each room along the inside face of its walls, one corner per change of wall direction, going clockwise. Do not include door swings, furniture or dimension lines as corners.
- Open-plan areas without walls between them can be one room.
- Use the room's label from the plan as its name; if unlabeled, name it by its likely use.

Scale: set plan_width_meters to the real-world width covered by the full image width. Read it from dimension annotations or a scale bar (convert feet and inches to meters) and set scale_source to "dimensions". If the plan has no measurements, estimate from standard sizes (interior doors are about 0.8-0.9 m wide) and set scale_source to "estimated".

Ceiling height: set ceiling_height_meters only when the plan states it; otherwise use 0.

Photos: the user message lists the tour's 360° photos by id and name. Set scene_id to the id of the photo that was most likely taken in that room (matching by name, e.g. "Master Bedroom" for "Bedroom 1 (master)"), or "" when none fits. Use each photo for at most one room.

Put anything the user should double-check (unclear walls, guessed scale, multiple floors) in notes, in one or two short sentences.`

var outputSchema = map[string]any{
	"type": "object",
	"properties": map[string]any{
		"plan_width_meters":     map[string]any{"type": "number"},
		"scale_source":          map[string]any{"type": "string", "enum": []string{"dimensions", "estimated"}},
		"ceiling_height_meters": map[string]any{"type": "number"},
		"notes":                 map[string]any{"type": "string"},
		"rooms": map[string]any{
			"type": "array",
			"items": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"name":     map[string]any{"type": "string"},
					"scene_id": map[string]any{"type": "string"},
					"corners": map[string]any{
						"type": "array",
						"items": map[string]any{
							"type": "object",
							"properties": map[string]any{
								"x": map[string]any{"type": "number"},
								"y": map[string]any{"type": "number"},
							},
							"required":             []string{"x", "y"},
							"additionalProperties": false,
						},
					},
				},
				"required":             []string{"name", "scene_id", "corners"},
				"additionalProperties": false,
			},
		},
	},
	"required":             []string{"plan_width_meters", "scale_source", "ceiling_height_meters", "notes", "rooms"},
	"additionalProperties": false,
}

// Corner is a point in normalized image coordinates (0..1, origin top left)
type Corner struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

type Room struct {
	Name    string   `json:"name"`
	SceneID string   `json:"scene_id"`
	Corners []Corner `json:"corners"`
}

type Analysis struct {
	PlanWidthMeters     float64 `json:"plan_width_meters"`
	ScaleSource         string  `json:"scale_source"`
	CeilingHeightMeters float64 `json:"ceiling_height_meters"`
	Notes               string  `json:"notes"`
	Rooms               []Room  `json:"rooms"`
}

type Handler struct {
	tours  tours.Service
	client *anthropic.Client // nil when no API key is configured
}

func NewHandler(tourService tours.Service, apiKey string) *Handler {
	h := &Handler{tours: tourService}
	if apiKey != "" {
		client := anthropic.NewClient(option.WithAPIKey(apiKey))
		h.client = &client
	}
	return h
}

func (h *Handler) RegisterRoutes(r *gin.RouterGroup) {
	r.POST("/tours/:id/floorplan/analyze", h.Analyze)
}

// Analyze reads the rooms from an uploaded floor plan image (multipart field "image").
func (h *Handler) Analyze(c *gin.Context) {
	if h.client == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Floor plan reading is not set up on the server (ANTHROPIC_API_KEY is missing)."})
		return
	}

	tour, ok := h.ownedTour(c)
	if !ok {
		return
	}
	tourID := tour.ID

	file, err := c.FormFile("image")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Upload the floor plan as an image."})
		return
	}
	if file.Size > maxImageBytes {
		c.JSON(http.StatusBadRequest, gin.H{"error": "The image is larger than 5 MB."})
		return
	}
	reader, err := file.Open()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Could not read the image."})
		return
	}
	defer reader.Close()
	data, err := io.ReadAll(io.LimitReader(reader, maxImageBytes+1))
	if err != nil || len(data) > maxImageBytes {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Could not read the image."})
		return
	}
	mediaType, ok := allowedImageTypes[http.DetectContentType(data)]
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Use a PNG, JPEG, WebP or GIF image of the floor plan."})
		return
	}

	scenes, _, err := h.tours.ListScenes(tourID, 1, maxSceneOptions)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not load the tour's photos."})
		return
	}
	type sceneOption struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	options := make([]sceneOption, 0, len(scenes))
	validScene := map[string]bool{}
	for _, scene := range scenes {
		options = append(options, sceneOption{ID: scene.ID, Name: scene.Name})
		validScene[scene.ID] = true
	}
	optionsJSON, _ := json.Marshal(options)

	ctx, cancel := context.WithTimeout(c.Request.Context(), analyzeTimeout)
	defer cancel()
	analysis, err := h.analyze(ctx, base64.StdEncoding.EncodeToString(data), mediaType, string(optionsJSON))
	if err != nil {
		log.Printf("[floorplan] analyze tour %s: %v", tourID, err)
		var apiErr *anthropic.Error
		switch {
		case errors.Is(err, errRefused):
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "The floor plan could not be read. Try a clearer image."})
		case errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusTooManyRequests:
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Floor plan reading is busy. Try again in a minute."})
		case errors.Is(err, context.DeadlineExceeded):
			c.JSON(http.StatusGatewayTimeout, gin.H{"error": "Reading the floor plan took too long. Try again."})
		default:
			c.JSON(http.StatusBadGateway, gin.H{"error": "Reading the floor plan failed. Try again."})
		}
		return
	}

	// Drop anything unusable rather than trusting the model's output blindly
	rooms := make([]Room, 0, len(analysis.Rooms))
	usedScenes := map[string]bool{}
	for _, room := range analysis.Rooms {
		if len(room.Corners) < 3 {
			continue
		}
		for i := range room.Corners {
			room.Corners[i].X = clamp01(room.Corners[i].X)
			room.Corners[i].Y = clamp01(room.Corners[i].Y)
		}
		if !validScene[room.SceneID] || usedScenes[room.SceneID] {
			room.SceneID = ""
		}
		if room.SceneID != "" {
			usedScenes[room.SceneID] = true
		}
		rooms = append(rooms, room)
	}
	analysis.Rooms = rooms
	if analysis.PlanWidthMeters <= 0 || analysis.PlanWidthMeters > 500 {
		analysis.PlanWidthMeters = 15
		analysis.ScaleSource = "estimated"
	}
	if analysis.CeilingHeightMeters < 1.8 || analysis.CeilingHeightMeters > 8 {
		analysis.CeilingHeightMeters = 0
	}

	c.JSON(http.StatusOK, analysis)
}

// ownedTour loads the :id tour if the caller owns it or is an admin; otherwise it responds.
func (h *Handler) ownedTour(c *gin.Context) (*models.Tour, bool) {
	tour, err := h.tours.GetTour(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "tour not found"})
		return nil, false
	}
	if !auth.IsAdmin(c) && tour.UserID != c.GetString("user_id") {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return nil, false
	}
	return tour, true
}

var errRefused = errors.New("model declined the request")

func (h *Handler) analyze(ctx context.Context, imageBase64 string, mediaType anthropic.BetaBase64ImageSourceMediaType, scenesJSON string) (*Analysis, error) {
	response, err := h.client.Beta.Messages.New(ctx, anthropic.BetaMessageNewParams{
		Model:     model,
		MaxTokens: 16000,
		Betas:     []anthropic.AnthropicBeta{anthropic.AnthropicBetaServerSideFallback2026_07_01},
		// Re-serve a safety-classifier decline with a fallback model inside the same call
		Fallbacks: anthropic.BetaFallbacksParamUnion{OfDefault: constant.ValueOf[constant.Default]()},
		OutputConfig: anthropic.BetaOutputConfigParam{
			Effort: anthropic.BetaOutputConfigEffortHigh,
			Format: anthropic.BetaJSONOutputFormatParam{Schema: outputSchema},
		},
		System: []anthropic.BetaTextBlockParam{{Text: systemPrompt}},
		Messages: []anthropic.BetaMessageParam{
			anthropic.NewBetaUserMessage(
				anthropic.NewBetaImageBlock(anthropic.BetaBase64ImageSourceParam{Data: imageBase64, MediaType: mediaType}),
				anthropic.NewBetaTextBlock("The tour's 360° photos:\n"+scenesJSON+"\n\nRead the rooms from this floor plan."),
			),
		},
	})
	if err != nil {
		return nil, err
	}
	switch response.StopReason {
	case anthropic.BetaStopReasonRefusal:
		return nil, errRefused
	case anthropic.BetaStopReasonMaxTokens:
		return nil, errors.New("response hit max_tokens")
	}

	for _, block := range response.Content {
		if text, ok := block.AsAny().(anthropic.BetaTextBlock); ok {
			var analysis Analysis
			if err := json.Unmarshal([]byte(text.Text), &analysis); err != nil {
				return nil, err
			}
			return &analysis, nil
		}
	}
	return nil, errors.New("response had no text block")
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}
