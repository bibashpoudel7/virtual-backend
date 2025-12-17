package payment

import (
	"backend/internal/models"
	"backend/internal/pkg/uuid"
	"errors"
	"time"

	"gorm.io/gorm"
)

type LocalPaymentService interface {
	CheckTourLimit(userID string) (canCreate bool, remainingFree int, error error)
	CreateLocalPayment(userID string, tourID string, amount int64) (*models.Payment, error)
	ProcessLocalPayment(paymentID string) error
	GetUserPayments(userID string) ([]models.Payment, error)
	ValidateUserSubscription(userID string) (bool, error)
}

type localService struct {
	db            *gorm.DB
	maxFreeTours  int
	tourPrice     int64
	monthlyPrice  int64
}

func NewLocalPaymentService(db *gorm.DB) LocalPaymentService {
	return &localService{
		db:           db,
		maxFreeTours: 1,      // 1 free tour
		tourPrice:    2999,    // $29.99 in cents
		monthlyPrice: 9999,    // $99.99 in cents
	}
}

func (s *localService) CheckTourLimit(userID string) (bool, int, error) {
	// Check if user has active subscription
	hasSubscription, err := s.ValidateUserSubscription(userID)
	if err != nil {
		return false, 0, err
	}
	
	if hasSubscription {
		return true, -1, nil // Unlimited with subscription
	}
	
	// Count user's tours
	var tourCount int64
	err = s.db.Model(&models.Tour{}).Where("user_id = ?", userID).Count(&tourCount).Error
	if err != nil {
		return false, 0, err
	}
	
	remainingFree := s.maxFreeTours - int(tourCount)
	if remainingFree > 0 {
		return true, remainingFree, nil
	}
	
	return false, 0, nil
}

func (s *localService) CreateLocalPayment(userID string, tourID string, amount int64) (*models.Payment, error) {
	payment := &models.Payment{
		ID:          uuid.GenerateUUIDv7(),
		UserID:      userID,
		TourID:      &tourID,
		Amount:      amount,
		Currency:    "usd",
		Status:      "pending",
		PaymentType: "tour_creation",
		Description: "Virtual Tour Creation",
	}
	
	err := s.db.Create(payment).Error
	if err != nil {
		return nil, err
	}
	
	return payment, nil
}

func (s *localService) ProcessLocalPayment(paymentID string) error {
	// In a real implementation, this would process payment through a payment gateway
	// For now, we'll just update the status
	err := s.db.Model(&models.Payment{}).
		Where("id = ?", paymentID).
		Update("status", "succeeded").Error
	
	if err != nil {
		return err
	}
	
	// Get the payment to update the tour
	var payment models.Payment
	err = s.db.Where("id = ?", paymentID).First(&payment).Error
	if err != nil {
		return err
	}
	
	// Update tour as paid
	if payment.TourID != nil {
		err = s.db.Model(&models.Tour{}).
			Where("id = ?", *payment.TourID).
			Updates(map[string]interface{}{
				"is_paid":    true,
				"payment_id": paymentID,
			}).Error
	}
	
	return err
}

func (s *localService) GetUserPayments(userID string) ([]models.Payment, error) {
	var payments []models.Payment
	err := s.db.Where("user_id = ?", userID).
		Order("created_at DESC").
		Find(&payments).Error
	
	return payments, err
}

func (s *localService) ValidateUserSubscription(userID string) (bool, error) {
	// Check if user has active subscription
	var subscription models.Subscription
	err := s.db.Where("user_id = ? AND status = ? AND expires_at > ?", 
		userID, "active", time.Now()).First(&subscription).Error
	
	if err == gorm.ErrRecordNotFound {
		return false, nil
	}
	
	if err != nil {
		return false, err
	}
	
	return true, nil
}

// CreateSubscription creates a new subscription for a user
func (s *localService) CreateSubscription(userID string, planType string) (*models.Subscription, error) {
	// Check if user already has a subscription
	var existing models.Subscription
	err := s.db.Where("user_id = ?", userID).First(&existing).Error
	if err == nil {
		return nil, errors.New("user already has a subscription")
	}
	
	subscription := &models.Subscription{
		ID:        uuid.GenerateUUIDv7(),
		UserID:    userID,
		PlanType:  planType,
		Status:    "active",
		StartedAt: time.Now(),
		ExpiresAt: time.Now().AddDate(0, 1, 0), // 1 month from now
		Features: `{
			"maxTours": -1,
			"maxScenesPerTour": 50,
			"customBranding": true,
			"analytics": true,
			"apiAccess": true
		}`,
	}
	
	err = s.db.Create(subscription).Error
	return subscription, err
}