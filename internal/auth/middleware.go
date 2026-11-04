package auth

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const (
	CtxUserIDKey = "userID"
	CtxRoleKey   = "role"
)

func RequireAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		if jwtService == nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "auth service unavailable"})
			c.Abort()
			return
		}

		authHeader := strings.TrimSpace(c.GetHeader("Authorization"))
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "missing authorization header"})
			c.Abort()
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || strings.TrimSpace(parts[1]) == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid authorization header format"})
			c.Abort()
			return
		}

		claims, err := jwtService.Parse(strings.TrimSpace(parts[1]))
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired token"})
			c.Abort()
			return
		}

		userID := claims.UserID
		if userID == uuid.Nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid token subject"})
			c.Abort()
			return
		}

		c.Set(CtxUserIDKey, userID)
		c.Set(CtxRoleKey, claims.Role)
		c.Next()
	}
}
