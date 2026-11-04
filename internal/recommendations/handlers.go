package recommendations

import (
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"recommendation-system/internal/auth"
	"recommendation-system/internal/repository"
)

type AddInteractionRequest struct {
	ItemID int64  `json:"item_id" binding:"required" example:"306"`
	Type   string `json:"type" binding:"required" example:"liked"`
}

type AddInteractionResponse struct {
	Status string                   `json:"status" example:"recorded"`
	State  InteractionStateResponse `json:"state"`
}

type InteractionStateResponse struct {
	Viewed   bool `json:"viewed"`
	Liked    bool `json:"liked"`
	Disliked bool `json:"disliked"`
	Favorite bool `json:"favorite"`
	Skipped  bool `json:"skipped"`
}

type GetInteractionStatesResponse struct {
	Items map[string]InteractionStateResponse `json:"items"`
}

type RecommendationItemResponse struct {
	ID          int64   `json:"id"`
	Title       string  `json:"title"`
	Description string  `json:"description"`
	ReleaseYear int     `json:"release_year"`
	ImageURL    string  `json:"image_url"`
	Score       float64 `json:"score" example:"0.91"`
	Reason      string  `json:"reason" example:"Because you liked similar Ukrainian movies"`
}

type GetRecommendationsResponse struct {
	Items   []RecommendationItemResponse `json:"items"`
	Total   int                          `json:"total"`
	Offset  int                          `json:"offset"`
	HasMore bool                         `json:"has_more"`
}

type FriendRecommendationItemResponse struct {
	ID          int64  `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	ReleaseYear int    `json:"release_year"`
	ImageURL    string `json:"image_url"`
	Score       int    `json:"score" example:"3"`
	Reason      string `json:"reason" example:"Liked by 3 of your friends"`
}

type GetFriendRecommendationsResponse struct {
	Items []FriendRecommendationItemResponse `json:"items"`
}

type ErrorResponse struct {
	Error string `json:"error" example:"invalid request"`
}

// AddInteractionHandler godoc
// @Summary      Записати взаємодію з елементом
// @Description  Зберігає viewed, liked, disliked, favorite або skipped для поточного користувача. Legacy-значення view, like і skip також приймаються.
// @Tags         recommendations
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request body AddInteractionRequest true "Взаємодія користувача"
// @Success      200  {object}  AddInteractionResponse
// @Failure      400  {object}  ErrorResponse "Невірний запит або interaction type"
// @Failure      401  {object}  ErrorResponse "Неавторизовано"
// @Failure      404  {object}  ErrorResponse "Елемент не знайдено"
// @Failure      500  {object}  ErrorResponse "Внутрішня помилка сервера"
// @Router       /interactions [post]
func AddInteractionHandler(c *gin.Context) {
	userID, ok := authenticatedUserID(c)
	if !ok {
		return
	}

	var req AddInteractionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Printf("❌ interactions: invalid request body, err=%v, ip=%s", err, c.ClientIP())
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid request"})
		return
	}

	result, err := AddInteraction(c.Request.Context(), userID, req.ItemID, req.Type)
	if err != nil {
		log.Printf("❌ interactions: add interaction failed, user_id=%s item_id=%d type=%s err=%v", userID, req.ItemID, req.Type, err)
		switch {
		case errors.Is(err, ErrInvalidItemID), errors.Is(err, ErrInvalidInteractionType):
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		case errors.Is(err, repository.ErrItemNotFound):
			c.JSON(http.StatusNotFound, ErrorResponse{Error: err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "internal server error"})
		}
		return
	}

	status := "recorded"
	if result.ToggledOff {
		status = "toggled_off"
	}

	c.JSON(http.StatusOK, AddInteractionResponse{
		Status: status,
		State: InteractionStateResponse{
			Viewed:   result.State.Viewed,
			Liked:    result.State.Liked,
			Disliked: result.State.Disliked,
			Favorite: result.State.Favorite,
			Skipped:  result.State.Skipped,
		},
	})
}

// GetInteractionStatesHandler godoc
// @Summary      Отримати стани взаємодій для списку елементів
// @Description  Повертає поточні стани viewed, liked, disliked, favorite і skipped для переданих item_id.
// @Tags         recommendations
// @Produce      json
// @Security     BearerAuth
// @Param        ids  query     string  true  "CSV список item_id, наприклад 1,2,3"
// @Success      200  {object}  GetInteractionStatesResponse
// @Failure      400  {object}  ErrorResponse "Невірні query-параметри"
// @Failure      401  {object}  ErrorResponse "Неавторизовано"
// @Failure      500  {object}  ErrorResponse "Внутрішня помилка сервера"
// @Router       /interactions/items [get]
func GetInteractionStatesHandler(c *gin.Context) {
	userID, ok := authenticatedUserID(c)
	if !ok {
		return
	}

	itemIDs, err := parseItemIDsParam(c.Query("ids"))
	if err != nil {
		log.Printf("❌ interactions: invalid ids query, raw=%q err=%v ip=%s", c.Query("ids"), err, c.ClientIP())
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid query parameter: ids"})
		return
	}

	states, err := GetInteractionStates(c.Request.Context(), userID, itemIDs)
	if err != nil {
		log.Printf("❌ interactions: get states failed, user_id=%s item_ids=%v err=%v", userID, itemIDs, err)
		switch {
		case errors.Is(err, ErrInvalidItemID):
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "internal server error"})
		}
		return
	}

	responseItems := make(map[string]InteractionStateResponse, len(states))
	for _, itemID := range itemIDs {
		state := states[itemID]
		responseItems[strconv.FormatInt(itemID, 10)] = InteractionStateResponse{
			Viewed:   state.Viewed,
			Liked:    state.Liked,
			Disliked: state.Disliked,
			Favorite: state.Favorite,
			Skipped:  state.Skipped,
		}
	}

	c.JSON(http.StatusOK, GetInteractionStatesResponse{Items: responseItems})
}

// GetRecommendationsHandler godoc
// @Summary      Отримати персональні рекомендації
// @Description  Повертає рекомендації на основі liked/favorite сигналів і pgvector similarity з fallback на популярні або нові фільми.
// @Tags         recommendations
// @Produce      json
// @Security     BearerAuth
// @Param        limit  query     int  false  "Ліміт рекомендацій (default 20, max 50)"
// @Param        offset query     int  false  "Зсув для пагінації (default 0)"
// @Success      200    {object}  GetRecommendationsResponse
// @Failure      400    {object}  ErrorResponse "Невірні query-параметри"
// @Failure      401    {object}  ErrorResponse "Неавторизовано"
// @Failure      500    {object}  ErrorResponse "Внутрішня помилка сервера"
// @Router       /recommendations [get]
func GetRecommendationsHandler(c *gin.Context) {
	userID, ok := authenticatedUserID(c)
	if !ok {
		return
	}

	limit, err := parseOptionalPositiveInt(c.Query("limit"))
	if err != nil {
		log.Printf("❌ recommendations: invalid limit query, raw=%q err=%v ip=%s", c.Query("limit"), err, c.ClientIP())
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid query parameter: limit"})
		return
	}

	offset, err := parseOptionalPositiveInt(c.Query("offset"))
	if err != nil {
		log.Printf("❌ recommendations: invalid offset query, raw=%q err=%v ip=%s", c.Query("offset"), err, c.ClientIP())
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid query parameter: offset"})
		return
	}

	recommendations, total, err := GetRecommendations(c.Request.Context(), userID, limit, offset)
	if err != nil {
		log.Printf("❌ recommendations: get recommendations failed, user_id=%s limit=%d offset=%d err=%v", userID, limit, offset, err)
		if errors.Is(err, ErrInvalidLimit) {
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "internal server error"})
		return
	}

	items := make([]RecommendationItemResponse, 0, len(recommendations))
	for _, recommendation := range recommendations {
		items = append(items, RecommendationItemResponse{
			ID:          recommendation.Item.ID,
			Title:       recommendation.Item.Title,
			Description: recommendation.Item.Description,
			ReleaseYear: recommendation.Item.ReleaseYear,
			ImageURL:    recommendation.Item.ImageURL,
			Score:       recommendation.Score,
			Reason:      recommendation.Reason,
		})
	}

	c.JSON(http.StatusOK, GetRecommendationsResponse{
		Items:   items,
		Total:   total,
		Offset:  offset,
		HasMore: offset+len(items) < total,
	})
}

// GetFriendRecommendationsHandler godoc
// @Summary      Отримати friend-based рекомендації
// @Description  Повертає фільми, які accepted friends поточного користувача лайкнули або зберегли у favorite, відсортовані за силою social signal.
// @Tags         recommendations
// @Produce      json
// @Security     BearerAuth
// @Param        limit  query     int  false  "Ліміт рекомендацій (default 20, max 50)"
// @Success      200    {object}  GetFriendRecommendationsResponse
// @Failure      400    {object}  ErrorResponse "Невірні query-параметри"
// @Failure      401    {object}  ErrorResponse "Неавторизовано"
// @Failure      500    {object}  ErrorResponse "Внутрішня помилка сервера"
// @Router       /recommendations/friends [get]
func GetFriendRecommendationsHandler(c *gin.Context) {
	userID, ok := authenticatedUserID(c)
	if !ok {
		return
	}

	limit, err := parseOptionalPositiveInt(c.Query("limit"))
	if err != nil {
		log.Printf("❌ recommendations: invalid friends limit query, raw=%q err=%v ip=%s", c.Query("limit"), err, c.ClientIP())
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid query parameter: limit"})
		return
	}

	recommendations, err := GetFriendRecommendations(c.Request.Context(), userID, limit)
	if err != nil {
		log.Printf("❌ recommendations: get friend recommendations failed, user_id=%s limit=%d err=%v", userID, limit, err)
		if errors.Is(err, ErrInvalidLimit) {
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "internal server error"})
		return
	}

	items := make([]FriendRecommendationItemResponse, 0, len(recommendations))
	for _, recommendation := range recommendations {
		items = append(items, FriendRecommendationItemResponse{
			ID:          recommendation.Item.ID,
			Title:       recommendation.Item.Title,
			Description: recommendation.Item.Description,
			ReleaseYear: recommendation.Item.ReleaseYear,
			ImageURL:    recommendation.Item.ImageURL,
			Score:       recommendation.FriendCount,
			Reason:      recommendation.Reason,
		})
	}

	c.JSON(http.StatusOK, GetFriendRecommendationsResponse{Items: items})
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

func parseOptionalPositiveInt(raw string) (int, error) {
	if raw == "" {
		return 0, nil
	}

	value, err := strconv.Atoi(raw)
	if err != nil {
		return 0, err
	}
	if value < 0 {
		return 0, ErrInvalidLimit
	}

	return value, nil
}

func parseItemIDsParam(raw string) ([]int64, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, ErrInvalidItemID
	}

	parts := strings.Split(raw, ",")
	itemIDs := make([]int64, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" {
			return nil, ErrInvalidItemID
		}

		itemID, err := strconv.ParseInt(trimmed, 10, 64)
		if err != nil || itemID <= 0 {
			return nil, ErrInvalidItemID
		}

		itemIDs = append(itemIDs, itemID)
	}

	return itemIDs, nil
}
