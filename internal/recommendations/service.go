package recommendations

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math"
	"slices"
	"sort"
	"strings"

	"github.com/google/uuid"

	"recommendation-system/internal/models"
	"recommendation-system/internal/repository"
)

const (
	defaultLimit                     = 20
	maxLimit                         = 50
	similarItemsPerLikedItem         = 12
	fallbackBaseScore        float64 = 0.30
)

var (
	ErrInvalidLimit           = errors.New("invalid limit")
	ErrInvalidItemID          = errors.New("invalid item_id")
	ErrInvalidInteractionType = errors.New("invalid interaction type")
)

// InteractionResult is returned by AddInteraction to indicate what happened.
type InteractionResult struct {
	ToggledOff bool
	State      repository.InteractionState
}

type Recommendation struct {
	Item   models.Item
	Score  float64
	Reason string
}

type FriendRecommendation struct {
	Item          models.Item
	FriendCount   int
	LikedCount    int
	FavoriteCount int
	Reason        string
}

type recommendationSource struct {
	Item   models.Item
	Signal string
}

type recommendationAggregate struct {
	Item          models.Item
	Score         float64
	MaxSourceGain float64
	Reason        string
}

func AddInteraction(ctx context.Context, userID uuid.UUID, itemID int64, interactionType string) (*InteractionResult, error) {
	if itemID <= 0 {
		return nil, ErrInvalidItemID
	}
	interactionType, err := normalizeInteractionType(interactionType)
	if err != nil {
		return nil, ErrInvalidInteractionType
	}

	if _, err := repository.GetItemByID(ctx, itemID); err != nil {
		return nil, err
	}

	// Check current state for toggle behavior.
	currentState, err := repository.GetUserInteractionState(ctx, userID, itemID)
	if err != nil {
		return nil, fmt.Errorf("get interaction state: %w", err)
	}

	if isInteractionActive(currentState, interactionType) {
		if err := repository.ResetInteractionState(ctx, userID, itemID, interactionType); err != nil {
			return nil, fmt.Errorf("reset interaction state: %w", err)
		}
		newState, _ := repository.GetUserInteractionState(ctx, userID, itemID)
		return &InteractionResult{ToggledOff: true, State: newState}, nil
	}

	if err := repository.UpsertInteraction(ctx, userID, itemID, interactionType); err != nil {
		return nil, err
	}
	newState, _ := repository.GetUserInteractionState(ctx, userID, itemID)
	return &InteractionResult{ToggledOff: false, State: newState}, nil
}

func GetInteractionStates(ctx context.Context, userID uuid.UUID, itemIDs []int64) (map[int64]repository.InteractionState, error) {
	normalizedItemIDs, err := normalizeItemIDs(itemIDs)
	if err != nil {
		return nil, err
	}

	return repository.GetUserInteractionStates(ctx, userID, normalizedItemIDs)
}

func GetRecommendations(ctx context.Context, userID uuid.UUID, limit int, offset int) ([]Recommendation, int, error) {
	limit, err := normalizeLimit(limit)
	if err != nil {
		return nil, 0, err
	}
	if offset < 0 {
		offset = 0
	}

	likedItems, err := repository.GetUserItemsByInteractionType(ctx, userID, repository.InteractionTypeLiked)
	if err != nil {
		return nil, 0, fmt.Errorf("get user liked items: %w", err)
	}

	favoriteItems, err := repository.GetUserItemsByInteractionType(ctx, userID, repository.InteractionTypeFavorite)
	if err != nil {
		return nil, 0, fmt.Errorf("get user favorite items: %w", err)
	}

	sourceItems := buildRecommendationSources(likedItems, favoriteItems)

	excludedItemIDs, err := repository.GetUserExcludedItemIDs(ctx, userID)
	if err != nil {
		return nil, 0, fmt.Errorf("get user excluded items: %w", err)
	}

	excludeSet := make(map[int64]struct{}, len(excludedItemIDs))
	for _, itemID := range excludedItemIDs {
		excludeSet[itemID] = struct{}{}
	}

	// Build the full candidate pool (up to offset + limit)
	poolSize := offset + limit
	recommendations := make([]Recommendation, 0, poolSize)
	if len(sourceItems) > 0 {
		recommendations, err = buildSimilarityRecommendations(ctx, sourceItems, excludeSet, poolSize)
		if err != nil {
			return nil, 0, err
		}
	}

	if len(recommendations) < poolSize {
		recommendations, err = appendFallbackRecommendations(ctx, recommendations, excludeSet, poolSize, len(sourceItems) == 0)
		if err != nil {
			return nil, 0, err
		}
	}

	total := len(recommendations)

	// Apply offset
	if offset >= total {
		return []Recommendation{}, total, nil
	}
	recommendations = recommendations[offset:]

	if len(recommendations) > limit {
		recommendations = recommendations[:limit]
	}

	return recommendations, total, nil
}

func GetFriendRecommendations(ctx context.Context, userID uuid.UUID, limit int) ([]FriendRecommendation, error) {
	limit, err := normalizeLimit(limit)
	if err != nil {
		log.Printf("❌ friend recommendations: invalid limit, user_id=%s raw_limit=%d err=%v", userID, limit, err)
		return nil, err
	}
	log.Printf("ℹ️ friend recommendations: start, user_id=%s limit=%d", userID, limit)

	friendIDs, err := repository.ListAcceptedFriendIDs(ctx, userID)
	if err != nil {
		log.Printf("❌ friend recommendations: list accepted friend ids failed, user_id=%s err=%v", userID, err)
		return nil, fmt.Errorf("get accepted friends: %w", err)
	}
	log.Printf("ℹ️ friend recommendations: accepted friends loaded, user_id=%s friend_count=%d", userID, len(friendIDs))
	if len(friendIDs) == 0 {
		log.Printf("ℹ️ friend recommendations: no accepted friends, user_id=%s", userID)
		return []FriendRecommendation{}, nil
	}

	positiveItems, err := repository.GetPositiveItemsByUserIDs(ctx, friendIDs)
	if err != nil {
		log.Printf("❌ friend recommendations: get positive items failed, user_id=%s friend_count=%d err=%v", userID, len(friendIDs), err)
		return nil, fmt.Errorf("get friends positive items: %w", err)
	}
	log.Printf("ℹ️ friend recommendations: positive items loaded, user_id=%s positive_count=%d", userID, len(positiveItems))
	if len(positiveItems) == 0 {
		log.Printf("ℹ️ friend recommendations: friends have no liked/favorite items, user_id=%s", userID)
		return []FriendRecommendation{}, nil
	}

	excludedItemIDs, err := repository.GetUserExcludedItemIDs(ctx, userID)
	if err != nil {
		log.Printf("❌ friend recommendations: get excluded items failed, user_id=%s err=%v", userID, err)
		return nil, fmt.Errorf("get user excluded items: %w", err)
	}
	log.Printf("ℹ️ friend recommendations: excluded items loaded, user_id=%s excluded_count=%d", userID, len(excludedItemIDs))

	excludeSet := make(map[int64]struct{}, len(excludedItemIDs))
	for _, itemID := range excludedItemIDs {
		excludeSet[itemID] = struct{}{}
	}

	aggregated := make(map[int64]*FriendRecommendation)
	for _, positiveItem := range positiveItems {
		if _, excluded := excludeSet[positiveItem.Item.ID]; excluded {
			continue
		}

		recommendation, exists := aggregated[positiveItem.Item.ID]
		if !exists {
			recommendation = &FriendRecommendation{
				Item: positiveItem.Item,
			}
			aggregated[positiveItem.Item.ID] = recommendation
		}

		recommendation.FriendCount++
		if positiveItem.Liked {
			recommendation.LikedCount++
		}
		if positiveItem.Favorite {
			recommendation.FavoriteCount++
		}
	}

	recommendations := make([]FriendRecommendation, 0, len(aggregated))
	for _, recommendation := range aggregated {
		recommendation.Reason = buildFriendRecommendationReason(recommendation.FriendCount, recommendation.LikedCount, recommendation.FavoriteCount)
		recommendations = append(recommendations, *recommendation)
	}
	log.Printf("ℹ️ friend recommendations: aggregated candidates, user_id=%s candidate_count=%d", userID, len(recommendations))

	sort.Slice(recommendations, func(i, j int) bool {
		if recommendations[i].FriendCount == recommendations[j].FriendCount {
			return recommendations[i].Item.ID > recommendations[j].Item.ID
		}
		return recommendations[i].FriendCount > recommendations[j].FriendCount
	})

	if len(recommendations) > limit {
		recommendations = recommendations[:limit]
	}
	log.Printf("ℹ️ friend recommendations: done, user_id=%s returned=%d", userID, len(recommendations))

	return recommendations, nil
}

func buildSimilarityRecommendations(ctx context.Context, sourceItems []recommendationSource, excludeSet map[int64]struct{}, limit int) ([]Recommendation, error) {
	aggregated := make(map[int64]*recommendationAggregate)

	for _, sourceItem := range sourceItems {
		candidates, err := repository.GetSimilarItemsByItemID(ctx, sourceItem.Item.ID, similarItemsPerLikedItem, mapKeys(excludeSet))
		if err != nil {
			return nil, fmt.Errorf("get similar items for %d: %w", sourceItem.Item.ID, err)
		}

		for _, candidate := range candidates {
			score := similarityScore(candidate.Distance)
			if score <= 0 {
				continue
			}

			aggregate, exists := aggregated[candidate.Item.ID]
			reason := buildSimilarityReason(sourceItem, candidate.Item)
			if !exists {
				aggregated[candidate.Item.ID] = &recommendationAggregate{
					Item:          candidate.Item,
					Score:         score,
					MaxSourceGain: score,
					Reason:        reason,
				}
				continue
			}

			aggregate.Score = combineScores(aggregate.Score, score)
			if score > aggregate.MaxSourceGain {
				aggregate.MaxSourceGain = score
				aggregate.Reason = reason
			}
		}
	}

	recommendations := make([]Recommendation, 0, len(aggregated))
	for itemID, aggregate := range aggregated {
		if _, excluded := excludeSet[itemID]; excluded {
			continue
		}

		recommendations = append(recommendations, Recommendation{
			Item:   aggregate.Item,
			Score:  roundScore(aggregate.Score),
			Reason: aggregate.Reason,
		})
	}

	sort.Slice(recommendations, func(i, j int) bool {
		if recommendations[i].Score == recommendations[j].Score {
			return recommendations[i].Item.ID > recommendations[j].Item.ID
		}
		return recommendations[i].Score > recommendations[j].Score
	})

	if len(recommendations) > limit {
		recommendations = recommendations[:limit]
	}

	for _, recommendation := range recommendations {
		excludeSet[recommendation.Item.ID] = struct{}{}
	}

	return recommendations, nil
}

func appendFallbackRecommendations(ctx context.Context, current []Recommendation, excludeSet map[int64]struct{}, limit int, coldStart bool) ([]Recommendation, error) {
	missing := limit - len(current)
	if missing <= 0 {
		return current, nil
	}

	// For cold-start users, try trending movies first (most engaged in last 30 days)
	if coldStart {
		trendingItems, err := repository.GetTrendingMovies(ctx, missing, 30, mapKeys(excludeSet))
		if err != nil {
			log.Printf("⚠️ recommendations: trending query failed (non-fatal), falling back: %v", err)
		} else {
			for idx, item := range trendingItems {
				excludeSet[item.ID] = struct{}{}
				current = append(current, Recommendation{
					Item:   item,
					Score:  roundScore(math.Max(0.15, fallbackBaseScore-float64(idx)*0.015)),
					Reason: "Trending among our users right now",
				})
			}
			missing = limit - len(current)
		}
	}

	if missing <= 0 {
		return current, nil
	}

	fallbackItems, err := repository.GetPopularOrNewestMovies(ctx, missing, mapKeys(excludeSet))
	if err != nil {
		return nil, fmt.Errorf("get fallback movies: %w", err)
	}

	for idx, item := range fallbackItems {
		excludeSet[item.ID] = struct{}{}

		current = append(current, Recommendation{
			Item:   item,
			Score:  roundScore(math.Max(0.10, fallbackBaseScore-float64(idx)*0.02)),
			Reason: buildFallbackReason(coldStart),
		})
	}

	return current, nil
}

func similarityScore(distance float64) float64 {
	if distance < 0 {
		return 0
	}
	return 1 / (1 + distance)
}

func roundScore(score float64) float64 {
	if score < 0 {
		score = 0
	}
	if score > 0.99 {
		score = 0.99
	}
	return math.Round(score*100) / 100
}

func combineScores(current float64, next float64) float64 {
	return 1 - (1-current)*(1-next)
}

func buildSimilarityReason(source recommendationSource, candidate models.Item) string {
	sharedGenres := intersectGenres(source.Item.Genres, candidate.Genres)
	action := positiveSignalLabel(source.Signal)
	if len(sharedGenres) > 0 {
		return fmt.Sprintf(`Because you %s "%s" and both movies share %s`, action, source.Item.Title, strings.Join(sharedGenres, ", "))
	}
	return fmt.Sprintf(`Because you %s "%s" and this movie has a similar content profile`, action, source.Item.Title)
}

func buildFallbackReason(coldStart bool) string {
	if coldStart {
		return "Popular or recent movie while we learn your preferences"
	}
	return "Popular or recent movie used as fallback while more similarity data is collected"
}

func buildFriendRecommendationReason(friendCount int, likedCount int, favoriteCount int) string {
	switch {
	case favoriteCount > 0 && likedCount > 0:
		return fmt.Sprintf("Liked by %d of your friends and saved by %d of them", likedCount, favoriteCount)
	case favoriteCount > 0 && favoriteCount == 1:
		return "Saved by your friends"
	case favoriteCount > 0:
		return fmt.Sprintf("Saved by %d of your friends", favoriteCount)
	default:
		return fmt.Sprintf("Liked by %d of your friends", likedCount)
	}
}

func intersectGenres(left []string, right []string) []string {
	if len(left) == 0 || len(right) == 0 {
		return nil
	}

	rightSet := make(map[string]struct{}, len(right))
	for _, genre := range right {
		trimmed := strings.TrimSpace(genre)
		if trimmed == "" {
			continue
		}
		rightSet[strings.ToLower(trimmed)] = struct{}{}
	}

	shared := make([]string, 0)
	for _, genre := range left {
		trimmed := strings.TrimSpace(genre)
		if trimmed == "" {
			continue
		}
		if _, ok := rightSet[strings.ToLower(trimmed)]; ok && !slices.Contains(shared, trimmed) {
			shared = append(shared, trimmed)
		}
	}

	if len(shared) > 2 {
		shared = shared[:2]
	}
	return shared
}

func mapKeys(values map[int64]struct{}) []int64 {
	keys := make([]int64, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	return keys
}

func normalizeLimit(limit int) (int, error) {
	if limit < 0 {
		return 0, ErrInvalidLimit
	}
	if limit == 0 {
		limit = defaultLimit
	}
	if limit < 1 {
		return 0, ErrInvalidLimit
	}
	if limit > maxLimit {
		limit = maxLimit
	}

	return limit, nil
}

func normalizeInteractionType(interactionType string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(interactionType)) {
	case "view", repository.InteractionTypeViewed:
		return repository.InteractionTypeViewed, nil
	case "like", repository.InteractionTypeLiked:
		return repository.InteractionTypeLiked, nil
	case repository.InteractionTypeDisliked:
		return repository.InteractionTypeDisliked, nil
	case repository.InteractionTypeFavorite:
		return repository.InteractionTypeFavorite, nil
	case "skip", repository.InteractionTypeSkipped:
		return repository.InteractionTypeSkipped, nil
	default:
		return "", ErrInvalidInteractionType
	}
}

func normalizeItemIDs(itemIDs []int64) ([]int64, error) {
	if len(itemIDs) == 0 {
		return []int64{}, nil
	}

	normalized := make([]int64, 0, len(itemIDs))
	seen := make(map[int64]struct{}, len(itemIDs))
	for _, itemID := range itemIDs {
		if itemID <= 0 {
			return nil, ErrInvalidItemID
		}
		if _, exists := seen[itemID]; exists {
			continue
		}
		seen[itemID] = struct{}{}
		normalized = append(normalized, itemID)
	}

	return normalized, nil
}

func buildRecommendationSources(likedItems []models.Item, favoriteItems []models.Item) []recommendationSource {
	total := len(likedItems) + len(favoriteItems)
	sources := make([]recommendationSource, 0, total)
	seen := make(map[int64]struct{}, total)

	for _, item := range favoriteItems {
		if _, exists := seen[item.ID]; exists {
			continue
		}
		seen[item.ID] = struct{}{}
		sources = append(sources, recommendationSource{
			Item:   item,
			Signal: repository.InteractionTypeFavorite,
		})
	}

	for _, item := range likedItems {
		if _, exists := seen[item.ID]; exists {
			continue
		}
		seen[item.ID] = struct{}{}
		sources = append(sources, recommendationSource{
			Item:   item,
			Signal: repository.InteractionTypeLiked,
		})
	}

	return sources
}

func positiveSignalLabel(signal string) string {
	switch signal {
	case repository.InteractionTypeFavorite:
		return "saved"
	default:
		return "liked"
	}
}

func isInteractionActive(state repository.InteractionState, interactionType string) bool {
	switch interactionType {
	case repository.InteractionTypeViewed:
		return state.Viewed
	case repository.InteractionTypeLiked:
		return state.Liked
	case repository.InteractionTypeDisliked:
		return state.Disliked
	case repository.InteractionTypeFavorite:
		return state.Favorite
	case repository.InteractionTypeSkipped:
		return state.Skipped
	default:
		return false
	}
}

