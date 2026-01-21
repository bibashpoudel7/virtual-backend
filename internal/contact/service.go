package contact

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"backend/internal/models"
	"backend/internal/email"
)

type Service struct {
	db           *gorm.DB
	emailService *email.Service
}

func NewService(db *gorm.DB) *Service {
	return &Service{
		db:           db,
		emailService: email.NewService(),
	}
}

// CreateContact creates a new contact form submission
func (s *Service) CreateContact(req *models.ContactRequest, ip string) (*models.Contact, error) {
	contact := &models.Contact{
		ID:          uuid.New().String(),
		FullName:    req.FullName,
		Email:       req.Email,
		CountryCode: &req.CountryCode,
		PhoneNumber: req.PhoneNumber,
		Message:     req.Message,
		Status:      1, // Active
		IP:          &ip,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	if err := s.db.Create(contact).Error; err != nil {
		return nil, fmt.Errorf("failed to create contact: %w", err)
	}

	// Send email notification (non-blocking)
	go func() {
		phoneNumber := req.CountryCode + " " + req.PhoneNumber
		if err := s.emailService.SendContactNotification(req.FullName, req.Email, phoneNumber, req.Message); err != nil {
			// Log error but don't fail the request
			fmt.Printf("Failed to send email notification: %v\n", err)
		}
	}()

	return contact, nil
}

// GetContacts retrieves all contact submissions (for admin use)
func (s *Service) GetContacts(limit, offset int) ([]models.Contact, int64, error) {
	var contacts []models.Contact
	var total int64

	// Get total count
	if err := s.db.Model(&models.Contact{}).Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count contacts: %w", err)
	}

	// Get contacts with pagination
	if err := s.db.Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&contacts).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to get contacts: %w", err)
	}

	return contacts, total, nil
}

// GetContactByID retrieves a specific contact by ID
func (s *Service) GetContactByID(id string) (*models.Contact, error) {
	var contact models.Contact
	if err := s.db.Where("id = ?", id).First(&contact).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("contact not found")
		}
		return nil, fmt.Errorf("failed to get contact: %w", err)
	}

	return &contact, nil
}

// UpdateContactStatus updates the status of a contact
func (s *Service) UpdateContactStatus(id string, status int) error {
	if err := s.db.Model(&models.Contact{}).
		Where("id = ?", id).
		Update("status", status).Error; err != nil {
		return fmt.Errorf("failed to update contact status: %w", err)
	}

	return nil
}