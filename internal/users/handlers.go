package users

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"recommendation-system/internal/auth"
	"recommendation-system/internal/models"
	"recommendation-system/internal/repository"
)

type MeResponse struct {
	User models.User `json:"user"`
}

type ErrorResponse struct {
	Error string `json:"error" example:"unauthorized"`
}

// GetMeHandler godoc
// @Summary      Поточний профіль користувача
// @Description  Повертає профіль поточного авторизованого користувача.
// @Tags         users
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  MeResponse "Профіль користувача"
// @Failure      401  {object}  ErrorResponse "Токен невалідний або користувача не знайдено"
// @Failure      403  {object}  ErrorResponse "Користувач неактивний"
// @Router       /users/me [get]
func GetMeHandler(c *gin.Context) {
	userIDRaw, exists := c.Get(auth.CtxUserIDKey)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	userID, ok := userIDRaw.(uuid.UUID)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid token subject"})
		return
	}

	user, err := GetMe(c.Request.Context(), userID)
	if err != nil {
		switch {
		case errors.Is(err, repository.ErrUserNotFound):
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid token subject"})
		case errors.Is(err, ErrUserInactive):
			c.JSON(http.StatusForbidden, gin.H{"error": ErrUserInactive.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		}
		return
	}

	c.JSON(http.StatusOK, MeResponse{User: *user})
}
