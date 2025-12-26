package users

import (
	"fmt"
	"gorm.io/gorm"
)

type Service interface {
	GetUserRole(userID string) (string, error)
}

type service struct {
	mainDB *gorm.DB // For querying users table in main database
}

func NewService(mainDB *gorm.DB) Service {
	return &service{
		mainDB: mainDB,
	}
}

// GetUserRole queries the main database to get user's role
func (s *service) GetUserRole(userID string) (string, error) {
	type UserRole struct {
		Roles string `json:"roles"`
	}
	
	var userRole UserRole
	
	// Query the users table in main database for the role
	err := s.mainDB.Table("users").
		Select("roles").
		Where("id = ?", userID).
		First(&userRole).Error
	
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return "", fmt.Errorf("user not found: %s", userID)
		}
		return "", fmt.Errorf("failed to query user role: %w", err)
	}
	
	switch userRole.Roles {
	case "1":
		return "1", nil // SUPERADMIN
	case "2":
		return "2", nil // CUSTOMER  
	case "3":
		return "3", nil // VENDOR
	default:
		// Return the raw value if it doesn't match expected enum values
		return userRole.Roles, nil
	}
}