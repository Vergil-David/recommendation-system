package search

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"recommendation-system/internal/repository"
)

type SearchItemResponse struct {
	ID          int64    `json:"id"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	ReleaseYear int      `json:"release_year"`
	ImageURL    string   `json:"image_url"`
	Type        string   `json:"type"`
	Genres      []string `json:"genres"`
	Score       float64  `json:"score"`
}

type SearchResponse struct {
	Items []SearchItemResponse `json:"items"`
	Query string               `json:"query"`
	Total int                  `json:"total"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}

// SearchHandler godoc
// @Summary      Пошук фільмів і книг
// @Description  Нечіткий пошук по назві, опису та жанру.
// @Tags         search
// @Produce      json
// @Param        q      query  string  true   "Пошуковий запит"
// @Param        type   query  string  false  "Тип: movie, book або порожньо (обидва)"
// @Param        limit  query  int     false  "Ліміт результатів (default 20, max 50)"
// @Success      200  {object}  SearchResponse
// @Failure      400  {object}  ErrorResponse
// @Router       /search [get]
func SearchHandler(c *gin.Context) {
	q := strings.TrimSpace(c.Query("q"))
	if q == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "query parameter 'q' is required"})
		return
	}

	itemType := strings.TrimSpace(c.Query("type"))
	if itemType != "" && itemType != "movie" && itemType != "book" {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "type must be 'movie' or 'book'"})
		return
	}

	limit := 20
	if raw := c.Query("limit"); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil && v > 0 {
			limit = v
		}
	}

	results, err := repository.SearchItems(c.Request.Context(), q, itemType, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "search failed"})
		return
	}

	items := make([]SearchItemResponse, 0, len(results))
	for _, r := range results {
		items = append(items, SearchItemResponse{
			ID:          r.Item.ID,
			Title:       r.Item.Title,
			Description: r.Item.Description,
			ReleaseYear: r.Item.ReleaseYear,
			ImageURL:    r.Item.ImageURL,
			Type:        r.Type,
			Genres:      r.Item.Genres,
			Score:       r.Score,
		})
	}

	c.JSON(http.StatusOK, SearchResponse{
		Items: items,
		Query: q,
		Total: len(items),
	})
}
