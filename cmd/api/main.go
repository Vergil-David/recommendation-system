package main

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"

	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	_ "recommendation-system/docs"

	"recommendation-system/internal/auth"
	"recommendation-system/internal/config"
	"recommendation-system/internal/database"
	"recommendation-system/internal/security"
)

// @title       Book Recommendation System API
// @version     1.0
// @description API для реєстрації, автентифікації та базових сервісів системи рекомендацій.
// @BasePath    /
// @schemes     http
func main() {
	// 1. Load config
	cfg := config.Load()

	// 2. Init database
	database.InitDB(cfg.Database.URL)
	defer database.DB.Close()

	// 2.1 Init JWT service
	auth.Init(security.NewJWTService(cfg.JWT.Secret, cfg.JWT.ExpiresIn))

	// 3. Gin setup
	r := gin.Default()

	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	authGroup := r.Group("/auth")
	{
		authGroup.POST("/register", auth.RegisterHandler)
		authGroup.POST("/login", auth.LoginHandler)
	}

	r.GET("/ping", PingHandler)

	log.Printf("🚀 Server running on :%s", cfg.Server.Port)
	r.Run(":" + cfg.Server.Port)
}

// PingHandler godoc
// @Summary     Перевірка доступності сервісу
// @Description Повертає "pong" якщо сервіс працює.
// @Tags        health
// @Produce     json
// @Success     200 {object} PingResponse
// @Router      /ping [get]
func PingHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"message": "pong"})
}

type PingResponse struct {
	Message string `json:"message" example:"pong"`
}
