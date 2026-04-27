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
	"recommendation-system/internal/friends"
	"recommendation-system/internal/middleware"
	"recommendation-system/internal/movies"
	"recommendation-system/internal/security"
	"recommendation-system/internal/users"
)

// @title       Book Recommendation System API
// @version     1.0
// @description API для реєстрації, автентифікації та базових сервісів системи рекомендацій.
// @BasePath    /
// @schemes     http
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
func main() {
	// 1. Load config
	cfg := config.Load()
	log.Printf("🔎 Migrations DB URL: %s", database.MaskDatabaseURL(cfg.Database.MigrationsURL))

	// 2. Init database
	database.InitDB(cfg.Database.URL)
	defer database.DB.Close()

	// 2.1 Init JWT service
	auth.Init(security.NewJWTService(cfg.JWT.Secret, cfg.JWT.ExpiresIn))

	// 3. Gin setup
	r := gin.Default()
	if err := r.SetTrustedProxies(nil); err != nil {
		log.Fatalf("failed to set trusted proxies: %v", err)
	}

	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	authGroup := r.Group("/auth")
	{
		authGroup.POST("/register", auth.RegisterHandler)
		authGroup.POST("/login", auth.LoginHandler)
	}

	usersGroup := r.Group("/users")
	usersGroup.Use(auth.RequireAuth())
	{
		usersGroup.GET("/me", users.GetMeHandler)
		usersGroup.GET("/search", users.SearchUsersHandler)
	}

	friendsGroup := r.Group("/friends")
	friendsGroup.Use(auth.RequireAuth())
	{
		friendsGroup.GET("", friends.GetFriendsHandler)
		friendsGroup.POST("/requests", friends.SendFriendRequestHandler)
		friendsGroup.GET("/requests/incoming", friends.ListIncomingFriendRequestsHandler)
		friendsGroup.GET("/requests/outgoing", friends.ListOutgoingFriendRequestsHandler)
		friendsGroup.POST("/requests/respond", friends.RespondToFriendRequestHandler)
	}

	r.GET("/movies", movies.GetMoviesHandler)
	r.GET("/ping", PingHandler)

	handler := middleware.SetupCORS(r, cfg.Server.FrontendURL)
	server := &http.Server{
		Addr:    ":" + cfg.Server.Port,
		Handler: handler,
	}

	log.Printf("🚀 Server running on :%s", cfg.Server.Port)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("server failed: %v", err)
	}
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
