package handlers

import (
	"log"
	"net/http"
	"os"
	"regexp"
	"time"

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

// ForgotPasswordRequest is the request body for forgot password
type ForgotPasswordRequest struct {
	Email string `json:"email" binding:"required"`
}

// ResetPasswordRequest is the request body for reset password
type ResetPasswordRequest struct {
	Token       string `json:"token" binding:"required"`
	NewPassword string `json:"newPassword" binding:"required"`
}

// ForgotPassword initiates password reset - sends email with reset link (or logs in dev)
func (h *AuthHandler) ForgotPassword(c *gin.Context) {
	var req ForgotPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Email is required"})
		return
	}

	email := auth.NormalizeEmail(req.Email)
	if !emailRegex.MatchString(email) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid email format"})
		return
	}

	user, err := h.userRepo.GetByEmail(c.Request.Context(), email)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "If an account exists, a reset link has been sent"})
		return
	}
	// Always return success to prevent email enumeration
	if user == nil {
		c.JSON(http.StatusOK, gin.H{"message": "If an account exists, a reset link has been sent"})
		return
	}

	plainToken, err := repository.GenerateSecureToken()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate reset token"})
		return
	}

	tokenHash := auth.HashToken(plainToken)
	expiresAt := time.Now().Add(1 * time.Hour)
	err = h.userRepo.CreatePasswordResetToken(c.Request.Context(), user.ID, tokenHash, expiresAt)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create reset token"})
		return
	}

	frontendURL := os.Getenv("FRONTEND_URL")
	if frontendURL == "" {
		frontendURL = "http://localhost:5173"
	}
	resetLink := frontendURL + "/reset-password?token=" + plainToken

	// In production, send email. For dev, log the link.
	if os.Getenv("SMTP_HOST") != "" {
		// TODO: Integrate with email service (SMTP, SendGrid, etc.)
		log.Printf("Password reset for %s: %s", email, resetLink)
	} else {
		log.Printf("Password reset link for %s (dev mode): %s", email, resetLink)
	}

	c.JSON(http.StatusOK, gin.H{"message": "If an account exists, a reset link has been sent"})
}

// ResetPassword completes password reset with token
func (h *AuthHandler) ResetPassword(c *gin.Context) {
	var req ResetPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Token and new password are required"})
		return
	}

	if err := auth.ValidatePassword(req.NewPassword); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	tokenHash := auth.HashToken(req.Token)
	userID, err := h.userRepo.GetUserIDByResetToken(c.Request.Context(), tokenHash)
	if err != nil || userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid or expired reset token"})
		return
	}

	passwordHash, err := auth.HashPassword(req.NewPassword)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to reset password"})
		return
	}

	if err := h.userRepo.UpdatePassword(c.Request.Context(), userID, passwordHash); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to reset password"})
		return
	}

	_ = h.userRepo.DeletePasswordResetToken(c.Request.Context(), tokenHash)

	c.JSON(http.StatusOK, gin.H{"message": "Password has been reset successfully"})
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
