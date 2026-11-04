package recommendations

import (
	"log"
	"math"
	"sort"

	"recommendation-system/internal/repository"
)

const (
	// negativeSimilarityThreshold is the minimum cosine similarity between a
	// candidate and the negative centroid for the penalty to activate.
	// Below this the candidate is considered sufficiently different.
	negativeSimilarityThreshold = 0.65

	// negativePenaltyStrength controls how aggressively we penalize.
	// penalty_factor = 1 - (similarity × strength)
	// With strength=0.5 and similarity=1.0 the score is halved.
	negativePenaltyStrength = 0.5

	// maxNegativeEmbeddings caps how many negative items we use to build
	// the centroid (most recent first) to keep latency bounded.
	maxNegativeEmbeddings = 30
)

// applyNegativePenalty reduces the score of recommendation candidates that are
// semantically close to the centroid of items the user has disliked or skipped.
//
// candidateEmbeddings maps item_id → embedding for candidates whose embeddings
// were loaded from the DB.  Candidates without an embedding entry are left
// untouched.
//
// The slice is modified in-place and re-sorted by penalized score.
func applyNegativePenalty(
	recommendations []Recommendation,
	candidateEmbeddings map[int64][]float32,
	negativeCentroid []float32,
) {
	if len(recommendations) == 0 || negativeCentroid == nil || len(candidateEmbeddings) == 0 {
		return
	}

	penalizedCount := 0
	for i := range recommendations {
		emb, ok := candidateEmbeddings[recommendations[i].Item.ID]
		if !ok || len(emb) == 0 {
			continue
		}

		sim := cosineSimilarity(emb, negativeCentroid)
		if sim < negativeSimilarityThreshold {
			continue
		}

		penalty := 1.0 - sim*negativePenaltyStrength
		if penalty < 0.1 {
			penalty = 0.1 // never reduce score below 10% of original
		}

		recommendations[i].Score = roundScore(recommendations[i].Score * penalty)
		penalizedCount++
	}

	if penalizedCount > 0 {
		// Re-sort after penalties shifted scores.
		sort.Slice(recommendations, func(i, j int) bool {
			if recommendations[i].Score == recommendations[j].Score {
				return recommendations[i].Item.ID > recommendations[j].Item.ID
			}
			return recommendations[i].Score > recommendations[j].Score
		})
		log.Printf("ℹ️ negative penalty: penalized %d/%d candidates", penalizedCount, len(recommendations))
	}
}

// buildNegativeCentroid computes the L2-normalized centroid of all negative
// item embeddings.  This single vector represents "what the user dislikes".
func buildNegativeCentroid(negatives []repository.NegativeItemEmbedding) []float32 {
	if len(negatives) == 0 {
		return nil
	}

	// Cap to most recent N.
	if len(negatives) > maxNegativeEmbeddings {
		negatives = negatives[:maxNegativeEmbeddings]
	}

	dim := len(negatives[0].Embedding)
	if dim == 0 {
		return nil
	}

	sum := make([]float64, dim)
	count := 0
	for _, neg := range negatives {
		if len(neg.Embedding) != dim {
			continue
		}
		count++
		for i, v := range neg.Embedding {
			sum[i] += float64(v)
		}
	}

	if count == 0 {
		return nil
	}

	centroid := make([]float32, dim)
	for i := range sum {
		centroid[i] = float32(sum[i] / float64(count))
	}
	l2NormalizeFloat32(centroid)
	return centroid
}

// cosineSimilarity computes the cosine similarity between two float32 vectors.
// Returns a value in [-1, 1].  Higher means more similar.
func cosineSimilarity(a, b []float32) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}

	var dot, normA, normB float64
	for i := range a {
		ai, bi := float64(a[i]), float64(b[i])
		dot += ai * bi
		normA += ai * ai
		normB += bi * bi
	}

	denom := math.Sqrt(normA) * math.Sqrt(normB)
	if denom < 1e-12 {
		return 0
	}
	return dot / denom
}
