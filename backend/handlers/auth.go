package handlers

import (
	"net/http"
	"regexp"

	"liftoff/backend/auth"
	"liftoff/backend/repository"

	"github.com/gin-gonic/gin"
)

var emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)

// AuthHandler handles authentication HTTP requests
type AuthHandler struct {
	userRepo *repository.UserRepository
}

// NewAuthHandler creates a new auth handler
func NewAuthHandler(userRepo *repository.UserRepository) *AuthHandler {
	return &AuthHandler{userRepo: userRepo}
}

// LoginRequest is the request body for login
type LoginRequest struct {
	Email      string `json:"email" binding:"required"`
	Password   string `json:"password" binding:"required"`
	RememberMe bool   `json:"rememberMe"`
}

// RegisterRequest is the request body for registration
type RegisterRequest struct {
	Email    string `json:"email" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// AuthResponse is the response for auth endpoints
type AuthResponse struct {
	Token     string `json:"token"`
	ExpiresAt string `json:"expiresAt"`
	User      struct {
		ID    string `json:"id"`
		Email string `json:"email"`
	} `json:"user"`
}

// Login handles user login
func (h *AuthHandler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Email and password are required"})
		return
	}

	email := auth.NormalizeEmail(req.Email)
	if !emailRegex.MatchString(email) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid email format"})
		return
	}

	user, err := h.userRepo.GetByEmail(c.Request.Context(), email)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Login failed"})
		return
	}

	if user == nil || !auth.CheckPassword(req.Password, user.PasswordHash) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid email or password"})
		return
	}

	tokenString, expiresAt, err := auth.GenerateToken(user.ID, user.Email, req.RememberMe)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
		return
	}

	c.JSON(http.StatusOK, AuthResponse{
		Token:     tokenString,
		ExpiresAt: expiresAt.Format("2006-01-02T15:04:05Z07:00"),
		User: struct {
			ID    string `json:"id"`
			Email string `json:"email"`
		}{
			ID:    user.ID,
			Email: user.Email,
		},
	})
}

// Register handles user registration
func (h *AuthHandler) Register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Email and password are required"})
		return
	}

	email := auth.NormalizeEmail(req.Email)
	if !emailRegex.MatchString(email) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid email format"})
		return
	}

	if err := auth.ValidatePassword(req.Password); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Check if user already exists
	existing, err := h.userRepo.GetByEmail(c.Request.Context(), email)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Registration failed"})
		return
	}
	if existing != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "An account with this email already exists"})
		return
	}

	passwordHash, err := auth.HashPassword(req.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Registration failed"})
		return
	}

	user, err := h.userRepo.CreateUser(c.Request.Context(), email, passwordHash)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Registration failed"})
		return
	}

	// Generate short-lived token for new registration (no remember me on signup)
	tokenString, expiresAt, err := auth.GenerateToken(user.ID, user.Email, false)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Registration succeeded but failed to generate token"})
		return
	}

	c.JSON(http.StatusCreated, AuthResponse{
		Token:     tokenString,
		ExpiresAt: expiresAt.Format("2006-01-02T15:04:05Z07:00"),
		User: struct {
			ID    string `json:"id"`
			Email string `json:"email"`
		}{
			ID:    user.ID,
			Email: user.Email,
		},
	})
}

// Me returns the current authenticated user (requires AuthMiddleware)
func (h *AuthHandler) Me(c *gin.Context) {
	userID := auth.GetUserID(c)
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Not authenticated"})
		return
	}

	user, err := h.userRepo.GetByID(c.Request.Context(), userID)
	if err != nil || user == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"user": gin.H{
			"id":    user.ID,
			"email": user.Email,
		},
	})
}
