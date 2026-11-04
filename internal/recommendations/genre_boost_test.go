package recommendations

import (
	"math"
	"testing"

	"recommendation-system/internal/models"
)

// --- BuildGenreProfile tests ---

func TestBuildGenreProfile_Empty(t *testing.T) {
	profile := BuildGenreProfile(nil, nil)
	if !profile.IsEmpty() {
		t.Errorf("expected empty profile for nil inputs, got %v", profile.GenreWeights)
	}
}

func TestBuildGenreProfile_EmptySlices(t *testing.T) {
	profile := BuildGenreProfile([]models.Item{}, []models.Item{})
	if !profile.IsEmpty() {
		t.Errorf("expected empty profile for empty slices, got %v", profile.GenreWeights)
	}
}

func TestBuildGenreProfile_NoGenres(t *testing.T) {
	liked := []models.Item{
		{ID: 1, Title: "No Genre Movie", Genres: nil},
		{ID: 2, Title: "Also No Genre", Genres: []string{}},
	}
	profile := BuildGenreProfile(liked, nil)
	if !profile.IsEmpty() {
		t.Errorf("expected empty profile for items with no genres, got %v", profile.GenreWeights)
	}
}

func TestBuildGenreProfile_LikedOnly(t *testing.T) {
	liked := []models.Item{
		{ID: 1, Genres: []string{"Action", "Drama"}},
		{ID: 2, Genres: []string{"Action", "Comedy"}},
	}
	profile := BuildGenreProfile(liked, nil)

	if profile.IsEmpty() {
		t.Fatal("expected non-empty profile")
	}

	// Action appears twice (weight 1+1=2), Drama once (1), Comedy once (1)
	// Total raw weight = 4, so Action=0.5, Drama=0.25, Comedy=0.25
	assertWeight(t, profile, "action", 0.5)
	assertWeight(t, profile, "drama", 0.25)
	assertWeight(t, profile, "comedy", 0.25)
}

func TestBuildGenreProfile_FavoritesWeightedDouble(t *testing.T) {
	liked := []models.Item{
		{ID: 1, Genres: []string{"Action"}}, // weight 1.0
	}
	favorited := []models.Item{
		{ID: 2, Genres: []string{"Drama"}}, // weight 2.0
	}
	profile := BuildGenreProfile(liked, favorited)

	// Action=1.0, Drama=2.0, total=3.0
	assertWeight(t, profile, "action", 1.0/3.0)
	assertWeight(t, profile, "drama", 2.0/3.0)
}

func TestBuildGenreProfile_OverlappingGenres(t *testing.T) {
	liked := []models.Item{
		{ID: 1, Genres: []string{"Sci-Fi"}}, // weight 1.0
	}
	favorited := []models.Item{
		{ID: 2, Genres: []string{"Sci-Fi"}}, // weight 2.0
	}
	profile := BuildGenreProfile(liked, favorited)

	// Sci-Fi = 1.0 + 2.0 = 3.0, total=3.0 → weight=1.0
	assertWeight(t, profile, "sci-fi", 1.0)
}

func TestBuildGenreProfile_CaseInsensitive(t *testing.T) {
	liked := []models.Item{
		{ID: 1, Genres: []string{"Action"}},
		{ID: 2, Genres: []string{"action"}},
		{ID: 3, Genres: []string{"ACTION"}},
	}
	profile := BuildGenreProfile(liked, nil)

	// All three should merge into "action" with weight 1.0
	if len(profile.GenreWeights) != 1 {
		t.Errorf("expected 1 genre, got %d: %v", len(profile.GenreWeights), profile.GenreWeights)
	}
	assertWeight(t, profile, "action", 1.0)
}

func TestBuildGenreProfile_SkipsEmptyGenreStrings(t *testing.T) {
	liked := []models.Item{
		{ID: 1, Genres: []string{"Action", "", "  ", "Drama"}},
	}
	profile := BuildGenreProfile(liked, nil)

	if len(profile.GenreWeights) != 2 {
		t.Errorf("expected 2 genres (skip empty), got %d: %v", len(profile.GenreWeights), profile.GenreWeights)
	}
}

func TestBuildGenreProfile_NormalizesToOne(t *testing.T) {
	liked := []models.Item{
		{ID: 1, Genres: []string{"A", "B", "C"}},
		{ID: 2, Genres: []string{"B", "D"}},
	}
	favorited := []models.Item{
		{ID: 3, Genres: []string{"A", "E"}},
	}
	profile := BuildGenreProfile(liked, favorited)

	totalWeight := 0.0
	for _, w := range profile.GenreWeights {
		totalWeight += w
	}
	if math.Abs(totalWeight-1.0) > 1e-9 {
		t.Errorf("genre weights should sum to 1.0, got %f", totalWeight)
	}
}

// --- GenreProfile.IsEmpty tests ---

func TestGenreProfile_IsEmpty_True(t *testing.T) {
	gp := GenreProfile{GenreWeights: map[string]float64{}}
	if !gp.IsEmpty() {
		t.Error("expected empty")
	}
}

func TestGenreProfile_IsEmpty_False(t *testing.T) {
	gp := GenreProfile{GenreWeights: map[string]float64{"action": 0.5}}
	if gp.IsEmpty() {
		t.Error("expected non-empty")
	}
}

// --- genreOverlap tests ---

func TestGenreOverlap_NoOverlap(t *testing.T) {
	weights := map[string]float64{"action": 0.5, "drama": 0.5}
	overlap := genreOverlap([]string{"Comedy", "Horror"}, weights)
	if overlap != 0 {
		t.Errorf("expected 0, got %f", overlap)
	}
}

func TestGenreOverlap_FullOverlap(t *testing.T) {
	weights := map[string]float64{"action": 0.6, "drama": 0.4}
	overlap := genreOverlap([]string{"Action", "Drama"}, weights)
	if math.Abs(overlap-1.0) > 1e-9 {
		t.Errorf("expected 1.0, got %f", overlap)
	}
}

func TestGenreOverlap_PartialOverlap(t *testing.T) {
	weights := map[string]float64{"action": 0.5, "drama": 0.3, "comedy": 0.2}
	overlap := genreOverlap([]string{"Action", "Comedy"}, weights)
	expected := 0.5 + 0.2
	if math.Abs(overlap-expected) > 1e-9 {
		t.Errorf("expected %f, got %f", expected, overlap)
	}
}

func TestGenreOverlap_EmptyGenres(t *testing.T) {
	weights := map[string]float64{"action": 0.5}
	if genreOverlap(nil, weights) != 0 {
		t.Error("expected 0 for nil genres")
	}
	if genreOverlap([]string{}, weights) != 0 {
		t.Error("expected 0 for empty genres")
	}
}

func TestGenreOverlap_EmptyProfile(t *testing.T) {
	overlap := genreOverlap([]string{"Action"}, map[string]float64{})
	if overlap != 0 {
		t.Errorf("expected 0 for empty profile, got %f", overlap)
	}
}

// --- ApplyGenreBoost tests ---

func TestApplyGenreBoost_BoostsMatchingGenres(t *testing.T) {
	recs := []Recommendation{
		{Item: models.Item{ID: 1, Genres: []string{"Action"}}, Score: 0.80},
		{Item: models.Item{ID: 2, Genres: []string{"Comedy"}}, Score: 0.70},
	}

	profile := GenreProfile{GenreWeights: map[string]float64{
		"action": 0.8,
		"drama":  0.2,
	}}

	ApplyGenreBoost(recs, profile)

	// Item 1 (Action) should be boosted: 0.80 * (1 + 0.8 * 0.3) = 0.80 * 1.24 = 0.992 → capped at 0.99
	// Item 2 (Comedy) should be unchanged: 0.70
	if recs[0].Item.ID != 1 {
		t.Errorf("expected item 1 first after boost, got %d", recs[0].Item.ID)
	}
	if recs[0].Score <= 0.80 {
		t.Errorf("expected item 1 score to increase, got %f", recs[0].Score)
	}
	if recs[1].Score != 0.70 {
		t.Errorf("expected item 2 score unchanged at 0.70, got %f", recs[1].Score)
	}
}

func TestApplyGenreBoost_ReranksAfterBoost(t *testing.T) {
	recs := []Recommendation{
		{Item: models.Item{ID: 1, Genres: []string{"Comedy"}}, Score: 0.80},    // no matching genre
		{Item: models.Item{ID: 2, Genres: []string{"Action"}}, Score: 0.70},    // matches
	}

	// Profile heavily favors Action
	profile := GenreProfile{GenreWeights: map[string]float64{
		"action": 1.0,
	}}

	ApplyGenreBoost(recs, profile)

	// Item 2 was 0.70, boosted by 30% → 0.91
	// Item 1 stays at 0.80
	// After re-sort, item 2 should be first
	if recs[0].Item.ID != 2 {
		t.Errorf("expected item 2 to be reranked first after genre boost, got ID=%d (score=%f)", recs[0].Item.ID, recs[0].Score)
	}
}

func TestApplyGenreBoost_EmptyProfile(t *testing.T) {
	recs := []Recommendation{
		{Item: models.Item{ID: 1, Genres: []string{"Action"}}, Score: 0.80},
	}
	profile := GenreProfile{GenreWeights: map[string]float64{}}

	ApplyGenreBoost(recs, profile)

	if recs[0].Score != 0.80 {
		t.Errorf("score should be unchanged with empty profile, got %f", recs[0].Score)
	}
}

func TestApplyGenreBoost_EmptyRecommendations(t *testing.T) {
	// Should not panic.
	profile := GenreProfile{GenreWeights: map[string]float64{"action": 1.0}}
	ApplyGenreBoost(nil, profile)
	ApplyGenreBoost([]Recommendation{}, profile)
}

func TestApplyGenreBoost_ScoreCappedAt099(t *testing.T) {
	recs := []Recommendation{
		{Item: models.Item{ID: 1, Genres: []string{"Action"}}, Score: 0.95},
	}
	profile := GenreProfile{GenreWeights: map[string]float64{"action": 1.0}}

	ApplyGenreBoost(recs, profile)

	// 0.95 * 1.3 = 1.235 → roundScore caps at 0.99
	if recs[0].Score > 0.99 {
		t.Errorf("score should be capped at 0.99, got %f", recs[0].Score)
	}
}

func TestApplyGenreBoost_MultipleGenresStack(t *testing.T) {
	recs := []Recommendation{
		{Item: models.Item{ID: 1, Genres: []string{"Action", "Drama"}}, Score: 0.60},
		{Item: models.Item{ID: 2, Genres: []string{"Comedy"}}, Score: 0.60},
	}

	profile := GenreProfile{GenreWeights: map[string]float64{
		"action": 0.3,
		"drama":  0.3,
		"comedy": 0.4,
	}}

	ApplyGenreBoost(recs, profile)

	// Item 1: overlap = 0.3 + 0.3 = 0.6, boost = 1 + 0.6*0.3 = 1.18, score = 0.60*1.18 = 0.708
	// Item 2: overlap = 0.4, boost = 1 + 0.4*0.3 = 1.12, score = 0.60*1.12 = 0.672
	if recs[0].Item.ID != 1 {
		t.Errorf("item 1 (more genre overlap) should rank first, got ID=%d", recs[0].Item.ID)
	}
}

func TestApplyGenreBoost_OverlapCappedAtOne(t *testing.T) {
	// Construct a case where raw overlap > 1.0 (item has genres that collectively
	// exceed the profile total — can happen if an item has many genres and all
	// are in the profile with high weights).
	recs := []Recommendation{
		{Item: models.Item{ID: 1, Genres: []string{"A", "B", "C", "D"}}, Score: 0.50},
	}

	// Each genre has weight 0.4 → total would be 1.6 but capped at 1.0
	profile := GenreProfile{GenreWeights: map[string]float64{
		"a": 0.4, "b": 0.4, "c": 0.4, "d": 0.4,
	}}

	ApplyGenreBoost(recs, profile)

	// Max boost = 1 + 1.0*0.3 = 1.3, score = 0.50 * 1.3 = 0.65
	if math.Abs(recs[0].Score-0.65) > 0.01 {
		t.Errorf("expected score ~0.65 (overlap capped at 1.0), got %f", recs[0].Score)
	}
}

// --- normalizeGenreName tests ---

func TestNormalizeGenreName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Action", "action"},
		{"  Drama  ", "drama"},
		{"SCI-FI", "sci-fi"},
		{"", ""},
		{"  ", ""},
	}

	for _, tc := range tests {
		result := normalizeGenreName(tc.input)
		if result != tc.expected {
			t.Errorf("normalizeGenreName(%q) = %q, want %q", tc.input, result, tc.expected)
		}
	}
}

// --- helper ---

func assertWeight(t *testing.T, profile GenreProfile, genre string, expected float64) {
	t.Helper()
	w, ok := profile.GenreWeights[genre]
	if !ok {
		t.Errorf("genre %q not found in profile: %v", genre, profile.GenreWeights)
		return
	}
	if math.Abs(w-expected) > 1e-6 {
		t.Errorf("genre %q weight = %f, want %f", genre, w, expected)
	}
}
