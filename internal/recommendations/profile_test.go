package recommendations

import (
	"math"
	"testing"

	"recommendation-system/internal/repository"
)

// --- computeWeightedAverage tests ---

func TestComputeWeightedAverage_SingleLiked(t *testing.T) {
	items := []repository.UserItemEmbedding{
		{ItemID: 1, Embedding: []float32{1.0, 0.0, 0.0}, Signal: "liked"},
	}

	result := computeWeightedAverage(items, 3)

	if len(result) != 3 {
		t.Fatalf("expected dim=3, got %d", len(result))
	}
	if result[0] != 1.0 || result[1] != 0.0 || result[2] != 0.0 {
		t.Errorf("expected [1,0,0], got %v", result)
	}
}

func TestComputeWeightedAverage_LikedAndFavorite(t *testing.T) {
	items := []repository.UserItemEmbedding{
		{ItemID: 1, Embedding: []float32{1.0, 0.0}, Signal: "liked"},    // weight 1.0
		{ItemID: 2, Embedding: []float32{0.0, 1.0}, Signal: "favorite"}, // weight 2.0
	}

	result := computeWeightedAverage(items, 2)

	// weighted avg = (1*1.0 + 0*2.0) / 3.0, (0*1.0 + 1*2.0) / 3.0
	// = 1/3, 2/3
	expectedX := float32(1.0 / 3.0)
	expectedY := float32(2.0 / 3.0)

	if math.Abs(float64(result[0]-expectedX)) > 1e-5 {
		t.Errorf("result[0] = %f, want %f", result[0], expectedX)
	}
	if math.Abs(float64(result[1]-expectedY)) > 1e-5 {
		t.Errorf("result[1] = %f, want %f", result[1], expectedY)
	}
}

func TestComputeWeightedAverage_FavoriteWeightedHigher(t *testing.T) {
	// Two identical embeddings but one is favorite — the favorite should
	// dominate the average.
	items := []repository.UserItemEmbedding{
		{ItemID: 1, Embedding: []float32{1.0, 0.0}, Signal: "liked"},
		{ItemID: 2, Embedding: []float32{0.0, 1.0}, Signal: "favorite"},
	}

	result := computeWeightedAverage(items, 2)

	// favorite has 2× weight, so the Y component should be larger than X.
	if result[1] <= result[0] {
		t.Errorf("expected Y > X (favorite dominance), got X=%f Y=%f", result[0], result[1])
	}
}

func TestComputeWeightedAverage_Empty(t *testing.T) {
	result := computeWeightedAverage(nil, 3)
	if len(result) != 3 {
		t.Fatalf("expected dim=3, got %d", len(result))
	}
	for i, v := range result {
		if v != 0 {
			t.Errorf("result[%d] = %f, want 0", i, v)
		}
	}
}

func TestComputeWeightedAverage_MismatchedDimSkipped(t *testing.T) {
	items := []repository.UserItemEmbedding{
		{ItemID: 1, Embedding: []float32{1.0, 0.0, 0.0}, Signal: "liked"},
		{ItemID: 2, Embedding: []float32{0.0, 1.0}, Signal: "liked"}, // wrong dim, should be skipped
	}

	result := computeWeightedAverage(items, 3)

	// Only item 1 should contribute
	if result[0] != 1.0 {
		t.Errorf("expected result[0]=1.0 (only valid item), got %f", result[0])
	}
}

// --- l2NormalizeFloat32 tests ---

func TestL2NormalizeFloat32_UnitVector(t *testing.T) {
	v := []float32{1.0, 0.0, 0.0}
	l2NormalizeFloat32(v)
	if v[0] != 1.0 || v[1] != 0.0 || v[2] != 0.0 {
		t.Errorf("unit vector should stay unchanged, got %v", v)
	}
}

func TestL2NormalizeFloat32_Normalization(t *testing.T) {
	v := []float32{3.0, 4.0}
	l2NormalizeFloat32(v)

	// norm should be 1.0
	var norm float64
	for _, val := range v {
		norm += float64(val) * float64(val)
	}
	norm = math.Sqrt(norm)

	if math.Abs(norm-1.0) > 1e-5 {
		t.Errorf("norm after normalize = %f, want 1.0", norm)
	}
}

func TestL2NormalizeFloat32_ZeroVector(t *testing.T) {
	v := []float32{0.0, 0.0, 0.0}
	l2NormalizeFloat32(v)
	// Should not panic and remain zero
	for i, val := range v {
		if val != 0 {
			t.Errorf("zero vector component [%d] = %f, want 0", i, val)
		}
	}
}

// --- signalWeight tests ---

func TestSignalWeight_Favorite(t *testing.T) {
	if got := signalWeight("favorite"); got != profileWeightFavorite {
		t.Errorf("signalWeight(favorite) = %f, want %f", got, profileWeightFavorite)
	}
}

func TestSignalWeight_Liked(t *testing.T) {
	if got := signalWeight("liked"); got != profileWeightLiked {
		t.Errorf("signalWeight(liked) = %f, want %f", got, profileWeightLiked)
	}
}

func TestSignalWeight_Unknown(t *testing.T) {
	if got := signalWeight("viewed"); got != profileWeightLiked {
		t.Errorf("signalWeight(viewed) = %f, want %f (default)", got, profileWeightLiked)
	}
}
