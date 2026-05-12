package library

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"recommendation-system/internal/auth"
	"recommendation-system/internal/models"
)

type MovieItemResponse struct {
	ID          int64  `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	ReleaseYear int    `json:"release_year"`
	ImageURL    string `json:"image_url"`
}

type GetLibraryMoviesResponse struct {
	Items []MovieItemResponse `json:"items"`
}

type ErrorResponse struct {
	Error string `json:"error" example:"unauthorized"`
}

// GetFavoriteMoviesHandler godoc
// @Summary      Улюблені фільми користувача
// @Description  Повертає фільми поточного користувача зі станом favorite.
// @Tags         library
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  GetLibraryMoviesResponse
// @Failure      401  {object}  ErrorResponse "Неавторизовано"
// @Failure      500  {object}  ErrorResponse "Внутрішня помилка сервера"
// @Router       /library/favorites [get]
func GetFavoriteMoviesHandler(c *gin.Context) {
	userID, ok := authenticatedUserID(c)
	if !ok {
		return
	}

	items, err := GetFavoriteMovies(c.Request.Context(), userID, c.Query("type"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "internal server error"})
		return
	}

	c.JSON(http.StatusOK, GetLibraryMoviesResponse{Items: mapMovieItems(items)})
}

// GetViewedMoviesHandler godoc
// @Summary      Переглянуті фільми користувача
// @Description  Повертає фільми поточного користувача зі станом viewed.
// @Tags         library
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  GetLibraryMoviesResponse
// @Failure      401  {object}  ErrorResponse "Неавторизовано"
// @Failure      500  {object}  ErrorResponse "Внутрішня помилка сервера"
// @Router       /library/viewed [get]
func GetViewedMoviesHandler(c *gin.Context) {
	userID, ok := authenticatedUserID(c)
	if !ok {
		return
	}

	items, err := GetViewedMovies(c.Request.Context(), userID, c.Query("type"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "internal server error"})
		return
	}

	c.JSON(http.StatusOK, GetLibraryMoviesResponse{Items: mapMovieItems(items)})
}

// GetLikedMoviesHandler godoc
// @Summary      Лайкнуті фільми користувача
// @Description  Повертає фільми поточного користувача зі станом liked.
// @Tags         library
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  GetLibraryMoviesResponse
// @Failure      401  {object}  ErrorResponse "Неавторизовано"
// @Failure      500  {object}  ErrorResponse "Внутрішня помилка сервера"
// @Router       /library/liked [get]
func GetLikedMoviesHandler(c *gin.Context) {
	userID, ok := authenticatedUserID(c)
	if !ok {
		return
	}

	items, err := GetLikedMovies(c.Request.Context(), userID, c.Query("type"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "internal server error"})
		return
	}

	c.JSON(http.StatusOK, GetLibraryMoviesResponse{Items: mapMovieItems(items)})
}

func authenticatedUserID(c *gin.Context) (uuid.UUID, bool) {
	userIDRaw, exists := c.Get(auth.CtxUserIDKey)
	if !exists {
		c.JSON(http.StatusUnauthorized, ErrorResponse{Error: "unauthorized"})
		return uuid.Nil, false
	}

	userID, ok := userIDRaw.(uuid.UUID)
	if !ok {
		c.JSON(http.StatusUnauthorized, ErrorResponse{Error: "invalid token subject"})
		return uuid.Nil, false
	}

	return userID, true
}

func mapMovieItems(items []models.Item) []MovieItemResponse {
	responseItems := make([]MovieItemResponse, 0, len(items))
	for _, item := range items {
		responseItems = append(responseItems, MovieItemResponse{
			ID:          item.ID,
			Title:       item.Title,
			Description: item.Description,
			ReleaseYear: item.ReleaseYear,
			ImageURL:    item.ImageURL,
		})
	}
	return responseItems
}
