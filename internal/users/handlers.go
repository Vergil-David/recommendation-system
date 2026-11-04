package users

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"recommendation-system/internal/auth"
	"recommendation-system/internal/models"
	"recommendation-system/internal/repository"
)

type MeResponse struct {
	User models.User `json:"user"`
}

type SearchUsersResponse struct {
	Users []models.UserSearchResult `json:"users"`
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
// @Failure      500  {object}  ErrorResponse "Внутрішня помилка сервера"
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

// SearchUsersHandler godoc
// @Summary      Пошук користувачів
// @Description  Шукає користувачів по username, display_name або email і повертає relation_status відносно поточного користувача.
// @Tags         users
// @Produce      json
// @Security     BearerAuth
// @Param        q query string true "Пошуковий запит"
// @Param        limit query int false "Максимальна кількість результатів" default(20)
// @Success      200  {object}  SearchUsersResponse "Список знайдених користувачів"
// @Failure      400  {object}  ErrorResponse "Невірний або порожній запит"
// @Failure      401  {object}  ErrorResponse "Неавторизовано"
// @Failure      500  {object}  ErrorResponse "Внутрішня помилка сервера"
// @Router       /users/search [get]
func SearchUsersHandler(c *gin.Context) {
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

	limit := 0
	if rawLimit := c.Query("limit"); rawLimit != "" {
		parsedLimit, err := strconv.Atoi(rawLimit)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid limit"})
			return
		}
		limit = parsedLimit
	}

	foundUsers, err := SearchUsers(c.Request.Context(), userID, c.Query("q"), limit)
	if err != nil {
		switch {
		case errors.Is(err, ErrSearchQueryRequired):
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		}
		return
	}

	c.JSON(http.StatusOK, SearchUsersResponse{Users: foundUsers})
}

// UpdateProfileRequest is the JSON body for PUT /users/me.
type UpdateProfileRequest struct {
	DisplayName *string `json:"display_name"`
	AvatarURL   *string `json:"avatar_url"`
	Bio         *string `json:"bio"`
}

// UpdateProfileHandler godoc
// @Summary      Оновити профіль поточного користувача
// @Description  Оновлює display_name, avatar_url та/або bio. Поля, які не передані, залишаються без змін.
// @Tags         users
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request body UpdateProfileRequest true "Поля профілю для оновлення"
// @Success      200  {object}  MeResponse "Оновлений профіль"
// @Failure      400  {object}  ErrorResponse "Невірний запит"
// @Failure      401  {object}  ErrorResponse "Неавторизовано"
// @Failure      403  {object}  ErrorResponse "Користувач неактивний"
// @Failure      500  {object}  ErrorResponse "Внутрішня помилка сервера"
// @Router       /users/me [put]
func UpdateProfileHandler(c *gin.Context) {
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

	var req UpdateProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	updatedUser, err := UpdateProfile(c.Request.Context(), userID, repository.ProfileUpdate{
		DisplayName: req.DisplayName,
		AvatarURL:   req.AvatarURL,
		Bio:         req.Bio,
	})
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

	c.JSON(http.StatusOK, MeResponse{User: *updatedUser})
}
