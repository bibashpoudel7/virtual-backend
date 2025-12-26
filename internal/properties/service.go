package properties

import (
	"backend/internal/models"
	"fmt"
	"gorm.io/gorm"
)

type Service interface {
	GetVendorApprovedProperties(userID string) ([]models.PropertyResponse, error)
	GetAllApprovedVenueProperties() ([]models.PropertyResponse, error)
	GetPropertyByID(propertyID string) (*models.Property, error)
	GetUserCompanyInfo(userID string) (*models.CompanyInfo, error)
	GetTourByPropertyID(propertyID string) (*models.Tour, error)
	GetScenesByTourID(tourID string) ([]models.Scene, error)
}

type service struct {
	virtualDB *gorm.DB // For tours data
	mainDB    *gorm.DB // For users, companies, properties data
}

func NewService(virtualDB, mainDB *gorm.DB) Service {
	return &service{
		virtualDB: virtualDB,
		mainDB:    mainDB,
	}
}

// GetVendorApprovedProperties gets approved venue properties for a specific vendor
func (s *service) GetVendorApprovedProperties(userID string) ([]models.PropertyResponse, error) {
	var properties []models.PropertyResponse
	
	// First, get the company info for this user from main database
	type CompanyResult struct {
		ID             string `json:"id"`
		CompanyPurpose string `json:"company_purpose"`
	}
	var company CompanyResult
	
	// Status.APPROVED = 1 in the enum
	err := s.mainDB.Table("companies").
		Select("id, company_purpose").
		Where("created_by_user_id = ? AND approved = true AND status = ?", userID, 1).
		First(&company).Error
	
	if err != nil {
		// Let's try without the approved condition to see if company exists
		var companyExists CompanyResult
		existsErr := s.mainDB.Table("companies").
			Select("id, company_purpose").
			Where("created_by_user_id = ?", userID).
			First(&companyExists).Error
		
		if existsErr != nil {
			return nil, fmt.Errorf("no company found for user %s: %w", userID, err)
		} else {
			return nil, fmt.Errorf("company found for user %s but not approved: %w", userID, err)
		}
	}
	
	// Check if this is a VENUE company (PropertyType.VENUE = 0)
	if company.CompanyPurpose != "0" {
		return []models.PropertyResponse{}, nil // Return empty array for non-venue companies
	}
	
	// Get approved venue properties for this company from main database
	query := `
		SELECT 
			p.id,
			p.property_name,
			p.slug,
			p.property_type,
			c.full_legal_company_name
		FROM properties p
		LEFT JOIN companies c ON p.company_id = c.id
		WHERE p.company_id = ? 
		AND p.approval_status = '1'
		AND p.property_type = '0'
		ORDER BY p.property_name ASC
	`
	
	// Execute query on main database and scan into a temporary struct
	type TempProperty struct {
		ID                     string `json:"id"`
		PropertyName           string `json:"property_name"`
		Slug                   string `json:"slug"`
		PropertyType           string `json:"property_type"`
		FullLegalCompanyName   string `json:"full_legal_company_name"`
	}
	
	var tempProperties []TempProperty
	err = s.mainDB.Raw(query, company.ID).Scan(&tempProperties).Error
	if err != nil {
		return nil, fmt.Errorf("failed to fetch vendor properties: %w", err)
	}
	
	// Convert to PropertyResponse format
	for _, temp := range tempProperties {
		prop := models.PropertyResponse{
			ID:           temp.ID,
			PropertyName: temp.PropertyName,
			Slug:         temp.Slug,
			PropertyType: temp.PropertyType,
			CompanyName:  temp.FullLegalCompanyName,
			HasTour:      false, 
		}
		properties = append(properties, prop)
	}
	
	// Update has_tour status by checking virtual database
	for i := range properties {
		var tourCount int64
		s.virtualDB.Table("tours").Where("property_id = ?", properties[i].ID).Count(&tourCount)
		properties[i].HasTour = tourCount > 0
	}
	
	return properties, nil
}

// GetAllApprovedVenueProperties gets all approved venue properties (for superadmin)
func (s *service) GetAllApprovedVenueProperties() ([]models.PropertyResponse, error) {
	var properties []models.PropertyResponse
	
	query := `
		SELECT 
			p.id,
			p.property_name,
			p.slug,
			p.property_type,
			c.full_legal_company_name
		FROM properties p
		LEFT JOIN companies c ON p.company_id = c.id
		WHERE p.approval_status = '1'
		AND p.property_type = '0'
		ORDER BY c.full_legal_company_name ASC, p.property_name ASC
	`
	
	// Execute query on main database and scan into a temporary struct
	type TempProperty struct {
		ID                     string `json:"id"`
		PropertyName           string `json:"property_name"`
		Slug                   string `json:"slug"`
		PropertyType           string `json:"property_type"`
		FullLegalCompanyName   string `json:"full_legal_company_name"`
	}
	
	var tempProperties []TempProperty
	err := s.mainDB.Raw(query).Scan(&tempProperties).Error
	if err != nil {
		return nil, fmt.Errorf("failed to fetch all properties: %w", err)
	}
	
	// Convert to PropertyResponse format
	for _, temp := range tempProperties {
		prop := models.PropertyResponse{
			ID:           temp.ID,
			PropertyName: temp.PropertyName,
			Slug:         temp.Slug,
			PropertyType: temp.PropertyType,
			CompanyName:  temp.FullLegalCompanyName,
			HasTour:      false, // Will be updated below
		}
		properties = append(properties, prop)
	}
	
	// Update has_tour status by checking virtual database
	for i := range properties {
		var tourCount int64
		s.virtualDB.Table("tours").Where("property_id = ?", properties[i].ID).Count(&tourCount)
		properties[i].HasTour = tourCount > 0
	}
	
	return properties, nil
}

// GetUserCompanyInfo gets the user's company information
func (s *service) GetUserCompanyInfo(userID string) (*models.CompanyInfo, error) {
	type CompanyResult struct {
		ID                   string `json:"id"`
		FullLegalCompanyName string `json:"full_legal_company_name"`
		CompanyPurpose       string `json:"company_purpose"`
	}
	var company CompanyResult
	
	err := s.mainDB.Table("companies").
		Select("id, full_legal_company_name, company_purpose").
		Where("created_by_user_id = ? AND approved = true AND status = ?", userID, 1).
		First(&company).Error
	
	if err != nil {
		return nil, fmt.Errorf("no approved company found for user %s: %w", userID, err)
	}
	
	// Check if this is a VENUE company (PropertyType.VENUE = 0)
	isVenueCompany := company.CompanyPurpose == "0"
	
	return &models.CompanyInfo{
		ID:             company.ID,
		CompanyName:    company.FullLegalCompanyName,
		CompanyPurpose: company.CompanyPurpose,
		IsVenueCompany: isVenueCompany,
	}, nil
}

// GetPropertyByID gets a property by ID for validation
func (s *service) GetPropertyByID(propertyID string) (*models.Property, error) {
	var property models.Property
	
	err := s.mainDB.Preload("Company").
		Where("id = ? AND approval_status = ? AND property_type = ?", propertyID, "1", "0").
		First(&property).Error
	
	if err != nil {
		return nil, fmt.Errorf("property not found: %w", err)
	}
	
	return &property, nil
}

// GetTourByPropertyID gets a tour by property ID
func (s *service) GetTourByPropertyID(propertyID string) (*models.Tour, error) {
	var tour models.Tour
	
	err := s.virtualDB.Where("property_id = ?", propertyID).First(&tour).Error
	if err != nil {
		return nil, fmt.Errorf("tour not found for property %s: %w", propertyID, err)
	}
	
	return &tour, nil
}

// GetScenesByTourID gets all scenes for a tour
func (s *service) GetScenesByTourID(tourID string) ([]models.Scene, error) {
	var scenes []models.Scene
	
	err := s.virtualDB.Where("tour_id = ?", tourID).Order("order_index ASC").Find(&scenes).Error
	if err != nil {
		return nil, fmt.Errorf("failed to fetch scenes for tour %s: %w", tourID, err)
	}
	
	return scenes, nil
}