package friends

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"recommendation-system/internal/auth"
	"recommendation-system/internal/models"
	"recommendation-system/internal/repository"
)

type SendFriendRequestRequest struct {
	ToUserID string `json:"to_user_id" binding:"required" example:"550e8400-e29b-41d4-a716-446655440000"`
}

type SendFriendRequestResponse struct {
	Status string `json:"status" example:"pending"`
}

type RespondFriendRequestRequest struct {
	FromUserID string `json:"from_user_id" binding:"required" example:"550e8400-e29b-41d4-a716-446655440000"`
	Action     string `json:"action" binding:"required" example:"accept"`
}

type FriendRequestStatusResponse struct {
	Status string `json:"status" example:"accepted"`
}

type FriendsListResponse struct {
	Friends []models.User `json:"friends"`
}

type ErrorResponse struct {
	Error string `json:"error" example:"invalid request"`
}

// SendFriendRequestHandler godoc
// @Summary      Надіслати запит у друзі
// @Description  Створює новий запит у друзі зі статусом pending.
// @Tags         friends
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request body SendFriendRequestRequest true "Цільовий користувач"
// @Success      201  {object}  SendFriendRequestResponse "Запит надіслано"
// @Failure      400  {object}  ErrorResponse "Невірні дані або спроба додати себе"
// @Failure      401  {object}  ErrorResponse "Неавторизовано"
// @Failure      404  {object}  ErrorResponse "Користувача не знайдено"
// @Failure      409  {object}  ErrorResponse "Запит уже існує"
// @Failure      500  {object}  ErrorResponse "Внутрішня помилка сервера"
// @Router       /friends/requests [post]
func SendFriendRequestHandler(c *gin.Context) {
	userIDRaw, exists := c.Get(auth.CtxUserIDKey)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	fromUserID, ok := userIDRaw.(uuid.UUID)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid token subject"})
		return
	}

	var req SendFriendRequestRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	toUserID, err := uuid.Parse(req.ToUserID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid to_user_id"})
		return
	}

	if err := SendFriendRequest(c.Request.Context(), fromUserID, toUserID); err != nil {
		switch {
		case errors.Is(err, ErrCannotFriendSelf):
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		case errors.Is(err, ErrTargetUserNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		case errors.Is(err, repository.ErrFriendRequestAlreadyExists):
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		}
		return
	}

	c.JSON(http.StatusCreated, SendFriendRequestResponse{Status: "pending"})
}

// RespondToFriendRequestHandler godoc
// @Summary      Відповісти на запит у друзі
// @Description  Дозволяє отримувачу запиту прийняти або відхилити pending-запит.
// @Tags         friends
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request body RespondFriendRequestRequest true "Користувач-відправник і дія"
// @Success      200  {object}  FriendRequestStatusResponse "Новий статус запиту"
// @Failure      400  {object}  ErrorResponse "Невірні дані або дія"
// @Failure      401  {object}  ErrorResponse "Неавторизовано"
// @Failure      404  {object}  ErrorResponse "Запит не знайдено"
// @Failure      409  {object}  ErrorResponse "Запит не в pending статусі"
// @Failure      500  {object}  ErrorResponse "Внутрішня помилка сервера"
// @Router       /friends/requests/respond [post]
func RespondToFriendRequestHandler(c *gin.Context) {
	userIDRaw, exists := c.Get(auth.CtxUserIDKey)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	receiverID, ok := userIDRaw.(uuid.UUID)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid token subject"})
		return
	}

	var req RespondFriendRequestRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	fromUserID, err := uuid.Parse(req.FromUserID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid from_user_id"})
		return
	}

	status, err := RespondToFriendRequest(c.Request.Context(), receiverID, fromUserID, req.Action)
	if err != nil {
		switch {
		case errors.Is(err, ErrInvalidAction):
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		case errors.Is(err, ErrFriendRequestNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		case errors.Is(err, ErrFriendRequestNotPending):
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		}
		return
	}

	c.JSON(http.StatusOK, FriendRequestStatusResponse{Status: status})
}

// GetFriendsHandler godoc
// @Summary      Список друзів користувача
// @Description  Повертає список користувачів зі статусом accepted для поточного користувача.
// @Tags         friends
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  FriendsListResponse "Список друзів"
// @Failure      401  {object}  ErrorResponse "Неавторизовано"
// @Failure      500  {object}  ErrorResponse "Внутрішня помилка сервера"
// @Router       /friends [get]
func GetFriendsHandler(c *gin.Context) {
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

	friends, err := GetFriends(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}

	c.JSON(http.StatusOK, FriendsListResponse{Friends: friends})
}
