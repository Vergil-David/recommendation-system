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
	profileVectorCandidatePool       = 60
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
		scheduleProfileRebuildIfNeeded(ctx, userID, interactionType)
		return &InteractionResult{ToggledOff: true, State: newState}, nil
	}

	if err := repository.UpsertInteraction(ctx, userID, itemID, interactionType); err != nil {
		return nil, err
	}
	newState, _ := repository.GetUserInteractionState(ctx, userID, itemID)
	scheduleProfileRebuildIfNeeded(ctx, userID, interactionType)
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

	// Strategy 1: Profile-vector based (single pgvector query — O(1))
	profileVector, profileErr := repository.GetUserProfileEmbedding(ctx, userID)
	if profileErr != nil {
		log.Printf("⚠️ recommendations: failed to load profile vector, falling back: %v", profileErr)
	}

	if profileVector != nil {
		recommendations, err = buildProfileVectorRecommendations(ctx, userID, profileVector, excludeSet, poolSize)
		if err != nil {
			log.Printf("⚠️ recommendations: profile-vector search failed, falling back: %v", err)
		}
	}

	// Strategy 2: Per-item similarity (legacy, O(N) — used when no profile vector)
	if len(recommendations) == 0 {
		likedItems, err := repository.GetUserItemsByInteractionType(ctx, userID, repository.InteractionTypeLiked)
		if err != nil {
			return nil, 0, fmt.Errorf("get user liked items: %w", err)
		}

		favoriteItems, err := repository.GetUserItemsByInteractionType(ctx, userID, repository.InteractionTypeFavorite)
		if err != nil {
			return nil, 0, fmt.Errorf("get user favorite items: %w", err)
		}

		sourceItems := buildRecommendationSources(likedItems, favoriteItems)

		if len(sourceItems) > 0 {
			recommendations, err = buildSimilarityRecommendations(ctx, sourceItems, excludeSet, poolSize)
			if err != nil {
				return nil, 0, err
			}
		}

		// Trigger async profile rebuild so next request uses the fast path.
		if len(sourceItems) > 0 {
			go func() {
				if rebuildErr := RebuildUserProfileVector(context.Background(), userID); rebuildErr != nil {
					log.Printf("⚠️ recommendations: async profile rebuild failed: %v", rebuildErr)
				}
			}()
		}
	}

	coldStart := len(recommendations) == 0 && profileVector == nil

	if len(recommendations) < poolSize {
		recommendations, err = appendFallbackRecommendations(ctx, recommendations, excludeSet, poolSize, coldStart)
		if err != nil {
			return nil, 0, err
		}
	}

	// --- Genre-based boost ---
	// Boost candidates from the user's preferred genres before applying penalties.
	if len(recommendations) > 0 && !coldStart {
		genreProfile := loadUserGenreProfile(ctx, userID)
		if !genreProfile.IsEmpty() {
			ApplyGenreBoost(recommendations, genreProfile)
		}
	}

	// --- Negative signal penalty ---
	// Penalize candidates that are semantically similar to disliked/skipped items.
	if len(recommendations) > 0 && !coldStart {
		negativeEmbeddings, negErr := repository.GetUserNegativeItemEmbeddings(ctx, userID)
		if negErr != nil {
			log.Printf("⚠️ recommendations: failed to load negative embeddings (non-fatal): %v", negErr)
		}
		if len(negativeEmbeddings) > 0 {
			negativeCentroid := buildNegativeCentroid(negativeEmbeddings)
			if negativeCentroid != nil {
				// Batch-load candidate embeddings for cosine comparison.
				candidateIDs := make([]int64, len(recommendations))
				for i, rec := range recommendations {
					candidateIDs[i] = rec.Item.ID
				}
				candidateEmbeddings, embErr := repository.GetItemEmbeddingsByIDs(ctx, candidateIDs)
				if embErr != nil {
					log.Printf("⚠️ recommendations: failed to load candidate embeddings (non-fatal): %v", embErr)
				} else {
					applyNegativePenalty(recommendations, candidateEmbeddings, negativeCentroid)
				}
			}
		}
	}

	// --- Genre diversity re-ranking ---
	// Ensure no single genre dominates the result list.
	if len(recommendations) > 0 && !coldStart {
		recommendations = diversifyRecommendations(recommendations, defaultMaxPerGenre)
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

	// --- Compute taste similarity between user and each friend ---
	// Load profile embeddings for the user and all friends.
	allUserIDs := make([]uuid.UUID, 0, len(friendIDs)+1)
	allUserIDs = append(allUserIDs, userID)
	allUserIDs = append(allUserIDs, friendIDs...)

	profileEmbeddings, profileErr := repository.GetProfileEmbeddingsByUserIDs(ctx, allUserIDs)
	if profileErr != nil {
		log.Printf("⚠️ friend recommendations: failed to load profile embeddings (non-fatal): %v", profileErr)
		profileEmbeddings = nil
	}

	// Compute cosine similarity between the current user and each friend.
	friendSimilarities := make(map[uuid.UUID]float64, len(friendIDs))
	var userProfile []float32
	if profileEmbeddings != nil {
		userProfile = profileEmbeddings[userID]
	}
	if userProfile != nil {
		for _, friendID := range friendIDs {
			friendProfile := profileEmbeddings[friendID]
			if friendProfile != nil {
				sim := cosineSimilarity(userProfile, friendProfile)
				friendSimilarities[friendID] = sim
			}
		}
		log.Printf("ℹ️ friend recommendations: computed taste similarities, user_id=%s friends_with_profile=%d",
			userID, len(friendSimilarities))
	}

	// --- Aggregate per-item scores across friends ---
	type friendItemAggregate struct {
		Rec   *FriendRecommendation
		Score float64
	}
	aggregated := make(map[int64]*friendItemAggregate)

	for _, positiveItem := range positiveItems {
		if _, excluded := excludeSet[positiveItem.Item.ID]; excluded {
			continue
		}

		agg, exists := aggregated[positiveItem.Item.ID]
		if !exists {
			agg = &friendItemAggregate{
				Rec: &FriendRecommendation{
					Item: positiveItem.Item,
				},
			}
			aggregated[positiveItem.Item.ID] = agg
		}

		agg.Rec.FriendCount++
		if positiveItem.Liked {
			agg.Rec.LikedCount++
		}
		if positiveItem.Favorite {
			agg.Rec.FavoriteCount++
		}

		// Add this friend's weighted contribution.
		similarity := friendSimilarities[positiveItem.UserID] // 0 if unknown
		agg.Score += computePerFriendWeightedScore(positiveItem.Liked, positiveItem.Favorite, similarity)
	}

	recommendations := make([]FriendRecommendation, 0, len(aggregated))
	itemScores := make(map[int64]float64, len(aggregated))
	for _, agg := range aggregated {
		agg.Rec.Reason = buildFriendRecommendationReason(agg.Rec.FriendCount, agg.Rec.LikedCount, agg.Rec.FavoriteCount)
		recommendations = append(recommendations, *agg.Rec)
		itemScores[agg.Rec.Item.ID] = agg.Score
	}
	log.Printf("ℹ️ friend recommendations: aggregated candidates, user_id=%s candidate_count=%d", userID, len(recommendations))

	// Sort by weighted score (replaces old FriendCount-only sort).
	sortFriendRecommendationsByScore(recommendations, itemScores)

	if len(recommendations) > limit {
		recommendations = recommendations[:limit]
	}
	log.Printf("ℹ️ friend recommendations: done, user_id=%s returned=%d", userID, len(recommendations))

	return recommendations, nil
}

// buildProfileVectorRecommendations uses the precomputed user profile embedding
// for a single pgvector nearest-neighbor query instead of N per-item queries.
func buildProfileVectorRecommendations(ctx context.Context, userID uuid.UUID, profileVector []float32, excludeSet map[int64]struct{}, limit int) ([]Recommendation, error) {
	// Fetch more candidates than needed so we can filter and rank.
	candidateLimit := limit
	if candidateLimit < profileVectorCandidatePool {
		candidateLimit = profileVectorCandidatePool
	}

	candidates, err := repository.FindSimilarByProfileVector(ctx, profileVector, candidateLimit, mapKeys(excludeSet))
	if err != nil {
		return nil, fmt.Errorf("profile vector search: %w", err)
	}

	if len(candidates) == 0 {
		return nil, nil
	}

	recommendations := make([]Recommendation, 0, len(candidates))
	for _, candidate := range candidates {
		score := similarityScore(candidate.Distance)
		if score <= 0 {
			continue
		}

		if _, excluded := excludeSet[candidate.Item.ID]; excluded {
			continue
		}

		recommendations = append(recommendations, Recommendation{
			Item:   candidate.Item,
			Score:  roundScore(score),
			Reason: "Recommended based on your taste profile",
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

	for _, rec := range recommendations {
		excludeSet[rec.Item.ID] = struct{}{}
	}

	log.Printf("ℹ️ profile-vector recommendations: user_id=%s returned=%d", userID, len(recommendations))
	return recommendations, nil
}

// scheduleProfileRebuildIfNeeded triggers an async profile vector rebuild
// when the interaction type is liked or favorite (the signals that form
// the profile).
func scheduleProfileRebuildIfNeeded(ctx context.Context, userID uuid.UUID, interactionType string) {
	if interactionType != repository.InteractionTypeLiked && interactionType != repository.InteractionTypeFavorite {
		return
	}

	go func() {
		if err := RebuildUserProfileVector(context.Background(), userID); err != nil {
			log.Printf("⚠️ profile-vector: async rebuild failed for user %s: %v", userID, err)
		}
	}()
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

// loadUserGenreProfile fetches the user's liked and favorited items and builds
// a GenreProfile.  Errors are logged but treated as non-fatal (an empty profile
// means no genre boost is applied).
func loadUserGenreProfile(ctx context.Context, userID uuid.UUID) GenreProfile {
	likedItems, err := repository.GetUserItemsByInteractionType(ctx, userID, repository.InteractionTypeLiked)
	if err != nil {
		log.Printf("⚠️ genre boost: failed to load liked items for user %s (non-fatal): %v", userID, err)
		return GenreProfile{GenreWeights: map[string]float64{}}
	}

	favoriteItems, err := repository.GetUserItemsByInteractionType(ctx, userID, repository.InteractionTypeFavorite)
	if err != nil {
		log.Printf("⚠️ genre boost: failed to load favorite items for user %s (non-fatal): %v", userID, err)
		return GenreProfile{GenreWeights: map[string]float64{}}
	}

	profile := BuildGenreProfile(likedItems, favoriteItems)
	if !profile.IsEmpty() {
		log.Printf("ℹ️ genre boost: built profile for user %s (genres=%d, liked=%d, favorites=%d)",
			userID, len(profile.GenreWeights), len(likedItems), len(favoriteItems))
	}

	return profile
}
