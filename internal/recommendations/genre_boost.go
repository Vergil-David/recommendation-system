package recommendations

import (
	"log"
	"sort"
	"strings"

	"recommendation-system/internal/models"
)

const (
	// genreWeightLiked is the weight given to genres from liked items.
	genreWeightLiked = 1.0
	// genreWeightFavorite is the weight given to genres from favorited items.
	genreWeightFavorite = 2.0

	// genreBoostStrength controls how much the genre profile can boost a score.
	// With strength=0.3 and a max genre weight sum of 1.0, the maximum boost
	// to a candidate score is 30%.
	genreBoostStrength = 0.3
)

// GenreProfile captures a user's genre preferences as normalized weights.
// The weights sum to 1.0 so they can be used as a direct multiplier.
type GenreProfile struct {
	GenreWeights map[string]float64 // normalized genre name → weight [0, 1]
}

// IsEmpty reports whether the genre profile has any genre data.
func (gp GenreProfile) IsEmpty() bool {
	return len(gp.GenreWeights) == 0
}

// BuildGenreProfile computes a genre frequency profile from the user's liked
// and favorited items. Favorites are weighted 2× compared to likes.
//
// The resulting GenreWeights are normalized so they sum to 1.0.
func BuildGenreProfile(likedItems, favoritedItems []models.Item) GenreProfile {
	rawWeights := make(map[string]float64)

	addGenres := func(items []models.Item, weight float64) {
		for _, item := range items {
			for _, genre := range item.Genres {
				normalized := normalizeGenreName(genre)
				if normalized == "" {
					continue
				}
				rawWeights[normalized] += weight
			}
		}
	}

	addGenres(likedItems, genreWeightLiked)
	addGenres(favoritedItems, genreWeightFavorite)

	if len(rawWeights) == 0 {
		return GenreProfile{GenreWeights: map[string]float64{}}
	}

	// Normalize so weights sum to 1.0.
	totalWeight := 0.0
	for _, w := range rawWeights {
		totalWeight += w
	}

	normalized := make(map[string]float64, len(rawWeights))
	for genre, w := range rawWeights {
		normalized[genre] = w / totalWeight
	}

	return GenreProfile{GenreWeights: normalized}
}

// ApplyGenreBoost increases the score of recommendation candidates whose genres
// overlap with the user's genre profile. The boost is multiplicative:
//
//	boosted_score = score × (1 + genreOverlap × genreBoostStrength)
//
// where genreOverlap is the sum of the user's normalized genre weights for the
// candidate's genres (capped at 1.0).
//
// The function modifies the slice in-place and re-sorts by score.
func ApplyGenreBoost(recommendations []Recommendation, profile GenreProfile) {
	if len(recommendations) == 0 || profile.IsEmpty() {
		return
	}

	boostedCount := 0
	for i := range recommendations {
		overlap := genreOverlap(recommendations[i].Item.Genres, profile.GenreWeights)
		if overlap <= 0 {
			continue
		}

		// Cap overlap at 1.0 so boost is bounded.
		if overlap > 1.0 {
			overlap = 1.0
		}

		boost := 1.0 + overlap*genreBoostStrength
		recommendations[i].Score = roundScore(recommendations[i].Score * boost)
		boostedCount++
	}

	if boostedCount > 0 {
		// Re-sort after boost shifted scores.
		sort.Slice(recommendations, func(i, j int) bool {
			if recommendations[i].Score == recommendations[j].Score {
				return recommendations[i].Item.ID > recommendations[j].Item.ID
			}
			return recommendations[i].Score > recommendations[j].Score
		})
		log.Printf("ℹ️ genre boost: boosted %d/%d candidates", boostedCount, len(recommendations))
	}
}

// genreOverlap calculates the sum of profile weights for genres that appear in
// the candidate item.
func genreOverlap(itemGenres []string, profileWeights map[string]float64) float64 {
	if len(itemGenres) == 0 || len(profileWeights) == 0 {
		return 0
	}

	overlap := 0.0
	for _, genre := range itemGenres {
		normalized := normalizeGenreName(genre)
		if w, ok := profileWeights[normalized]; ok {
			overlap += w
		}
	}
	return overlap
}

// normalizeGenreName lowercases and trims a genre name for consistent matching.
func normalizeGenreName(genre string) string {
	return strings.ToLower(strings.TrimSpace(genre))
}
