package recommendations

import (
	"math"
	"testing"

	"github.com/google/uuid"

	"recommendation-system/internal/models"
)

func TestComputePerFriendWeightedScore_LikedOnly(t *testing.T) {
	score := computePerFriendWeightedScore(true, false, 0.0)
	// liked=1.0 × (1 + 0×1) = 1.0
	if math.Abs(score-1.0) > 1e-9 {
		t.Errorf("got %f, want 1.0", score)
	}
}

func TestComputePerFriendWeightedScore_FavoriteOnly(t *testing.T) {
	score := computePerFriendWeightedScore(false, true, 0.0)
	// fav=3.0 × (1 + 0×1) = 3.0
	if math.Abs(score-3.0) > 1e-9 {
		t.Errorf("got %f, want 3.0", score)
	}
}

func TestComputePerFriendWeightedScore_BothWithSimilarity(t *testing.T) {
	score := computePerFriendWeightedScore(true, true, 0.8)
	// base = 1+3 = 4.0, multiplier = 1 + 0.8×1 = 1.8
	want := 4.0 * 1.8
	if math.Abs(score-want) > 1e-9 {
		t.Errorf("got %f, want %f", score, want)
	}
}

func TestComputePerFriendWeightedScore_NegativeSimilarityClamped(t *testing.T) {
	score := computePerFriendWeightedScore(true, false, -0.5)
	// Negative similarity is clamped to 0: 1.0 × (1 + 0) = 1.0
	if math.Abs(score-1.0) > 1e-9 {
		t.Errorf("got %f, want 1.0 (negative similarity clamped)", score)
	}
}

func TestComputePerFriendWeightedScore_HighSimilarity(t *testing.T) {
	score := computePerFriendWeightedScore(true, false, 1.0)
	// 1.0 × (1 + 1×1) = 2.0
	if math.Abs(score-2.0) > 1e-9 {
		t.Errorf("got %f, want 2.0", score)
	}
}

func TestComputeWeightedFriendScore_NoSimilarityData(t *testing.T) {
	rec := &FriendRecommendation{
		Item:          models.Item{ID: 1},
		FriendCount:   3,
		LikedCount:    2,
		FavoriteCount: 1,
	}

	score := computeWeightedFriendScore(rec, nil)
	// 2×1 + 1×3 = 5.0
	if math.Abs(score-5.0) > 1e-9 {
		t.Errorf("got %f, want 5.0", score)
	}
}

func TestComputeWeightedFriendScore_WithEmptyMap(t *testing.T) {
	rec := &FriendRecommendation{
		Item:          models.Item{ID: 1},
		FriendCount:   2,
		LikedCount:    2,
		FavoriteCount: 0,
	}

	score := computeWeightedFriendScore(rec, map[uuid.UUID]float64{})
	// Empty map → fallback: 2×1 + 0×3 = 2.0
	if math.Abs(score-2.0) > 1e-9 {
		t.Errorf("got %f, want 2.0", score)
	}
}

func TestSortFriendRecommendationsByScore(t *testing.T) {
	recs := []FriendRecommendation{
		{Item: models.Item{ID: 1}, FriendCount: 1, LikedCount: 1},
		{Item: models.Item{ID: 2}, FriendCount: 3, LikedCount: 2, FavoriteCount: 1},
		{Item: models.Item{ID: 3}, FriendCount: 2, LikedCount: 1, FavoriteCount: 1},
	}

	scores := map[int64]float64{
		1: 1.0,
		2: 5.0,
		3: 4.0,
	}

	sortFriendRecommendationsByScore(recs, scores)

	if recs[0].Item.ID != 2 {
		t.Errorf("first item should be ID=2 (score 5.0), got ID=%d", recs[0].Item.ID)
	}
	if recs[1].Item.ID != 3 {
		t.Errorf("second item should be ID=3 (score 4.0), got ID=%d", recs[1].Item.ID)
	}
	if recs[2].Item.ID != 1 {
		t.Errorf("third item should be ID=1 (score 1.0), got ID=%d", recs[2].Item.ID)
	}
}
