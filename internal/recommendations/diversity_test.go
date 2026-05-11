package recommendations

import (
	"testing"

	"recommendation-system/internal/models"
)

// --- getDominantGenre tests ---

func TestGetDominantGenre_ReturnsFirst(t *testing.T) {
	genres := []string{"Action", "Comedy", "Drama"}
	got := getDominantGenre(genres)
	if got != "action" {
		t.Errorf("getDominantGenre(%v) = %q, want \"action\"", genres, got)
	}
}

func TestGetDominantGenre_Empty(t *testing.T) {
	got := getDominantGenre(nil)
	if got != "" {
		t.Errorf("getDominantGenre(nil) = %q, want \"\"", got)
	}
}

func TestGetDominantGenre_SkipsBlankFirst(t *testing.T) {
	genres := []string{"  ", "Drama"}
	got := getDominantGenre(genres)
	if got != "drama" {
		t.Errorf("getDominantGenre(%v) = %q, want \"drama\"", genres, got)
	}
}

func TestGetDominantGenre_NormalizesCase(t *testing.T) {
	genres := []string{"SCI-FI"}
	got := getDominantGenre(genres)
	if got != "sci-fi" {
		t.Errorf("getDominantGenre(%v) = %q, want \"sci-fi\"", genres, got)
	}
}

// --- diversifyRecommendations tests ---

func makeRec(id int64, score float64, genres ...string) Recommendation {
	return Recommendation{
		Item: models.Item{
			ID:     id,
			Title:  "Movie",
			Genres: genres,
		},
		Score:  score,
		Reason: "test",
	}
}

func TestDiversifyRecommendations_LimitsPerGenre(t *testing.T) {
	// 5 action movies, maxPerGenre = 2
	recs := []Recommendation{
		makeRec(1, 0.95, "Action"),
		makeRec(2, 0.90, "Action"),
		makeRec(3, 0.85, "Action"),
		makeRec(4, 0.80, "Action"),
		makeRec(5, 0.75, "Action"),
	}

	result := diversifyRecommendations(recs, 2)

	if len(result) != 5 {
		t.Fatalf("expected 5 results (total preserved), got %d", len(result))
	}

	// First 2 should be accepted action movies (ID=1, ID=2)
	if result[0].Item.ID != 1 || result[1].Item.ID != 2 {
		t.Errorf("first two should be ID 1,2 (accepted action); got %d,%d",
			result[0].Item.ID, result[1].Item.ID)
	}

	// Last 3 should be the deferred action movies
	if result[2].Item.ID != 3 || result[3].Item.ID != 4 || result[4].Item.ID != 5 {
		t.Errorf("last 3 should be deferred action IDs 3,4,5; got %d,%d,%d",
			result[2].Item.ID, result[3].Item.ID, result[4].Item.ID)
	}
}

func TestDiversifyRecommendations_MixedGenres(t *testing.T) {
	// Interleaved genres — maxPerGenre = 2
	recs := []Recommendation{
		makeRec(1, 0.95, "Action"),
		makeRec(2, 0.90, "Action"),
		makeRec(3, 0.85, "Action"),
		makeRec(4, 0.80, "Drama"),
		makeRec(5, 0.75, "Drama"),
		makeRec(6, 0.70, "Comedy"),
	}

	result := diversifyRecommendations(recs, 2)

	if len(result) != 6 {
		t.Fatalf("expected 6 results, got %d", len(result))
	}

	// First 5 should be: Action(1), Action(2), Drama(4), Drama(5), Comedy(6)
	// Then deferred: Action(3)
	accepted := result[:5]
	deferred := result[5:]

	// All accepted IDs
	acceptedIDs := make([]int64, len(accepted))
	for i, r := range accepted {
		acceptedIDs[i] = r.Item.ID
	}

	if deferred[0].Item.ID != 3 {
		t.Errorf("deferred[0] should be ID=3 (3rd action), got %d", deferred[0].Item.ID)
	}

	// Verify action count in accepted
	actionCount := 0
	for _, r := range accepted {
		if getDominantGenre(r.Item.Genres) == "action" {
			actionCount++
		}
	}
	if actionCount != 2 {
		t.Errorf("expected 2 action items in accepted, got %d", actionCount)
	}
}

func TestDiversifyRecommendations_NoGenreAlwaysAccepted(t *testing.T) {
	recs := []Recommendation{
		makeRec(1, 0.95, "Action"),
		makeRec(2, 0.90),          // no genres
		makeRec(3, 0.85, "Action"),
		makeRec(4, 0.80),          // no genres
	}

	result := diversifyRecommendations(recs, 1)

	// Only 1 action allowed. Items with no genres are always accepted.
	// Expected accepted order: Action(1), NoGenre(2), NoGenre(4)
	// Deferred: Action(3)
	if len(result) != 4 {
		t.Fatalf("expected 4 results, got %d", len(result))
	}

	if result[0].Item.ID != 1 {
		t.Errorf("result[0] should be ID=1 (first action), got %d", result[0].Item.ID)
	}
	if result[1].Item.ID != 2 {
		t.Errorf("result[1] should be ID=2 (no genre), got %d", result[1].Item.ID)
	}
	if result[2].Item.ID != 4 {
		t.Errorf("result[2] should be ID=4 (no genre), got %d", result[2].Item.ID)
	}
	if result[3].Item.ID != 3 {
		t.Errorf("result[3] should be ID=3 (deferred action), got %d", result[3].Item.ID)
	}
}

func TestDiversifyRecommendations_EmptyInput(t *testing.T) {
	result := diversifyRecommendations(nil, 3)
	if result != nil {
		t.Errorf("expected nil for nil input, got %v", result)
	}
}

func TestDiversifyRecommendations_DefaultCap(t *testing.T) {
	// When maxPerGenre <= 0, should use defaultMaxPerGenre (4).
	recs := make([]Recommendation, 6)
	for i := range recs {
		recs[i] = makeRec(int64(i+1), 0.9-float64(i)*0.05, "Action")
	}

	result := diversifyRecommendations(recs, 0)

	// With default cap of 4, first 4 are accepted, last 2 deferred
	if len(result) != 6 {
		t.Fatalf("expected 6 results, got %d", len(result))
	}

	// Count action in first 4 positions
	actionInTop := 0
	for _, r := range result[:4] {
		if getDominantGenre(r.Item.Genres) == "action" {
			actionInTop++
		}
	}
	if actionInTop != 4 {
		t.Errorf("expected 4 action items in top 4, got %d", actionInTop)
	}
}

func TestDiversifyRecommendations_PreservesTotalCount(t *testing.T) {
	recs := make([]Recommendation, 20)
	for i := range recs {
		recs[i] = makeRec(int64(i+1), 0.99-float64(i)*0.01, "Thriller")
	}

	result := diversifyRecommendations(recs, 3)
	if len(result) != 20 {
		t.Errorf("diversification must preserve total count: got %d, want 20", len(result))
	}
}
