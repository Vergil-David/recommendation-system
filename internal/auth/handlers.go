package auth

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"

	"recommendation-system/internal/models"
)

// Structures for Swagger documentation
type RegisterRequest struct {
	Email    string `json:"email" binding:"required" example:"user@example.com" format:"email"`
	Username string `json:"username" binding:"required" example:"johndoe"`
	Password string `json:"password" binding:"required" example:"secret123" minLength:"6"`
}

type LoginRequest struct {
	Email    string `json:"email" binding:"required" example:"user@example.com" format:"email"`
	Password string `json:"password" binding:"required" example:"secret123"`
}

type AuthResponse struct {
	Token string      `json:"token" example:"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."`
	User  models.User `json:"user"`
}

type ErrorResponse struct {
	Error string `json:"error" example:"invalid request"`
}

// RegisterHandler godoc
// @Summary      Реєстрація нового користувача
// @Description  Створює новий акаунт користувача з email, username та паролем.
// @Description  Повертає JWT токен та профіль користувача.
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        request body RegisterRequest true "Дані для реєстрації"
// @Success      201  {object}  AuthResponse "Користувача створено"
// @Failure      400  {object}  ErrorResponse "Невірні дані запиту або бізнес-правила"
// @Failure      409  {object}  ErrorResponse "Email або username вже зайняті"
// @Failure      500  {object}  ErrorResponse "Серверна помилка"
// @Router       /auth/register [post]
func RegisterHandler(c *gin.Context) {
	var req RegisterRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		log.Printf("❌ register: invalid request body, err=%v, ip=%s", err, c.ClientIP())
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	log.Printf("➡️ register: email=%s username=%s ip=%s ua=%s", req.Email, req.Username, c.ClientIP(), c.GetHeader("User-Agent"))
	user, token, err := Register(c.Request.Context(), req.Email, req.Username, req.Password)
	if err != nil {
		log.Printf("❌ register failed: email=%s username=%s err=%v", req.Email, req.Username, err)
		switch err {
		case ErrEmailTaken, ErrUsernameTaken:
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		case ErrJWTNotConfigured:
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		}
		return
	}

	c.JSON(http.StatusCreated, AuthResponse{
		Token: token,
		User:  *user,
	})
}

// LoginHandler godoc
// @Summary      Вхід користувача
// @Description  Перевіряє email/пароль та повертає JWT токен і профіль користувача.
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        request body LoginRequest true "Дані для входу"
// @Success      200  {object}  AuthResponse "Успішний вхід"
// @Failure      400  {object}  ErrorResponse "Невірні дані запиту"
// @Failure      401  {object}  ErrorResponse "Невірні облікові дані"
// @Failure      403  {object}  ErrorResponse "Користувач не активний"
// @Failure      500  {object}  ErrorResponse "Серверна помилка"
// @Router       /auth/login [post]
func LoginHandler(c *gin.Context) {
	var req LoginRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		log.Printf("❌ login: invalid request body, err=%v, ip=%s", err, c.ClientIP())
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	log.Printf("➡️ login: email=%s ip=%s ua=%s", req.Email, c.ClientIP(), c.GetHeader("User-Agent"))
	user, token, err := Login(c.Request.Context(), req.Email, req.Password)
	if err != nil {
		log.Printf("❌ login failed: email=%s err=%v", req.Email, err)
		switch err {
		case ErrInvalidCredentials:
			c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		case ErrUserInactive:
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		case ErrJWTNotConfigured:
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		}
		return
	}

	c.JSON(http.StatusOK, AuthResponse{
		Token: token,
		User:  *user,
	})
}
