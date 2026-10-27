package contact

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"backend/internal/models"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// CreateContact handles POST /api/contact
func (h *Handler) CreateContact(c *gin.Context) {
	var req models.ContactRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	// Basic validation
	if req.FullName == "" || req.Email == "" || req.PhoneNumber == "" || req.Message == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "All fields are required"})
		return
	}

	// Get client IP
	clientIP := c.ClientIP()

	contact, err := h.service.CreateContact(&req, clientIP)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to submit contact form"})
		return
	}

	response := models.ContactResponse{
		ID:      contact.ID,
		Message: "Contact form submitted successfully",
	}

	c.JSON(http.StatusCreated, response)
}

// GetContacts handles GET /api/contacts (admin only)
func (h *Handler) GetContacts(c *gin.Context) {
	// Parse pagination parameters
	limitStr := c.DefaultQuery("limit", "10")
	offsetStr := c.DefaultQuery("offset", "0")

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = 10
	}

	offset, err := strconv.Atoi(offsetStr)
	if err != nil || offset < 0 {
		offset = 0
	}

	contacts, total, err := h.service.GetContacts(limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get contacts"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"contacts": contacts,
		"total":    total,
		"limit":    limit,
		"offset":   offset,
	})
}

// GetContact handles GET /api/contacts/:id (admin only)
func (h *Handler) GetContact(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Contact ID is required"})
		return
	}

	contact, err := h.service.GetContactByID(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Contact not found"})
		return
	}

	c.JSON(http.StatusOK, contact)
}

// UpdateContactStatus handles PATCH /api/contacts/:id/status (admin only)
func (h *Handler) UpdateContactStatus(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Contact ID is required"})
		return
	}

	var req struct {
		Status int `json:"status"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	if err := h.service.UpdateContactStatus(id, req.Status); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update contact status"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Contact status updated successfully"})
}