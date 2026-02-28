package handlers

import (
	"net/http"

	"liftoff/backend/models"
	"liftoff/backend/repository"

	"github.com/gin-gonic/gin"
)

// AdminHandler handles admin-only endpoints
type AdminHandler struct {
	userRepo  *repository.UserRepository
	adminRepo *repository.AdminRepository
}

// NewAdminHandler creates a new admin handler
func NewAdminHandler(userRepo *repository.UserRepository, adminRepo *repository.AdminRepository) *AdminHandler {
	return &AdminHandler{userRepo: userRepo, adminRepo: adminRepo}
}

// ListUsers returns all registered users (admin only)
func (h *AdminHandler) ListUsers(c *gin.Context) {
	users, err := h.userRepo.ListAllUsers(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list users"})
		return
	}
	if users == nil {
		users = []*models.User{}
	}
	c.JSON(http.StatusOK, gin.H{"users": users})
}

// GetStats returns aggregate statistics (admin only)
func (h *AdminHandler) GetStats(c *gin.Context) {
	stats, err := h.adminRepo.GetStats(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get stats"})
		return
	}
	c.JSON(http.StatusOK, stats)
}
