package auth

import (
	"net/http"
	"net/mail"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"recoverpack-server/go-api/internal/firebase"
	"recoverpack-server/go-api/internal/models"
)

const userContextKey = "authUser"

type credentialsRequest struct {
	Email    string `json:"email" binding:"required"`
	Password string `json:"password" binding:"required"`
	Name     string `json:"name"`
}

func RegisterHandler(client *firebase.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		var request credentialsRequest
		if err := c.ShouldBindJSON(&request); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Email and password are required"})
			return
		}
		email := strings.ToLower(strings.TrimSpace(request.Email))
		if parsed, err := mail.ParseAddress(email); err != nil || parsed.Address != email {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid email address"})
			return
		}
		passwordHash, err := HashPassword(request.Password)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		user := &models.User{
			ID: uuid.NewString(), Email: email,
			Name: strings.TrimSpace(request.Name), PasswordHash: passwordHash,
		}
		if err := client.CreateUser(c.Request.Context(), user); err != nil {
			if strings.Contains(err.Error(), "already registered") {
				c.JSON(http.StatusConflict, gin.H{"error": "Email already registered"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create account"})
			return
		}
		respondWithToken(c, user, http.StatusCreated)
	}
}

func LoginHandler(client *firebase.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		var request credentialsRequest
		if err := c.ShouldBindJSON(&request); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Email and password are required"})
			return
		}
		user, err := client.GetUserByEmail(c.Request.Context(), request.Email)
		if err != nil || !VerifyPassword(user.PasswordHash, request.Password) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid email or password"})
			return
		}
		respondWithToken(c, user, http.StatusOK)
	}
}

func MeHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		user, ok := CurrentUser(c)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
			return
		}
		c.JSON(http.StatusOK, user)
	}
}

func respondWithToken(c *gin.Context, user *models.User, status int) {
	token, err := CreateToken(user.ID, user.Email)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create access token"})
		return
	}
	c.JSON(status, gin.H{
		"accessToken": token,
		"tokenType":   "Bearer",
		"expiresIn":   int(tokenLifetime.Seconds()),
		"user":        user,
	})
}
