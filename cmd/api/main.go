package main

import (
	"context"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"

	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	_ "recommendation-system/docs"            // Swagger docs
	"recommendation-system/internal/auth"     // Твої хендлери
	"recommendation-system/internal/database" // Твоя БД
	"recommendation-system/internal/repository"
)

// @title           Book Recommendation System API
// @version         1.0
func main() {
	// 1. Завантажуємо .env
	if err := godotenv.Load(); err != nil {
		log.Println("⚠️ .env файл не знайдено")
	}

	// 2. Ініціалізація БД
	// ВИПРАВЛЕНО: Ми просто викликаємо функцію, не присвоюючи результат
	database.InitDB()
	// Закриваємо з'єднання при виході
	defer database.DB.Close(context.Background())

	// 3. Налаштування Gin
	r := gin.Default()

	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	// 4. Налаштування Auth (Простий варіант)
	authGroup := r.Group("/auth")
	{
		// Використовуємо функції напряму
		authGroup.POST("/register", auth.RegisterHandler)
		authGroup.POST("/login", auth.LoginHandler)
	}

	// 5. Інші ендпоінти
	r.GET("/ping", PingHandler)
	r.GET("/recommend/vector", VectorHandler)

	log.Println("🚀 Server running on :8080")
	r.Run(":8080")
}

func PingHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"message": "pong"})
}

func VectorHandler(c *gin.Context) {
	userInterests := []float32{1.0, 0.5, 0.0}
	items, err := repository.FindSimilar(userInterests)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, items)
}
