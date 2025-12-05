package main

import (
	"net/http"

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	// Цей імпорт з'явиться після генерації, але додамо його заздалегідь
	// (заміни recommendation-system на назву своєї папки в go.mod, якщо інша)
	_ "recommendation-system/docs"
)

// @title           Book Recommendation System API
// @version         1.0
// @description     API сервер для курсової роботи з рекомендаційною системою.
// @host            localhost:8080
// @BasePath        /
func main() {
	r := gin.Default()

	// Маршрут для документації Swagger
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	// Твої ендпоінти
	r.GET("/ping", PingHandler)
	r.GET("/recommend/vector", VectorHandler)

	r.Run(":8080")
}

// PingHandler перевірка статусу сервера
// @Summary      Перевірка здоров'я сервера
// @Description  Повертає pong, якщо сервер живий
// @Tags         system
// @Accept       json
// @Produce      json
// @Success      200  {object}  map[string]string
// @Router       /ping [get]
func PingHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"message": "pong"})
}

// VectorHandler отримання рекомендацій (векторний метод)
// @Summary      Векторні рекомендації
// @Description  Повертає список книг на основі схожості описів
// @Tags         recommendations
// @Accept       json
// @Produce      json
// @Success      200  {object}  map[string]interface{}
// @Router       /recommend/vector [get]
func VectorHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"type":  "vector",
		"books": []string{"Harry Potter", "Dune"},
	})
}
