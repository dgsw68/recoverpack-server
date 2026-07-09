package auth

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"recoverpack-server/go-api/internal/firebase"
	"recoverpack-server/go-api/internal/models"
)

func RequireAuth(client *firebase.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if !strings.HasPrefix(header, "Bearer ") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Bearer token required"})
			return
		}
		claims, err := ParseToken(strings.TrimSpace(strings.TrimPrefix(header, "Bearer ")))
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired token"})
			return
		}
		user, err := client.GetUserByID(c.Request.Context(), claims.Subject)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "User not found"})
			return
		}
		c.Set(userContextKey, user)
		c.Next()
	}
}

func RequireProjectOwner(client *firebase.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		user, ok := CurrentUser(c)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
			return
		}
		project, err := client.GetProject(c.Request.Context(), c.Param("projectId"))
		if err != nil {
			c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"error": "Project not found"})
			return
		}
		if project.UserID != user.ID {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "Project access denied"})
			return
		}
		c.Next()
	}
}

func CurrentUser(c *gin.Context) (*models.User, bool) {
	value, exists := c.Get(userContextKey)
	if !exists {
		return nil, false
	}
	user, ok := value.(*models.User)
	return user, ok
}
