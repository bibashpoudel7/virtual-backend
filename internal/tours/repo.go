// backend/tours/repo.go
package tours

import (
	"encoding/json"

	"backend/internal/models"
	"backend/internal/pkg/uuid"

	"gorm.io/gorm"
)

type Repository interface {
	Create(tour *models.Tour) error
	FindByID(id string) (*models.Tour, error)
	FindByStringID(id string) (*models.Tour, error)
	UpdateTour(tour *models.Tour) error
	DeleteTour(id string) error
	FindByProperty(propertyID string) ([]models.Tour, error)
	ListAllTours(page, limit int) ([]models.Tour, int64, error)
	ListAllPublicTours() ([]models.Tour, error)
	ListUserTours(userID string, page, limit int) ([]models.Tour, int64, error)
	CountUserTours(userID string) (int64, error)
	UnfeatureAllTours() error
	GetFeaturedTour() (*models.Tour, error)

	CreateScene(scene *models.Scene) error
	GetScene(sceneID string) (*models.Scene, error)
	UpdateScene(scene *models.Scene) error
	DeleteScene(sceneID string) error
	ListScenes(tourID string, offset, limit int) ([]models.Scene, error)
	CountScenes(tourID string) (int64, error)

	CreateHotspot(tourID, sceneID, targetSceneID, kind string, yaw, pitch float64, payload map[string]any) (*models.Hotspot, error)
	GetHotspot(hotspotID string) (*models.Hotspot, error)
	ListHotspots(sceneID string) ([]models.Hotspot, error)
	UpdateHotspot(hotspot *models.Hotspot) error
	DeleteHotspot(hotspotID string) error

	// Overlay methods
	CreateOverlay(tourID, sceneID, kind string, yaw, pitch float64, payload map[string]any) (*models.Overlay, error)
	GetOverlay(overlayID string) (*models.Overlay, error)
	ListOverlays(sceneID string) ([]models.Overlay, error)
	UpdateOverlay(overlay *models.Overlay) error
	DeleteOverlay(overlayID string) error

	GetTourByPropertyID(propertyID string) (*models.Tour, error)

	// Check if any tour is featured
	HasFeaturedTour() (bool, error)

	// PlayTour methods
	CreatePlayTour(playTour *models.PlayTour) error
	GetPlayTour(id string) (*models.PlayTour, error)
	UpdatePlayTour(playTour *models.PlayTour) error
	DeletePlayTour(id string) error
	ListPlayTours(tourID string) ([]models.PlayTour, error)
}

type tourRepository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) Repository {
	return &tourRepository{db: db}
}

func (r *tourRepository) Create(tour *models.Tour) error {
	return r.db.Create(tour).Error
}

func (r *tourRepository) FindByID(id string) (*models.Tour, error) {
	var tour models.Tour
	println("bibash", id)
	if err := r.db.Where("id = ?", id).First(&tour).Error; err != nil {
		return nil, err
	}
	return &tour, nil
}

func (r *tourRepository) FindByStringID(id string) (*models.Tour, error) {
	var tour models.Tour
	if err := r.db.Where("id = ?", id).First(&tour).Error; err != nil {
		return nil, err
	}
	return &tour, nil
}

func (r *tourRepository) FindByProperty(propertyID string) ([]models.Tour, error) {
	var tours []models.Tour
	if err := r.db.Where("property_id = ?", propertyID).Order("created_at desc").Find(&tours).Error; err != nil {
		return nil, err
	}

	// For each tour, get the actual scene count and thumbnail URL from the first scene
	for i := range tours {
		var sceneCount int64
		r.db.Table("scenes").Where("tour_id = ?", tours[i].ID).Count(&sceneCount)

		// Get the first scene's src_original_url for thumbnail
		var firstScene models.Scene
		if err := r.db.Where("tour_id = ?", tours[i].ID).Order("scene_order asc, created_at asc").First(&firstScene).Error; err == nil {
			tours[i].ThumbnailURL = firstScene.SrcOriginalURL
		}

		tours[i].TourScenes = make([]models.TourScene, sceneCount)
	}

	return tours, nil
}

func (r *tourRepository) GetScene(sceneID string) (*models.Scene, error) {
	var s models.Scene
	if err := r.db.Where("id = ?", sceneID).First(&s).Error; err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *tourRepository) UpdateSceneCubemap(sceneID string, manifestURL string) error {
	return r.db.Model(&models.Scene{}).Where("id = ?", sceneID).Update("cubemap_manifest_url", manifestURL).Error
}

func (r *tourRepository) CreateHotspot(tourID, sceneID, targetSceneID, kind string, yaw, pitch float64, payload map[string]any) (*models.Hotspot, error) {
	// Verify the scene exists and belongs to the specified tour
	var scene models.Scene
	if err := r.db.Select("id, tour_id").Where("id = ?", sceneID).First(&scene).Error; err != nil {
		return nil, err
	}

	// Ensure the scene belongs to the specified tour
	if scene.TourID != tourID {
		return nil, gorm.ErrInvalidData
	}

	// marshal payload to JSONB string
	b, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	hs := &models.Hotspot{
		ID:            uuid.GenerateUUIDv7(),
		TourID:        tourID,
		SceneID:       sceneID,
		TargetSceneID: targetSceneID,
		Kind:          kind,
		Yaw:           yaw,
		Pitch:         pitch,
		Payload:       string(b),
	}

	if err := r.db.Create(hs).Error; err != nil {
		return nil, err
	}
	return hs, nil
}

func (r *tourRepository) ListHotspots(sceneID string) ([]models.Hotspot, error) {
	var out []models.Hotspot
	if err := r.db.Where("scene_id = ?", sceneID).Order("id asc").Find(&out).Error; err != nil {
		return nil, err
	}
	return out, nil
}

func (r *tourRepository) GetHotspot(hotspotID string) (*models.Hotspot, error) {
	var hotspot models.Hotspot
	if err := r.db.Where("id = ?", hotspotID).First(&hotspot).Error; err != nil {
		return nil, err
	}
	return &hotspot, nil
}

func (r *tourRepository) UpdateTour(tour *models.Tour) error {
	return r.db.Save(tour).Error
}

func (r *tourRepository) DeleteTour(id string) error {
	// Start a transaction to ensure all deletes succeed or fail together
	tx := r.db.Begin()
	if tx.Error != nil {
		return tx.Error
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// First, delete all hotspots for scenes belonging to this tour
	if err := tx.Exec("DELETE FROM hotspots WHERE scene_id IN (SELECT id FROM scenes WHERE tour_id = ?)", id).Error; err != nil {
		tx.Rollback()
		return err
	}

	// Delete all overlays for scenes belonging to this tour
	if err := tx.Exec("DELETE FROM overlays WHERE scene_id IN (SELECT id FROM scenes WHERE tour_id = ?)", id).Error; err != nil {
		tx.Rollback()
		return err
	}

	// Then, delete all scenes belonging to this tour
	if err := tx.Delete(&models.Scene{}, "tour_id = ?", id).Error; err != nil {
		tx.Rollback()
		return err
	}

	// Finally, delete the tour itself
	if err := tx.Delete(&models.Tour{}, "id = ?", id).Error; err != nil {
		tx.Rollback()
		return err
	}

	return tx.Commit().Error
}

func (r *tourRepository) ListAllTours(page, limit int) ([]models.Tour, int64, error) {
	var tours []models.Tour
	var total int64

	// Get total count
	if err := r.db.Model(&models.Tour{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Calculate offset
	offset := (page - 1) * limit
	if offset < 0 {
		offset = 0
	}

	// Get paginated tours
	if err := r.db.Order("created_at desc").Limit(limit).Offset(offset).Find(&tours).Error; err != nil {
		return nil, 0, err
	}

	// For each tour, get the actual scene count and thumbnail URL from the first scene
	for i := range tours {
		var sceneCount int64
		r.db.Table("scenes").Where("tour_id = ?", tours[i].ID).Count(&sceneCount)

		// Get the first scene's src_original_url for thumbnail
		var firstScene models.Scene
		if err := r.db.Where("tour_id = ?", tours[i].ID).Order("scene_order asc, created_at asc").First(&firstScene).Error; err == nil {
			tours[i].ThumbnailURL = firstScene.SrcOriginalURL
		}

		tours[i].TourScenes = make([]models.TourScene, sceneCount)
	}

	return tours, total, nil
}

func (r *tourRepository) ListAllPublicTours() ([]models.Tour, error) {
	var tours []models.Tour
	if err := r.db.Where("is_published = ?", true).Find(&tours).Error; err != nil {
		return nil, err
	}

	// For each tour, get the actual scene count and thumbnail URL from the first scene
	for i := range tours {
		var sceneCount int64
		r.db.Table("scenes").Where("tour_id = ?", tours[i].ID).Count(&sceneCount)

		// Get the first scene's src_original_url for thumbnail
		var firstScene models.Scene
		if err := r.db.Where("tour_id = ?", tours[i].ID).Order("scene_order asc, created_at asc").First(&firstScene).Error; err == nil {
			tours[i].ThumbnailURL = firstScene.SrcOriginalURL
		}

		// Create TourScene entries to represent the scene count
		// This is a workaround since the frontend expects tour_scenes array
		tours[i].TourScenes = make([]models.TourScene, sceneCount)
		for j := int64(0); j < sceneCount; j++ {
			tours[i].TourScenes[j] = models.TourScene{
				TourID:        tours[i].ID,
				SequenceOrder: int(j + 1),
			}
		}
	}

	return tours, nil
}

func (r *tourRepository) ListUserTours(userID string, page, limit int) ([]models.Tour, int64, error) {
	var tours []models.Tour
	var total int64

	// Get total count for user
	if err := r.db.Model(&models.Tour{}).Where("user_id = ?", userID).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Calculate offset
	offset := (page - 1) * limit
	if offset < 0 {
		offset = 0
	}

	// Get paginated tours for user
	if err := r.db.Where("user_id = ?", userID).Order("created_at desc").Limit(limit).Offset(offset).Find(&tours).Error; err != nil {
		return nil, 0, err
	}

	// For each tour, get the actual scene count and thumbnail URL from the first scene
	for i := range tours {
		var sceneCount int64
		r.db.Table("scenes").Where("tour_id = ?", tours[i].ID).Count(&sceneCount)

		// Get the first scene's src_original_url for thumbnail
		var firstScene models.Scene
		if err := r.db.Where("tour_id = ?", tours[i].ID).Order("scene_order asc, created_at asc").First(&firstScene).Error; err == nil {
			tours[i].ThumbnailURL = firstScene.SrcOriginalURL
		}

		tours[i].TourScenes = make([]models.TourScene, sceneCount)
	}

	return tours, total, nil
}

func (r *tourRepository) GetFeaturedTour() (*models.Tour, error) {
	var tour models.Tour
	err := r.db.Where("is_featured_on_homepage = ?", true).First(&tour).Error
	if err != nil {
		return nil, err
	}

	// Get the actual scene count and thumbnail URL from the first scene
	var sceneCount int64
	r.db.Table("scenes").Where("tour_id = ?", tour.ID).Count(&sceneCount)

	// Get the first scene's src_original_url for thumbnail
	var firstScene models.Scene
	if err := r.db.Where("tour_id = ?", tour.ID).Order("scene_order asc, created_at asc").First(&firstScene).Error; err == nil {
		tour.ThumbnailURL = firstScene.SrcOriginalURL
	}

	tour.TourScenes = make([]models.TourScene, sceneCount)

	return &tour, nil
}

func (r *tourRepository) UnfeatureAllTours() error {
	return r.db.Model(&models.Tour{}).Where("is_featured_on_homepage = ?", true).Update("is_featured_on_homepage", false).Error
}

func (r *tourRepository) CreateScene(scene *models.Scene) error {
	scene.ID = uuid.GenerateUUIDv7()
	return r.db.Create(scene).Error
}

func (r *tourRepository) UpdateScene(scene *models.Scene) error {
	return r.db.Save(scene).Error
}

func (r *tourRepository) DeleteScene(sceneID string) error {
	// Start a transaction to ensure all deletes succeed or fail together
	tx := r.db.Begin()
	if tx.Error != nil {
		return tx.Error
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// First, delete all hotspots for this scene
	if err := tx.Delete(&models.Hotspot{}, "scene_id = ?", sceneID).Error; err != nil {
		tx.Rollback()
		return err
	}

	// Delete all overlays for this scene
	if err := tx.Delete(&models.Overlay{}, "scene_id = ?", sceneID).Error; err != nil {
		tx.Rollback()
		return err
	}

	// Finally, delete the scene itself
	if err := tx.Delete(&models.Scene{}, "id = ?", sceneID).Error; err != nil {
		tx.Rollback()
		return err
	}

	return tx.Commit().Error
}

func (r *tourRepository) ListScenes(tourID string, offset, limit int) ([]models.Scene, error) {
	var scenes []models.Scene
	query := r.db.Preload("Hotspots").Preload("Overlays").Where("tour_id = ?", tourID).Order("scene_order asc")
	if limit > 0 {
		query = query.Limit(limit).Offset(offset)
	}
	if err := query.Find(&scenes).Error; err != nil {
		return nil, err
	}
	return scenes, nil
}

func (r *tourRepository) CountScenes(tourID string) (int64, error) {
	var count int64
	if err := r.db.Model(&models.Scene{}).Where("tour_id = ?", tourID).Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

func (r *tourRepository) UpdateHotspot(hotspot *models.Hotspot) error {
	return r.db.Save(hotspot).Error
}

func (r *tourRepository) DeleteHotspot(hotspotID string) error {
	return r.db.Delete(&models.Hotspot{}, "id = ?", hotspotID).Error
}

func (r *tourRepository) CountUserTours(userID string) (int64, error) {
	var count int64
	err := r.db.Model(&models.Tour{}).Where("user_id = ?", userID).Count(&count).Error
	return count, err
}

// GetTourByPropertyID gets an existing tour for a property
func (r *tourRepository) GetTourByPropertyID(propertyID string) (*models.Tour, error) {
	var tour models.Tour
	err := r.db.Where("property_id = ?", propertyID).First(&tour).Error
	if err != nil {
		return nil, err
	}
	return &tour, nil
}

// HasFeaturedTour checks if any tour is currently featured
func (r *tourRepository) HasFeaturedTour() (bool, error) {
	var count int64
	err := r.db.Model(&models.Tour{}).Where("is_featured_on_homepage = ?", true).Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// Overlay repository methods
func (r *tourRepository) CreateOverlay(tourID, sceneID, kind string, yaw, pitch float64, payload map[string]any) (*models.Overlay, error) {
	// Verify the scene exists and belongs to the specified tour
	var scene models.Scene
	if err := r.db.Select("id, tour_id").Where("id = ?", sceneID).First(&scene).Error; err != nil {
		return nil, err
	}

	// Ensure the scene belongs to the specified tour
	if scene.TourID != tourID {
		return nil, gorm.ErrInvalidData
	}

	// marshal payload to JSONB string
	b, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	overlay := &models.Overlay{
		ID:      uuid.GenerateUUIDv7(),
		TourID:  tourID,
		SceneID: sceneID,
		Kind:    kind,
		Yaw:     yaw,
		Pitch:   pitch,
		Payload: string(b),
	}

	if err := r.db.Create(overlay).Error; err != nil {
		return nil, err
	}
	return overlay, nil
}

func (r *tourRepository) GetOverlay(overlayID string) (*models.Overlay, error) {
	var overlay models.Overlay
	if err := r.db.Where("id = ?", overlayID).First(&overlay).Error; err != nil {
		return nil, err
	}
	return &overlay, nil
}

func (r *tourRepository) ListOverlays(sceneID string) ([]models.Overlay, error) {
	var overlays []models.Overlay
	if err := r.db.Where("scene_id = ?", sceneID).Order("id asc").Find(&overlays).Error; err != nil {
		return nil, err
	}
	return overlays, nil
}

func (r *tourRepository) UpdateOverlay(overlay *models.Overlay) error {
	return r.db.Save(overlay).Error
}

func (r *tourRepository) DeleteOverlay(overlayID string) error {
	return r.db.Delete(&models.Overlay{}, "id = ?", overlayID).Error
}

// PlayTour repository implementations

func (r *tourRepository) CreatePlayTour(playTour *models.PlayTour) error {
	if playTour.ID == "" {
		playTour.ID = uuid.GenerateUUIDv7()
	}
	for i := range playTour.PlayTourScenes {
		if playTour.PlayTourScenes[i].ID == "" {
			playTour.PlayTourScenes[i].ID = uuid.GenerateUUIDv7()
		}
		playTour.PlayTourScenes[i].PlayTourID = playTour.ID
	}
	return r.db.Create(playTour).Error
}

func (r *tourRepository) GetPlayTour(id string) (*models.PlayTour, error) {
	var playTour models.PlayTour
	if err := r.db.Preload("PlayTourScenes", func(db *gorm.DB) *gorm.DB {
		return db.Order("sequence_order asc")
	}).Where("id = ?", id).First(&playTour).Error; err != nil {
		return nil, err
	}
	return &playTour, nil
}

func (r *tourRepository) UpdatePlayTour(playTour *models.PlayTour) error {
	// Use a transaction to handle play tour scenes updates
	return r.db.Transaction(func(tx *gorm.DB) error {
		// Update the play tour itself
		if err := tx.Save(playTour).Error; err != nil {
			return err
		}

		// Delete existing scenes that are not in the new list (simplified: delete all and re-add or handle carefully)
		// For simplicity in this implementation, we'll delete all current scenes and re-add them
		// if they have IDs or manage them individually if we want to be more efficient.
		// However, a common pattern for "sequence" management is replacing the whole set.

		if err := tx.Where("play_tour_id = ?", playTour.ID).Delete(&models.PlayTourScene{}).Error; err != nil {
			return err
		}

		for i := range playTour.PlayTourScenes {
			if playTour.PlayTourScenes[i].ID == "" {
				playTour.PlayTourScenes[i].ID = uuid.GenerateUUIDv7()
			}
			playTour.PlayTourScenes[i].PlayTourID = playTour.ID
			if err := tx.Create(&playTour.PlayTourScenes[i]).Error; err != nil {
				return err
			}
		}

		return nil
	})
}

func (r *tourRepository) DeletePlayTour(id string) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("play_tour_id = ?", id).Delete(&models.PlayTourScene{}).Error; err != nil {
			return err
		}
		return tx.Delete(&models.PlayTour{}, "id = ?", id).Error
	})
}

func (r *tourRepository) ListPlayTours(tourID string) ([]models.PlayTour, error) {
	var playTours []models.PlayTour
	if err := r.db.Preload("PlayTourScenes", func(db *gorm.DB) *gorm.DB {
		return db.Order("sequence_order asc")
	}).Where("tour_id = ?", tourID).Find(&playTours).Error; err != nil {
		return nil, err
	}
	return playTours, nil
}
