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
	ListAllTours() ([]models.Tour, error)
	ListAllPublicTours() ([]models.Tour, error)
	ListUserTours(userID string) ([]models.Tour, error)
	CountUserTours(userID string) (int64, error)

	CreateScene(scene *models.Scene) error
	GetScene(sceneID string) (*models.Scene, error)
	UpdateScene(scene *models.Scene) error
	DeleteScene(sceneID string) error
	ListScenes(tourID string) ([]models.Scene, error)

	CreateHotspot(tourID, sceneID, targetSceneID, kind string, yaw, pitch float64, payload map[string]any) (*models.Hotspot, error)
	GetHotspot(hotspotID string) (*models.Hotspot, error)
	ListHotspots(sceneID string) ([]models.Hotspot, error)
	UpdateHotspot(hotspot *models.Hotspot) error
	DeleteHotspot(hotspotID string) error
	
	GetTourByPropertyID(propertyID string) (*models.Tour, error)
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
	if err := r.db.Where("property_id = ?", propertyID).Find(&tours).Error; err != nil {
		return nil, err
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

func (r *tourRepository) ListAllTours() ([]models.Tour, error) {
	var tours []models.Tour
	if err := r.db.Find(&tours).Error; err != nil {
		return nil, err
	}
	return tours, nil
}

func (r *tourRepository) ListAllPublicTours() ([]models.Tour, error) {
	var tours []models.Tour
	if err := r.db.Where("is_published = ?", true).Find(&tours).Error; err != nil {
		return nil, err
	}
	
	// For each tour, get the actual scene count from the scenes table
	for i := range tours {
		var sceneCount int64
		r.db.Table("scenes").Where("tour_id = ?", tours[i].ID).Count(&sceneCount)
		
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

func (r *tourRepository) ListUserTours(userID string) ([]models.Tour, error) {
	var tours []models.Tour
	if err := r.db.Where("user_id = ?", userID).Find(&tours).Error; err != nil {
		return nil, err
	}
	return tours, nil
}

func (r *tourRepository) CreateScene(scene *models.Scene) error {
	scene.ID = uuid.GenerateUUIDv7()
	return r.db.Create(scene).Error
}

func (r *tourRepository) UpdateScene(scene *models.Scene) error {
	return r.db.Save(scene).Error
}

func (r *tourRepository) DeleteScene(sceneID string) error {
	return r.db.Delete(&models.Scene{}, "id = ?", sceneID).Error
}

func (r *tourRepository) ListScenes(tourID string) ([]models.Scene, error) {
	var scenes []models.Scene
	if err := r.db.Where("tour_id = ?", tourID).Order("scene_order asc").Find(&scenes).Error; err != nil {
		return nil, err
	}
	return scenes, nil
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
