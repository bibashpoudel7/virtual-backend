package models

// Property represents a property from the main Nimto system
type Property struct {
	ID           string `json:"id" gorm:"primaryKey;column:id"`
	PropertyName string `json:"propertyName" gorm:"column:propertyName"`
	Slug         string `json:"slug" gorm:"column:slug"`
	PropertyType string `json:"propertyType" gorm:"column:propertyType"`
	CompanyID    string `json:"companyId" gorm:"column:companyId"`
	Status       int    `json:"status" gorm:"column:status"` // 1 = approved
	
	// Company details for display
	Company *Company `json:"company,omitempty" gorm:"foreignKey:CompanyID;references:ID"`
	
	BaseModel
}

// Company represents vendor company details
type Company struct {
	ID                     string `json:"id" gorm:"primaryKey;column:id"`
	FullLegalCompanyName   string `json:"fullLegalCompanyName" gorm:"column:fullLegalCompanyName"`
	OperatingName          string `json:"operatingName" gorm:"column:operatingName"`
	
	BaseModel
}

// PropertyResponse for API responses
type PropertyResponse struct {
	ID           string `json:"id"`
	PropertyName string `json:"propertyName"`
	Slug         string `json:"slug"`
	PropertyType string `json:"propertyType"`
	CompanyName  string `json:"companyName"`
	HasTour      bool   `json:"hasTour"`
}

// CompanyInfo for user's company information
type CompanyInfo struct {
	ID             string `json:"id"`
	CompanyName    string `json:"companyName"`
	CompanyPurpose string `json:"companyPurpose"`
	IsVenueCompany bool   `json:"isVenueCompany"`
}