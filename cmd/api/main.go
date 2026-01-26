package main

import (
	"context"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"

	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	_ "recommendation-system/docs"

	"recommendation-system/internal/auth"
	"recommendation-system/internal/config"
	"recommendation-system/internal/database"
)

// @title Book Recommendation System API
// @version 1.0
func main() {
	// 1. Load config
	cfg := config.Load()

	// 2. Init database
	database.InitDB(cfg.DatabaseURL)
	defer database.DB.Close(context.Background())

	// 3. Gin setup
	r := gin.Default()

	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	authGroup := r.Group("/auth")
	{
		authGroup.POST("/register", auth.RegisterHandler)
		authGroup.POST("/login", auth.LoginHandler)
	}

	r.GET("/ping", PingHandler)

	log.Printf("🚀 Server running on :%s", cfg.ServerPort)
	r.Run(":" + cfg.ServerPort)
}

func PingHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"message": "pong"})
}
