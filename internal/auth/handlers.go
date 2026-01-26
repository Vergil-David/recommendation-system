package auth

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Structures for Swagger documentation
type RegisterRequest struct {
	Email    string `json:"email" binding:"required" example:"user@example.com"`
	Username string `json:"username" binding:"required" example:"johndoe"`
	Password string `json:"password" binding:"required" example:"secret123"`
}

type LoginRequest struct {
	Email    string `json:"email" binding:"required" example:"user@example.com"`
	Password string `json:"password" binding:"required" example:"secret123"`
}

// RegisterHandler godoc
// @Summary      Реєстрація нового користувача
// @Description  Створює новий акаунт користувача з email та паролем
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        request body RegisterRequest true "Дані для реєстрації"
// @Success      201  {object}  map[string]interface{}
// @Failure      400  {object}  map[string]string
// @Failure      500  {object}  map[string]string
// @Router       /auth/register [post]
func RegisterHandler(c *gin.Context) {
	var req RegisterRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	user, err := Register(c.Request.Context(), req.Email, req.Username, req.Password)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, user)
}

// LoginHandler godoc
// @Summary      Вхід користувача
// @Description  Перевіряє email/пароль та повертає JWT токен
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        request body LoginRequest true "Дані для входу"
// @Success      200  {object}  map[string]string
// @Failure      400  {object}  map[string]string
// @Failure      401  {object}  map[string]string
// @Router       /auth/login [post]
func LoginHandler(c *gin.Context) {
	var req LoginRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	user, err := Login(c.Request.Context(), req.Email, req.Password)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, user)
}
