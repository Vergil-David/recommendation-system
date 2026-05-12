package main

import (
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	_ "recommendation-system/docs"

	"recommendation-system/internal/auth"
	"recommendation-system/internal/books"
	"recommendation-system/internal/search"
	"recommendation-system/internal/config"
	"recommendation-system/internal/database"
	"recommendation-system/internal/friends"
	"recommendation-system/internal/library"
	"recommendation-system/internal/middleware"
	"recommendation-system/internal/movies"
	"recommendation-system/internal/recommendations"
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

	// Rate limiter: 60 requests/second per IP, burst up to 120
	limiter := middleware.NewRateLimiter(60, 120, time.Second)
	r.Use(limiter.Middleware())

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
		usersGroup.PUT("/me", users.UpdateProfileHandler)
		usersGroup.GET("/search", users.SearchUsersHandler)
	}

	friendsGroup := r.Group("/friends")
	friendsGroup.Use(auth.RequireAuth())
	{
		friendsGroup.GET("", friends.GetFriendsHandler)
		friendsGroup.DELETE("/:id", friends.RemoveFriendHandler)
		friendsGroup.POST("/requests", friends.SendFriendRequestHandler)
		friendsGroup.GET("/requests/incoming", friends.ListIncomingFriendRequestsHandler)
		friendsGroup.GET("/requests/outgoing", friends.ListOutgoingFriendRequestsHandler)
		friendsGroup.POST("/requests/respond", friends.RespondToFriendRequestHandler)
	}

	libraryGroup := r.Group("/library")
	libraryGroup.Use(auth.RequireAuth())
	{
		libraryGroup.GET("/favorites", library.GetFavoriteMoviesHandler)
		libraryGroup.GET("/viewed", library.GetViewedMoviesHandler)
		libraryGroup.GET("/liked", library.GetLikedMoviesHandler)
	}

	r.GET("/movies", movies.GetMoviesHandler)
	r.GET("/movies/genres", movies.GetMovieGenresHandler)
	r.GET("/movies/:id", movies.GetMovieHandler)
	r.GET("/books", books.GetBooksHandler)
	r.GET("/books/genres", books.GetBookGenresHandler)
	r.GET("/books/:id", books.GetBookHandler)
	r.POST("/interactions", auth.RequireAuth(), recommendations.AddInteractionHandler)
	r.GET("/interactions/items", auth.RequireAuth(), recommendations.GetInteractionStatesHandler)
	r.GET("/recommendations", auth.RequireAuth(), recommendations.GetRecommendationsHandler)
	r.GET("/recommendations/friends", auth.RequireAuth(), recommendations.GetFriendRecommendationsHandler)
	r.GET("/search", search.SearchHandler)
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
