// backend/tours/service.go
package tours

import (
	"backend/internal/config"
	"backend/internal/models"
	"errors"

	"gorm.io/gorm"
)

var (
	ErrInvalidInput  = errors.New("invalid input")
	ErrNotConfigured = errors.New("feature not configured")
)

type Service interface {
	CreateTour(tour *models.Tour) error
	GetTour(id string) (*models.Tour, error)
	UpdateTour(tour *models.Tour) error
	DeleteTour(id string) error
	ListTours(propertyID string) ([]models.Tour, error)
	ListAllTours() ([]models.TourWithProperty, error)
	ListAllPublicTours() ([]models.TourWithProperty, error)
	ListUserTours(userID string) ([]models.Tour, error)
	UnfeatureAllTours() error

	// Scene management
	CreateScene(scene *models.Scene) error
	GetScene(sceneID string) (*models.Scene, error)
	UpdateScene(scene *models.Scene) error
	DeleteScene(sceneID string) error
	ListScenes(tourID string, page, limit int) ([]models.Scene, int64, error)

	// Hotspot methods
	CreateHotspot(
		sceneID,
		tourID,
		targetSceneID,
		kind string,
		yaw,
		pitch float64,
		payload map[string]any,
	) (*models.Hotspot, error)
	GetHotspot(hotspotID string) (*models.Hotspot, error)
	ListHotspots(sceneID string) ([]models.Hotspot, error)
	UpdateHotspot(hotspot *models.Hotspot) error
	DeleteHotspot(hotspotID string) error

	// Overlay methods
	CreateOverlay(
		sceneID,
		tourID,
		kind string,
		yaw,
		pitch float64,
		payload map[string]any,
	) (*models.Overlay, error)
	GetOverlay(overlayID string) (*models.Overlay, error)
	ListOverlays(sceneID string) ([]models.Overlay, error)
	UpdateOverlay(overlay *models.Overlay) error
	DeleteOverlay(overlayID string) error

	// Payment related
	CheckIfPaymentRequired(userID string) (bool, error)
	CreatePaymentSession(tour *models.Tour) (*models.PaymentSession, error)

	// Property validation
	ValidateVendorPropertyAccess(userID string, propertyID string) error
	GetTourByPropertyID(propertyID string) (*models.Tour, error)

	// PlayTour methods
	CreatePlayTour(playTour *models.PlayTour) error
	GetPlayTour(id string) (*models.PlayTour, error)
	UpdatePlayTour(playTour *models.PlayTour) error
	DeletePlayTour(id string) error
	ListPlayTours(tourID string) ([]models.PlayTour, error)
}

type service struct {
	repo   Repository
	mainDB *gorm.DB // For property validation queries
}

func NewService(virtualDB, mainDB *gorm.DB, cfg config.Config) (Service, error) {
	repo := NewRepository(virtualDB)
	return &service{
		repo:   repo,
		mainDB: mainDB,
	}, nil
}

func (s *service) CreateTour(tour *models.Tour) error {

	return s.repo.Create(tour)
}

func (s *service) GetTour(id string) (*models.Tour, error) {
	return s.repo.FindByID(id)
}

func (s *service) ListTours(propertyID string) ([]models.Tour, error) {
	return s.repo.FindByProperty(propertyID)
}

func (s *service) CreateHotspot(sceneID, tourID, targetSceneID, kind string, yaw, pitch float64, payload map[string]any) (*models.Hotspot, error) {
	// Store targetSceneID in the payload if needed
	if targetSceneID != "" {
		payload["targetSceneId"] = targetSceneID
	}
	return s.repo.CreateHotspot(tourID, sceneID, targetSceneID, kind, yaw, pitch, payload)
}

func (s *service) ListHotspots(sceneID string) ([]models.Hotspot, error) {
	return s.repo.ListHotspots(sceneID)
}

func (s *service) GetHotspot(hotspotID string) (*models.Hotspot, error) {
	return s.repo.GetHotspot(hotspotID)
}

func (s *service) UpdateTour(tour *models.Tour) error {
	return s.repo.UpdateTour(tour)
}

func (s *service) DeleteTour(id string) error {
	return s.repo.DeleteTour(id)
}

func (s *service) ListAllTours() ([]models.TourWithProperty, error) {
	// Get all tours from virtual database
	tours, err := s.repo.ListAllTours()
	if err != nil {
		return nil, err
	}

	// Convert to TourWithProperty and fetch property names
	var toursWithProperty []models.TourWithProperty
	for _, tour := range tours {
		tourWithProp := models.TourWithProperty{Tour: tour}

		// If tour has a property_id, fetch the property name from main database
		if tour.PropertyID != nil && *tour.PropertyID != "" {
			var propertyName string
			err := s.mainDB.Table("properties").
				Select("property_name").
				Where("id = ?", *tour.PropertyID).
				Scan(&propertyName).Error

			if err == nil && propertyName != "" {
				tourWithProp.PropertyName = &propertyName
			}
		}

		toursWithProperty = append(toursWithProperty, tourWithProp)
	}

	return toursWithProperty, nil
}

func (s *service) ListAllPublicTours() ([]models.TourWithProperty, error) {
	// Get all published tours from virtual database
	tours, err := s.repo.ListAllPublicTours()
	if err != nil {
		return nil, err
	}

	// Convert to TourWithProperty and fetch property names
	var toursWithProperty []models.TourWithProperty
	for _, tour := range tours {
		tourWithProp := models.TourWithProperty{Tour: tour}

		// If tour has a property_id, fetch the property name from main database
		if tour.PropertyID != nil && *tour.PropertyID != "" {
			var propertyName string
			err := s.mainDB.Table("properties").
				Select("property_name").
				Where("id = ?", *tour.PropertyID).
				Scan(&propertyName).Error

			if err == nil && propertyName != "" {
				tourWithProp.PropertyName = &propertyName
			}
		}

		toursWithProperty = append(toursWithProperty, tourWithProp)
	}

	return toursWithProperty, nil
}

func (s *service) ListUserTours(userID string) ([]models.Tour, error) {
	return s.repo.ListUserTours(userID)
}

func (s *service) UnfeatureAllTours() error {
	return s.repo.UnfeatureAllTours()
}

func (s *service) CreateScene(scene *models.Scene) error {
	return s.repo.CreateScene(scene)
}

func (s *service) GetScene(sceneID string) (*models.Scene, error) {
	return s.repo.GetScene(sceneID)
}

func (s *service) UpdateScene(scene *models.Scene) error {
	return s.repo.UpdateScene(scene)
}

func (s *service) DeleteScene(sceneID string) error {
	return s.repo.DeleteScene(sceneID)
}

func (s *service) ListScenes(tourID string, page, limit int) ([]models.Scene, int64, error) {
	offset := (page - 1) * limit
	if offset < 0 {
		offset = 0
	}
	scenes, err := s.repo.ListScenes(tourID, offset, limit)
	if err != nil {
		return nil, 0, err
	}
	count, err := s.repo.CountScenes(tourID)
	if err != nil {
		return scenes, 0, err
	}
	return scenes, count, nil
}

func (s *service) UpdateHotspot(hotspot *models.Hotspot) error {
	return s.repo.UpdateHotspot(hotspot)
}

func (s *service) DeleteHotspot(hotspotID string) error {
	return s.repo.DeleteHotspot(hotspotID)
}

func (s *service) CheckIfPaymentRequired(userID string) (bool, error) {
	// Count user's tours
	tourCount, err := s.repo.CountUserTours(userID)
	if err != nil {
		return false, err
	}

	// First tour is free
	if tourCount < 1 {
		return false, nil
	}

	// Check if user has active subscription (simplified - always false for now)
	// In production, this would check a subscriptions table
	return true, nil
}

func (s *service) CreatePaymentSession(tour *models.Tour) (*models.PaymentSession, error) {
	// Create a simple payment session for local handling
	session := &models.PaymentSession{
		ID:     "session_" + tour.ID,
		URL:    "/payment/checkout?tour=" + tour.ID,
		TourID: tour.ID,
		UserID: tour.UserID,
		Amount: 2999, // $29.99 in cents
		Status: "pending",
	}

	return session, nil
}

// ValidateVendorPropertyAccess checks if a vendor has access to a specific property
func (s *service) ValidateVendorPropertyAccess(userID string, propertyID string) error {
	var count int64

	// Check if the property belongs to a company created by this user
	query := `
		SELECT COUNT(*) 
		FROM properties p 
		JOIN companies c ON p.company_id = c.id 
		WHERE p.id = ? 
		AND c.created_by_user_id = ? 
		AND p.approval_status = ?
		AND p.property_type = ?
		AND c.approved = true
		AND c.status = ?
	`

	// Status.APPROVED = 1 (for both property approval_status and company status)
	// PropertyType.VENUE = 0
	err := s.mainDB.Raw(query, propertyID, userID, 1, 0, 1).Scan(&count).Error
	if err != nil {
		return err
	}

	if count == 0 {
		return gorm.ErrRecordNotFound
	}

	return nil
}

// GetTourByPropertyID gets an existing tour for a property
func (s *service) GetTourByPropertyID(propertyID string) (*models.Tour, error) {
	return s.repo.GetTourByPropertyID(propertyID)
}

// Overlay service methods
func (s *service) CreateOverlay(sceneID, tourID, kind string, yaw, pitch float64, payload map[string]any) (*models.Overlay, error) {
	return s.repo.CreateOverlay(tourID, sceneID, kind, yaw, pitch, payload)
}

func (s *service) GetOverlay(overlayID string) (*models.Overlay, error) {
	return s.repo.GetOverlay(overlayID)
}

func (s *service) ListOverlays(sceneID string) ([]models.Overlay, error) {
	return s.repo.ListOverlays(sceneID)
}

func (s *service) UpdateOverlay(overlay *models.Overlay) error {
	return s.repo.UpdateOverlay(overlay)
}

func (s *service) DeleteOverlay(overlayID string) error {
	return s.repo.DeleteOverlay(overlayID)
}

// PlayTour service implementations

func (s *service) CreatePlayTour(playTour *models.PlayTour) error {
	return s.repo.CreatePlayTour(playTour)
}

func (s *service) GetPlayTour(id string) (*models.PlayTour, error) {
	return s.repo.GetPlayTour(id)
}

func (s *service) UpdatePlayTour(playTour *models.PlayTour) error {
	return s.repo.UpdatePlayTour(playTour)
}

func (s *service) DeletePlayTour(id string) error {
	return s.repo.DeletePlayTour(id)
}

func (s *service) ListPlayTours(tourID string) ([]models.PlayTour, error) {
	return s.repo.ListPlayTours(tourID)
}
