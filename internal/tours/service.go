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
	ListAllTours() ([]models.Tour, error)

	// Scene management
	CreateScene(scene *models.Scene) error
	GetScene(sceneID string) (*models.Scene, error)
	UpdateScene(scene *models.Scene) error
	DeleteScene(sceneID string) error
	ListScenes(tourID string) ([]models.Scene, error)

	// NEW:
	CreateHotspot(
		sceneID,
		tourID,
		targetSceneID,
		kind string,
		yaw,
		pitch float64,
		payload map[string]any,
	) (*models.Hotspot, error)
	ListHotspots(sceneID string) ([]models.Hotspot, error)
	UpdateHotspot(hotspot *models.Hotspot) error
	DeleteHotspot(hotspotID string) error

	// Payment related
	CheckIfPaymentRequired(userID string) (bool, error)
	CreatePaymentSession(tour *models.Tour) (*models.PaymentSession, error)
}

type service struct {
	repo Repository
}

func NewService(db *gorm.DB, cfg config.Config) (Service, error) {
	repo := NewRepository(db)
	return &service{
		repo: repo,
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

func (s *service) UpdateTour(tour *models.Tour) error {
	return s.repo.UpdateTour(tour)
}

func (s *service) DeleteTour(id string) error {
	return s.repo.DeleteTour(id)
}

func (s *service) ListAllTours() ([]models.Tour, error) {
	return s.repo.ListAllTours()
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

func (s *service) ListScenes(tourID string) ([]models.Scene, error) {
	return s.repo.ListScenes(tourID)
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
