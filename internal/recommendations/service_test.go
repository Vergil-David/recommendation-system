package recommendations

import (
	"math"
	"testing"

	"recommendation-system/internal/models"
	"recommendation-system/internal/repository"
)

// --- similarityScore tests ---

func TestSimilarityScore_ZeroDistance(t *testing.T) {
	score := similarityScore(0)
	if score != 1.0 {
		t.Errorf("similarityScore(0) = %f, want 1.0", score)
	}
}

func TestSimilarityScore_PositiveDistance(t *testing.T) {
	tests := []struct {
		distance float64
		want     float64
	}{
		{1.0, 0.5},
		{3.0, 0.25},
		{9.0, 0.1},
	}

	for _, tt := range tests {
		got := similarityScore(tt.distance)
		if math.Abs(got-tt.want) > 1e-9 {
			t.Errorf("similarityScore(%f) = %f, want %f", tt.distance, got, tt.want)
		}
	}
}

func TestSimilarityScore_LargeDistance(t *testing.T) {
	score := similarityScore(1_000_000)
	if score <= 0 || score >= 0.001 {
		t.Errorf("similarityScore(1000000) = %f, want near 0", score)
	}
}

func TestSimilarityScore_AlwaysPositive(t *testing.T) {
	for d := 0.0; d <= 100; d += 0.5 {
		score := similarityScore(d)
		if score <= 0 || score > 1.0 {
			t.Errorf("similarityScore(%f) = %f, want in (0, 1]", d, score)
		}
	}
}

// --- combineScores tests ---

func TestCombineScores_Identity(t *testing.T) {
	// combining with 0 should not change score
	got := combineScores(0.5, 0)
	if math.Abs(got-0.5) > 1e-9 {
		t.Errorf("combineScores(0.5, 0) = %f, want 0.5", got)
	}
}

func TestCombineScores_Symmetric(t *testing.T) {
	a := combineScores(0.3, 0.7)
	b := combineScores(0.7, 0.3)
	if math.Abs(a-b) > 1e-9 {
		t.Errorf("combineScores is not symmetric: (%f != %f)", a, b)
	}
}

func TestCombineScores_NeverExceedsOne(t *testing.T) {
	score := combineScores(0.99, 0.99)
	if score > 1.0 {
		t.Errorf("combineScores(0.99, 0.99) = %f, want <= 1.0", score)
	}
}

func TestCombineScores_GrowsMonotonically(t *testing.T) {
	// Adding a positive score should always increase the combined value
	prev := 0.0
	for i := 0; i < 20; i++ {
		prev = combineScores(prev, 0.3)
	}
	if prev <= 0.3 {
		t.Errorf("20 rounds of combineScores(x, 0.3) = %f, want > 0.3", prev)
	}
	if prev > 1.0 {
		t.Errorf("20 rounds of combineScores(x, 0.3) = %f, want <= 1.0", prev)
	}
}

func TestCombineScores_ProbabilisticUnion(t *testing.T) {
	// Verify noisy-OR formula: 1 - (1-a)*(1-b)
	a, b := 0.4, 0.6
	want := 1 - (1-a)*(1-b)
	got := combineScores(a, b)
	if math.Abs(got-want) > 1e-9 {
		t.Errorf("combineScores(%f, %f) = %f, want %f", a, b, got, want)
	}
}

// --- roundScore tests ---

func TestRoundScore_Precision(t *testing.T) {
	tests := []struct {
		input float64
		want  float64
	}{
		{0.12345, 0.12},
		{0.999, 0.99},    // clamped to max 0.99
		{1.5, 0.99},      // clamped to max 0.99
		{-0.5, 0},         // clamped to min 0
		{0, 0},
		{0.505, 0.51},    // rounding
	}

	for _, tt := range tests {
		got := roundScore(tt.input)
		if math.Abs(got-tt.want) > 1e-9 {
			t.Errorf("roundScore(%f) = %f, want %f", tt.input, got, tt.want)
		}
	}
}

// --- normalizeInteractionType tests ---

func TestNormalizeInteractionType_ValidTypes(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		// New canonical types
		{"viewed", "viewed"},
		{"liked", "liked"},
		{"disliked", "disliked"},
		{"favorite", "favorite"},
		{"skipped", "skipped"},
		// Legacy types
		{"view", "viewed"},
		{"like", "liked"},
		{"skip", "skipped"},
	}

	for _, tt := range tests {
		got, err := normalizeInteractionType(tt.input)
		if err != nil {
			t.Errorf("normalizeInteractionType(%q) unexpected error: %v", tt.input, err)
			continue
		}
		if got != tt.want {
			t.Errorf("normalizeInteractionType(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestNormalizeInteractionType_Invalid(t *testing.T) {
	invalid := []string{"", "love", "block", "123"}

	for _, input := range invalid {
		_, err := normalizeInteractionType(input)
		if err == nil {
			t.Errorf("normalizeInteractionType(%q) expected error, got nil", input)
		}
	}
}

// --- buildRecommendationSources tests ---

func TestBuildRecommendationSources_DeduplicatesFavoritesAndLiked(t *testing.T) {
	sharedItem := models.Item{ID: 1, Title: "Shared"}
	onlyFavorite := models.Item{ID: 2, Title: "OnlyFav"}
	onlyLiked := models.Item{ID: 3, Title: "OnlyLiked"}

	favorites := []models.Item{sharedItem, onlyFavorite}
	liked := []models.Item{sharedItem, onlyLiked}

	sources := buildRecommendationSources(liked, favorites)

	if len(sources) != 3 {
		t.Fatalf("expected 3 sources, got %d", len(sources))
	}

	// Shared item should appear only once (from favorites, which come first)
	if sources[0].Item.ID != 1 || sources[0].Signal != repository.InteractionTypeFavorite {
		t.Errorf("sources[0] = {ID:%d, Signal:%s}, want {ID:1, Signal:favorite}", sources[0].Item.ID, sources[0].Signal)
	}
	if sources[1].Item.ID != 2 || sources[1].Signal != repository.InteractionTypeFavorite {
		t.Errorf("sources[1] = {ID:%d, Signal:%s}, want {ID:2, Signal:favorite}", sources[1].Item.ID, sources[1].Signal)
	}
	if sources[2].Item.ID != 3 || sources[2].Signal != repository.InteractionTypeLiked {
		t.Errorf("sources[2] = {ID:%d, Signal:%s}, want {ID:3, Signal:liked}", sources[2].Item.ID, sources[2].Signal)
	}
}

func TestBuildRecommendationSources_FavoritesFirst(t *testing.T) {
	favorites := []models.Item{{ID: 10, Title: "Fav"}}
	liked := []models.Item{{ID: 20, Title: "Liked"}}

	// Signature: buildRecommendationSources(likedItems, favoriteItems)
	// But favorites are processed first internally, so they appear first in output
	sources := buildRecommendationSources(liked, favorites)

	if len(sources) != 2 {
		t.Fatalf("expected 2 sources, got %d", len(sources))
	}
	if sources[0].Signal != repository.InteractionTypeFavorite {
		t.Errorf("favorites should come first in output, got signal=%s", sources[0].Signal)
	}
}

func TestBuildRecommendationSources_EmptyInputs(t *testing.T) {
	sources := buildRecommendationSources(nil, nil)
	if len(sources) != 0 {
		t.Errorf("expected 0 sources from nil inputs, got %d", len(sources))
	}
}

// --- isInteractionActive tests ---

func TestIsInteractionActive(t *testing.T) {
	state := repository.InteractionState{
		Viewed:   true,
		Liked:    false,
		Disliked: true,
		Favorite: false,
		Skipped:  true,
	}

	tests := []struct {
		interactionType string
		want            bool
	}{
		{"viewed", true},
		{"liked", false},
		{"disliked", true},
		{"favorite", false},
		{"skipped", true},
		{"unknown", false},
	}

	for _, tt := range tests {
		got := isInteractionActive(state, tt.interactionType)
		if got != tt.want {
			t.Errorf("isInteractionActive(state, %q) = %v, want %v", tt.interactionType, got, tt.want)
		}
	}
}

// --- positiveSignalLabel tests ---

func TestPositiveSignalLabel(t *testing.T) {
	if got := positiveSignalLabel("favorite"); got != "saved" {
		t.Errorf("positiveSignalLabel(favorite) = %q, want \"saved\"", got)
	}
	if got := positiveSignalLabel("liked"); got != "liked" {
		t.Errorf("positiveSignalLabel(liked) = %q, want \"liked\"", got)
	}
	if got := positiveSignalLabel("anything"); got != "liked" {
		t.Errorf("positiveSignalLabel(anything) = %q, want \"liked\"", got)
	}
}
