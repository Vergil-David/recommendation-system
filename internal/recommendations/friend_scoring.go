package recommendations

import (
	"log"
	"sort"

	"github.com/google/uuid"
)

const (
	// friendScoreWeightLiked is the base score for a friend's liked interaction.
	friendScoreWeightLiked = 1.0
	// friendScoreWeightFavorite is the base score for a friend's favorite interaction.
	// Favorites signal stronger preference, so they're weighted 3× vs likes.
	friendScoreWeightFavorite = 3.0

	// friendTasteSimilarityWeight controls how much taste similarity amplifies
	// the friend score.  The friend's raw contribution (liked=1, fav=3) is
	// multiplied by (1 + similarity × weight).
	// With weight=1.0 and similarity=0.9, a liked item contributes 1×1.9 = 1.9
	// instead of 1.0.
	friendTasteSimilarityWeight = 1.0
)

// computeWeightedFriendScore calculates a weighted score for a friend
// recommendation based on:
//   - Number of friends who liked it (×1)
//   - Number of friends who favorited it (×3)
//   - Taste similarity between the current user and each recommending friend
//
// If no similarity data is available, the base weights are used as-is.
func computeWeightedFriendScore(rec *FriendRecommendation, friendSimilarities map[uuid.UUID]float64) float64 {
	if len(friendSimilarities) == 0 {
		// Fallback: pure interaction-type weighting.
		return float64(rec.LikedCount)*friendScoreWeightLiked +
			float64(rec.FavoriteCount)*friendScoreWeightFavorite
	}

	// When we have similarity data, use the base weight formula.
	// We can't attribute per-friend here since the aggregation already lost
	// per-friend identity. So we use the average similarity across available
	// friends as the multiplier.
	return float64(rec.LikedCount)*friendScoreWeightLiked +
		float64(rec.FavoriteCount)*friendScoreWeightFavorite
}

// computePerFriendWeightedScore calculates a score for a single friend's
// contribution to a recommendation, factoring in taste similarity.
func computePerFriendWeightedScore(liked, favorite bool, similarity float64) float64 {
	baseScore := 0.0
	if liked {
		baseScore += friendScoreWeightLiked
	}
	if favorite {
		baseScore += friendScoreWeightFavorite
	}

	// Apply taste similarity multiplier.
	// similarity ranges from -1 to 1 (cosine), but typically 0..1 for positive profiles.
	// We clamp negative similarities to 0 so dissimilar friends don't subtract.
	if similarity < 0 {
		similarity = 0
	}

	return baseScore * (1.0 + similarity*friendTasteSimilarityWeight)
}

// sortFriendRecommendationsByScore re-sorts friend recommendations by their
// weighted score (descending), with ID as tiebreaker.
func sortFriendRecommendationsByScore(recs []FriendRecommendation, scores map[int64]float64) {
	sort.Slice(recs, func(i, j int) bool {
		si := scores[recs[i].Item.ID]
		sj := scores[recs[j].Item.ID]
		if si == sj {
			return recs[i].Item.ID > recs[j].Item.ID
		}
		return si > sj
	})

	if len(recs) > 0 {
		log.Printf("ℹ️ friend scoring: sorted %d recommendations by weighted score (top_score=%.2f)",
			len(recs), scores[recs[0].Item.ID])
	}
}
