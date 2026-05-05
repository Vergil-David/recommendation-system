package movies

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"recommendation-system/internal/repository"
)

type MovieItemResponse struct {
	ID          int64    `json:"id"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	ReleaseYear int      `json:"release_year"`
	ImageURL    string   `json:"image_url"`
	Genres      []string `json:"genres"`
}

type GetMoviesResponse struct {
	Items []MovieItemResponse `json:"items"`
	Total int                 `json:"total"`
	Page  int                 `json:"page"`
	Limit int                 `json:"limit"`
}

type GetMovieResponse struct {
	ID          int64          `json:"id"`
	Title       string         `json:"title"`
	Description string         `json:"description"`
	ReleaseYear int            `json:"release_year"`
	ImageURL    string         `json:"image_url"`
	Metadata    map[string]any `json:"metadata,omitempty"`
}

type ErrorResponse struct {
	Error string `json:"error" example:"invalid query parameters"`
}

// GetMoviesHandler godoc
// @Summary      Список фільмів
// @Description  Повертає список фільмів з пагінацією.
// @Tags         movies
// @Produce      json
// @Param        page   query     int  false  "Номер сторінки (default 1)"
// @Param        limit  query     int  false  "Ліміт на сторінку (default 20, max 50)"
// @Success      200    {object}  GetMoviesResponse
// @Failure      400    {object}  ErrorResponse "Некоректні query-параметри"
// @Failure      500    {object}  ErrorResponse "Помилка бази даних"
// @Router       /movies [get]
func GetMoviesHandler(c *gin.Context) {
	page, err := parseOptionalPositiveInt(c.Query("page"))
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid query parameter: page"})
		return
	}

	limit, err := parseOptionalPositiveInt(c.Query("limit"))
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid query parameter: limit"})
		return
	}

	result, err := ListMovies(c.Request.Context(), page, limit)
	if err != nil {
		if errors.Is(err, ErrInvalidPagination) {
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "internal server error"})
		return
	}

	items := make([]MovieItemResponse, 0, len(result.Items))
	for _, item := range result.Items {
		items = append(items, MovieItemResponse{
			ID:          item.ID,
			Title:       item.Title,
			Description: item.Description,
			ReleaseYear: item.ReleaseYear,
			ImageURL:    item.ImageURL,
			Genres:      item.Genres,
		})
	}

	c.JSON(http.StatusOK, GetMoviesResponse{
		Items: items,
		Total: result.Total,
		Page:  result.Page,
		Limit: result.Limit,
	})
}

// GetMovieHandler godoc
// @Summary      Фільм за ID
// @Description  Повертає один фільм за його ID.
// @Tags         movies
// @Produce      json
// @Param        id   path      int  true  "ID фільму"
// @Success      200  {object}  GetMovieResponse
// @Failure      400  {object}  ErrorResponse "Некоректний ID"
// @Failure      404  {object}  ErrorResponse "Фільм не знайдено"
// @Failure      500  {object}  ErrorResponse "Помилка бази даних"
// @Router       /movies/{id} [get]
func GetMovieHandler(c *gin.Context) {
	movieID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || movieID < 1 {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid path parameter: id"})
		return
	}

	result, err := GetMovieByID(c.Request.Context(), movieID)
	if err != nil {
		switch {
		case errors.Is(err, ErrInvalidMovieID):
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		case errors.Is(err, repository.ErrMovieNotFound):
			c.JSON(http.StatusNotFound, ErrorResponse{Error: err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "internal server error"})
		}
		return
	}

	c.JSON(http.StatusOK, GetMovieResponse{
		ID:          result.ID,
		Title:       result.Title,
		Description: result.Description,
		ReleaseYear: result.ReleaseYear,
		ImageURL:    result.ImageURL,
		Metadata:    result.Metadata,
	})
}

func parseOptionalPositiveInt(raw string) (int, error) {
	if raw == "" {
		return 0, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return 0, err
	}
	if value < 0 {
		return 0, ErrInvalidPagination
	}
	return value, nil
}
