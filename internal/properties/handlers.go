package properties

import (
	"backend/internal/models"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	service Service
}

func NewHandler(service Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) RegisterRoutes(r *gin.RouterGroup) {
	propertiesGroup := r.Group("/properties")
	{
		propertiesGroup.GET("/approved", h.GetApprovedProperties)
		propertiesGroup.GET("/details/:propertyId", h.GetProperty)
		propertiesGroup.GET("/company-info", h.GetCompanyInfo)
	}
}

// GetApprovedProperties returns approved venue properties based on user role
func (h *Handler) GetApprovedProperties(c *gin.Context) {
	// Get user info from context
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}
	
	role, exists := c.Get("role")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User role not found"})
		return
	}
	
	var properties []models.PropertyResponse
	var err error
	
	// Convert role to string first
	var roleStr string
	switch v := role.(type) {
	case string:
		roleStr = v
	case float64:
		roleStr = fmt.Sprintf("%.0f", v)
	case int:
		roleStr = fmt.Sprintf("%d", v)
	default:
		c.JSON(http.StatusForbidden, gin.H{"error": "Invalid role format"})
		return
	}
	
	// Convert role string to number based on your enum
	// ADMIN = 0, SUPERADMIN = 1, CUSTOMER = 2, VENDOR = 3, VENDOR_STAFF = 4
	var roleNum int
	
	switch roleStr {
	case "1", "SUPERADMIN":
		roleNum = 1
	case "3", "VENDOR":
		roleNum = 3
	default:
		// Try to parse as number
		if roleStr == "1" {
			roleNum = 1
		} else if roleStr == "3" {
			roleNum = 3
		} else {
			c.JSON(http.StatusForbidden, gin.H{"error": "Access denied. Only vendors and superadmins can access properties"})
			return
		}
	}
	
	switch roleNum {
	case 1: // SUPERADMIN
		properties, err = h.service.GetAllApprovedVenueProperties()
	case 3: // VENDOR
		properties, err = h.service.GetVendorApprovedProperties(userID.(string))
	default:
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied. Only vendors and superadmins can access properties"})
		return
	}
	
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{
		"properties": properties,
		"total":      len(properties),
	})
}

// GetProperty returns a specific property by ID
func (h *Handler) GetProperty(c *gin.Context) {
	propertyID := c.Param("propertyId")
	
	property, err := h.service.GetPropertyByID(propertyID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Property not found"})
		return
	}
	
	c.JSON(http.StatusOK, property)
}

// GetCompanyInfo returns the user's company information
func (h *Handler) GetCompanyInfo(c *gin.Context) {
	// Get user info from context
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}
	
	role, exists := c.Get("role")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User role not found"})
		return
	}
	
	// Only vendors need company info for property selection
	roleStr := fmt.Sprintf("%v", role)
	if roleStr != "3" {
		c.JSON(http.StatusOK, gin.H{
			"isVenueCompany": false,
			"companyPurpose": "",
		})
		return
	}
	
	companyInfo, err := h.service.GetUserCompanyInfo(userID.(string))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"isVenueCompany": false,
			"companyPurpose": "",
		})
		return
	}
	
	c.JSON(http.StatusOK, companyInfo)
}