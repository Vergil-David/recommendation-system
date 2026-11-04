package books

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"recommendation-system/internal/repository"
)



type BookItemResponse struct {
	ID          int64    `json:"id"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	ReleaseYear int      `json:"release_year"`
	ImageURL    string   `json:"image_url"`
	Genres      []string `json:"genres"`
}

type GetBooksResponse struct {
	Items []BookItemResponse `json:"items"`
	Total int                `json:"total"`
	Page  int                `json:"page"`
	Limit int                `json:"limit"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}

// GetBooksHandler godoc
// @Summary      Список книг
// @Description  Повертає список книг з пагінацією.
// @Tags         books
// @Produce      json
// @Param        page   query     int  false  "Номер сторінки (default 1)"
// @Param        limit  query     int  false  "Ліміт на сторінку (default 20, max 50)"
// @Success      200    {object}  GetBooksResponse
// @Failure      400    {object}  ErrorResponse
// @Failure      500    {object}  ErrorResponse
// @Router       /books [get]
func GetBooksHandler(c *gin.Context) {
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

	random := c.Query("sort") == "random"
	genre := c.Query("genre")
	result, err := ListBooks(c.Request.Context(), page, limit, random, genre)
	if err != nil {
		if errors.Is(err, ErrInvalidPagination) {
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "internal server error"})
		return
	}

	items := make([]BookItemResponse, 0, len(result.Items))
	for _, item := range result.Items {
		items = append(items, BookItemResponse{
			ID:          item.ID,
			Title:       item.Title,
			Description: item.Description,
			ReleaseYear: item.ReleaseYear,
			ImageURL:    item.ImageURL,
			Genres:      item.Genres,
		})
	}

	c.JSON(http.StatusOK, GetBooksResponse{
		Items: items,
		Total: result.Total,
		Page:  result.Page,
		Limit: result.Limit,
	})
}

// GetBookHandler godoc
// @Summary      Книга за ID
// @Description  Повертає одну книгу за її ID.
// @Tags         books
// @Produce      json
// @Param        id   path      int  true  "ID книги"
// @Success      200  {object}  BookItemResponse
// @Failure      400  {object}  ErrorResponse
// @Failure      404  {object}  ErrorResponse
// @Failure      500  {object}  ErrorResponse
// @Router       /books/{id} [get]
// GetBookGenresHandler godoc
// @Summary      Жанри книг
// @Description  Повертає всі жанри для книг, відсортовані за кількістю.
// @Tags         books
// @Produce      json
// @Success      200  {object}  map[string][]string
// @Router       /books/genres [get]
func GetBookGenresHandler(c *gin.Context) {
	genres, err := repository.GetItemGenres(c.Request.Context(), "book", 2)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "internal server error"})
		return
	}
	names := make([]string, 0, len(genres))
	for _, g := range genres {
		names = append(names, g.Name)
	}
	c.JSON(http.StatusOK, gin.H{"genres": names})
}

func GetBookHandler(c *gin.Context) {
	bookID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || bookID < 1 {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid path parameter: id"})
		return
	}

	item, err := GetBookByID(c.Request.Context(), bookID)
	if err != nil {
		switch {
		case errors.Is(err, ErrInvalidBookID):
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		case errors.Is(err, repository.ErrBookNotFound):
			c.JSON(http.StatusNotFound, ErrorResponse{Error: "book not found"})
		default:
			c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "internal server error"})
		}
		return
	}

	c.JSON(http.StatusOK, BookItemResponse{
		ID:          item.ID,
		Title:       item.Title,
		Description: item.Description,
		ReleaseYear: item.ReleaseYear,
		ImageURL:    item.ImageURL,
		Genres:      item.Genres,
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
