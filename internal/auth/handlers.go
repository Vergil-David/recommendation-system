package auth

import (
	"context"
	"log"
	"net/http"
	"recommendation-system/internal/database"

	"github.com/gin-gonic/gin"
)

// Structures for Swagger documentation
type RegisterRequest struct {
	Email    string `json:"email" binding:"required" example:"user@example.com"`
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
		c.JSON(http.StatusBadRequest, gin.H{"error": "Неправильні дані"})
		return
	}

	hashedPwd, err := HashPassword(req.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Помилка хешування"})
		return
	}

	query := `INSERT INTO users (email, password_hash) VALUES ($1, $2) RETURNING id`
	var newID int
	err = database.DB.QueryRow(context.Background(), query, req.Email, hashedPwd).Scan(&newID)

	if err != nil {
		log.Println("DB Error:", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Не вдалося створити користувача (можливо email зайнятий)"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": "Успішна реєстрація", "user_id": newID})
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
		c.JSON(http.StatusBadRequest, gin.H{"error": "Неправильні дані"})
		return
	}

	var id int
	var storedHash string

	query := `SELECT id, password_hash FROM users WHERE email = $1`
	err := database.DB.QueryRow(context.Background(), query, req.Email).Scan(&id, &storedHash)

	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Користувача не знайдено"})
		return
	}

	match := CheckPasswordHash(req.Password, storedHash)
	if !match {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Невірний пароль"})
		return
	}

	token, _ := GenerateJWT(string(rune(id)))

	c.JSON(http.StatusOK, gin.H{"token": token})
}
