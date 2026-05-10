package recommendations

import (
	"math"
	"testing"

	"recommendation-system/internal/models"
	"recommendation-system/internal/repository"
)

// --- cosineSimilarity tests ---

func TestCosineSimilarity_Identical(t *testing.T) {
	a := []float32{1, 0, 0}
	b := []float32{1, 0, 0}
	sim := cosineSimilarity(a, b)
	if math.Abs(sim-1.0) > 1e-6 {
		t.Errorf("cosineSimilarity(identical) = %f, want 1.0", sim)
	}
}

func TestCosineSimilarity_Orthogonal(t *testing.T) {
	a := []float32{1, 0, 0}
	b := []float32{0, 1, 0}
	sim := cosineSimilarity(a, b)
	if math.Abs(sim) > 1e-6 {
		t.Errorf("cosineSimilarity(orthogonal) = %f, want 0.0", sim)
	}
}

func TestCosineSimilarity_Opposite(t *testing.T) {
	a := []float32{1, 0, 0}
	b := []float32{-1, 0, 0}
	sim := cosineSimilarity(a, b)
	if math.Abs(sim+1.0) > 1e-6 {
		t.Errorf("cosineSimilarity(opposite) = %f, want -1.0", sim)
	}
}

func TestCosineSimilarity_Empty(t *testing.T) {
	sim := cosineSimilarity(nil, nil)
	if sim != 0 {
		t.Errorf("cosineSimilarity(nil,nil) = %f, want 0", sim)
	}
}

func TestCosineSimilarity_DimensionMismatch(t *testing.T) {
	a := []float32{1, 0}
	b := []float32{1, 0, 0}
	sim := cosineSimilarity(a, b)
	if sim != 0 {
		t.Errorf("cosineSimilarity(dim mismatch) = %f, want 0", sim)
	}
}

func TestCosineSimilarity_ZeroVector(t *testing.T) {
	a := []float32{0, 0, 0}
	b := []float32{1, 0, 0}
	sim := cosineSimilarity(a, b)
	if sim != 0 {
		t.Errorf("cosineSimilarity(zero, non-zero) = %f, want 0", sim)
	}
}

func TestCosineSimilarity_PartialOverlap(t *testing.T) {
	// 45-degree angle: cos(45°) ≈ 0.7071
	a := []float32{1, 0}
	b := []float32{1, 1}
	sim := cosineSimilarity(a, b)
	expected := 1.0 / math.Sqrt(2)
	if math.Abs(sim-expected) > 1e-5 {
		t.Errorf("cosineSimilarity(45deg) = %f, want %f", sim, expected)
	}
}

// --- buildNegativeCentroid tests ---

func TestBuildNegativeCentroid_Single(t *testing.T) {
	negatives := []repository.NegativeItemEmbedding{
		{ItemID: 1, Embedding: []float32{3, 4, 0}},
	}
	centroid := buildNegativeCentroid(negatives)
	if centroid == nil {
		t.Fatal("centroid should not be nil")
	}
	// Should be L2 normalized: [3/5, 4/5, 0]
	if math.Abs(float64(centroid[0])-0.6) > 1e-5 {
		t.Errorf("centroid[0] = %f, want 0.6", centroid[0])
	}
	if math.Abs(float64(centroid[1])-0.8) > 1e-5 {
		t.Errorf("centroid[1] = %f, want 0.8", centroid[1])
	}
}

func TestBuildNegativeCentroid_Multiple(t *testing.T) {
	negatives := []repository.NegativeItemEmbedding{
		{ItemID: 1, Embedding: []float32{1, 0}},
		{ItemID: 2, Embedding: []float32{0, 1}},
	}
	centroid := buildNegativeCentroid(negatives)
	if centroid == nil {
		t.Fatal("centroid should not be nil")
	}
	// Average = [0.5, 0.5], normalized = [1/√2, 1/√2]
	expected := float32(1.0 / math.Sqrt(2))
	if math.Abs(float64(centroid[0]-expected)) > 1e-5 {
		t.Errorf("centroid[0] = %f, want %f", centroid[0], expected)
	}
}

func TestBuildNegativeCentroid_Empty(t *testing.T) {
	centroid := buildNegativeCentroid(nil)
	if centroid != nil {
		t.Errorf("expected nil centroid for empty input, got %v", centroid)
	}
}

func TestBuildNegativeCentroid_SkipsDimensionMismatch(t *testing.T) {
	negatives := []repository.NegativeItemEmbedding{
		{ItemID: 1, Embedding: []float32{1, 0, 0}},
		{ItemID: 2, Embedding: []float32{0, 1}}, // wrong dim
	}
	centroid := buildNegativeCentroid(negatives)
	if centroid == nil {
		t.Fatal("centroid should not be nil")
	}
	// Only first item contributes
	if math.Abs(float64(centroid[0])-1.0) > 1e-5 {
		t.Errorf("centroid[0] = %f, want 1.0 (only valid item)", centroid[0])
	}
}

// --- applyNegativePenalty tests ---

func TestApplyNegativePenalty_PenalizesHighSimilarity(t *testing.T) {
	recs := []Recommendation{
		{Item: models.Item{ID: 1}, Score: 0.90},
		{Item: models.Item{ID: 2}, Score: 0.80},
	}

	// Candidate 1 has an embedding very similar to the negative centroid.
	// Candidate 2 has a different embedding.
	candidateEmb := map[int64][]float32{
		1: {1, 0, 0}, // identical direction to centroid
		2: {0, 1, 0}, // orthogonal to centroid
	}
	negativeCentroid := []float32{1, 0, 0}

	applyNegativePenalty(recs, candidateEmb, negativeCentroid)

	// Candidate 1 should be penalized (sim=1.0 > threshold=0.65)
	// penalty = 1 - 1.0*0.5 = 0.5 → score = 0.90 * 0.5 = 0.45
	if recs[0].Score >= 0.90 {
		t.Errorf("expected candidate 1 to be penalized, score=%f", recs[0].Score)
	}

	// After re-sort, candidate 2 (score 0.80, not penalized) should be first.
	if recs[0].Item.ID != 2 {
		t.Errorf("expected candidate 2 to be ranked first after penalty, got ID=%d", recs[0].Item.ID)
	}
}

func TestApplyNegativePenalty_NoEffectBelowThreshold(t *testing.T) {
	recs := []Recommendation{
		{Item: models.Item{ID: 1}, Score: 0.90},
	}

	candidateEmb := map[int64][]float32{
		1: {0, 1, 0}, // orthogonal → sim=0 < threshold
	}
	negativeCentroid := []float32{1, 0, 0}

	applyNegativePenalty(recs, candidateEmb, negativeCentroid)

	if recs[0].Score != 0.90 {
		t.Errorf("score should be unchanged when sim < threshold, got %f", recs[0].Score)
	}
}

func TestApplyNegativePenalty_NilCentroid(t *testing.T) {
	recs := []Recommendation{
		{Item: models.Item{ID: 1}, Score: 0.90},
	}

	applyNegativePenalty(recs, map[int64][]float32{1: {1, 0}}, nil)

	if recs[0].Score != 0.90 {
		t.Errorf("score should be unchanged with nil centroid, got %f", recs[0].Score)
	}
}

func TestApplyNegativePenalty_EmptyRecommendations(t *testing.T) {
	// Should not panic.
	applyNegativePenalty(nil, map[int64][]float32{}, []float32{1, 0})
}

func TestApplyNegativePenalty_MissingEmbeddingSkipped(t *testing.T) {
	recs := []Recommendation{
		{Item: models.Item{ID: 1}, Score: 0.90},
		{Item: models.Item{ID: 2}, Score: 0.80},
	}

	// Only candidate 1 has an embedding; candidate 2 is missing.
	candidateEmb := map[int64][]float32{
		1: {1, 0, 0},
	}
	negativeCentroid := []float32{1, 0, 0}

	applyNegativePenalty(recs, candidateEmb, negativeCentroid)

	// Candidate 2 should keep its original score.
	var candidate2Score float64
	for _, rec := range recs {
		if rec.Item.ID == 2 {
			candidate2Score = rec.Score
		}
	}
	if candidate2Score != 0.80 {
		t.Errorf("candidate 2 score should be unchanged (no embedding), got %f", candidate2Score)
	}
}

func TestApplyNegativePenalty_FloorAt10Percent(t *testing.T) {
	recs := []Recommendation{
		{Item: models.Item{ID: 1}, Score: 0.50},
	}

	candidateEmb := map[int64][]float32{
		1: {1, 0, 0},
	}
	negativeCentroid := []float32{1, 0, 0} // sim = 1.0

	applyNegativePenalty(recs, candidateEmb, negativeCentroid)

	// penalty = 1 - 1.0*0.5 = 0.5 → score = 0.50 * 0.5 = 0.25
	// Score should NOT go below 10% of original (0.05), but since 0.25 > 0.05 it stays.
	if recs[0].Score < 0.10 {
		t.Errorf("score should not go below floor, got %f", recs[0].Score)
	}
}
