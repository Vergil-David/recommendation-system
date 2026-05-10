package recommendations

import (
	"context"
	"fmt"
	"log"
	"math"

	"github.com/google/uuid"

	"recommendation-system/internal/repository"
)

const (
	// Weights for different interaction signals when building the profile vector.
	profileWeightLiked    = 1.0
	profileWeightFavorite = 2.0
)

// RebuildUserProfileVector recomputes the user's profile embedding as a
// weighted average of their liked (weight 1.0) and favorite (weight 2.0)
// item embeddings, then L2-normalizes and saves it to the users table.
//
// This should be called asynchronously after every liked/favorite interaction
// change so the profile stays up-to-date.
func RebuildUserProfileVector(ctx context.Context, userID uuid.UUID) error {
	itemEmbeddings, err := repository.GetUserPositiveItemEmbeddings(ctx, userID)
	if err != nil {
		return fmt.Errorf("get positive item embeddings: %w", err)
	}

	if len(itemEmbeddings) == 0 {
		if err := repository.ClearUserProfileEmbedding(ctx, userID); err != nil {
			log.Printf("⚠️ profile-vector: failed to clear for user %s: %v", userID, err)
		}
		log.Printf("ℹ️ profile-vector: cleared for user %s (no positive items)", userID)
		return nil
	}

	// Determine dimension from first embedding.
	dim := len(itemEmbeddings[0].Embedding)
	if dim == 0 {
		log.Printf("⚠️ profile-vector: first embedding has dim=0 for user %s", userID)
		return nil
	}

	profile := computeWeightedAverage(itemEmbeddings, dim)
	l2NormalizeFloat32(profile)

	if err := repository.SaveUserProfileEmbedding(ctx, userID, profile); err != nil {
		return fmt.Errorf("save profile embedding: %w", err)
	}

	log.Printf("ℹ️ profile-vector: rebuilt for user %s (sources=%d, dim=%d)", userID, len(itemEmbeddings), dim)
	return nil
}

// computeWeightedAverage produces a weighted centroid from item embeddings.
// liked items contribute weight 1.0, favorite items contribute weight 2.0.
func computeWeightedAverage(items []repository.UserItemEmbedding, dim int) []float32 {
	vector := make([]float64, dim)
	totalWeight := 0.0

	for _, item := range items {
		if len(item.Embedding) != dim {
			continue
		}

		weight := signalWeight(item.Signal)
		totalWeight += weight

		for i, v := range item.Embedding {
			vector[i] += float64(v) * weight
		}
	}

	result := make([]float32, dim)
	if totalWeight < 1e-12 {
		return result
	}

	for i, v := range vector {
		result[i] = float32(v / totalWeight)
	}
	return result
}

// signalWeight returns the weight multiplier for a given interaction signal.
func signalWeight(signal string) float64 {
	switch signal {
	case repository.InteractionTypeFavorite:
		return profileWeightFavorite
	default:
		return profileWeightLiked
	}
}

// l2NormalizeFloat32 normalizes a float32 vector to unit length in place.
func l2NormalizeFloat32(v []float32) {
	var norm float64
	for _, val := range v {
		norm += float64(val) * float64(val)
	}
	norm = math.Sqrt(norm)
	if norm < 1e-12 {
		return
	}
	for i := range v {
		v[i] = float32(float64(v[i]) / norm)
	}
}
