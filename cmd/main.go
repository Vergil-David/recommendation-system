package main

import (
	"context"
	"log"
	"net/http"

	"recommendation-system/internal/repository"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"

	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	// 👇 ТУТ МАЄ БУТИ НАЗВА ТВОГО МОДУЛЯ (з файлу go.mod)
	// Якщо в go.mod написано module recommendation-system, то лишай як є.
	// Якщо там module myapp, то зміни на "myapp/internal/database"
	_ "recommendation-system/docs"
	"recommendation-system/internal/database"
)

// @title           Book Recommendation System API
// @version         1.0
// @description     API сервер для курсової роботи.
// @host            localhost:8080
// @BasePath        /
func main() {
	// 1. Завантажуємо .env (щоб отримати пароль до бази)
	if err := godotenv.Load(); err != nil {
		log.Println("⚠️ .env файл не знайдено, шукаю змінні середовища")
	}

	// 2. Підключаємо Базу Даних (викликаємо функцію з іншого файлу)
	database.InitDB()
	// Закриваємо з'єднання, коли сервер зупиниться
	defer database.DB.Close(context.Background())

	// 3. Запускаємо Веб-сервер (Gin + Swagger)
	r := gin.Default()

	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	r.GET("/ping", PingHandler)
	r.GET("/recommend/vector", VectorHandler)

	r.Run(":8080")
}

// ... (твої хендлери залишаються без змін) ...
func PingHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"message": "pong"})
}

func VectorHandler(c *gin.Context) {
	// Імітуємо вектор інтересів користувача.
	// [1.0, 0.5, 0.0] -> Це означає: Дуже любить першу категорію (Історія),
	// трохи любить другу, і зовсім не любить третю.
	userInterests := []float32{1.0, 0.5, 0.0}

	// Викликаємо функцію пошуку
	items, err := repository.FindSimilar(userInterests)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":     "Рекомендації на основі вектора [1.0, 0.5, 0.0]",
		"recommended": items,
	})
}
