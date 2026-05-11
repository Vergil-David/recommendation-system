package recommendations

import (
	"log"
	"strings"
)

const (
	// defaultMaxPerGenre is the default cap on how many items from a single
	// dominant genre can appear in the diversified result list.
	// With a default limit of 20, this gives ≈5 genres representation.
	defaultMaxPerGenre = 4
)

// diversifyRecommendations re-ranks a scored recommendation list to ensure
// genre diversity. It uses a greedy approach: iterate in score order, accept
// each item only if its dominant genre hasn't exceeded the per-genre cap.
// Items that are skipped due to the cap are appended at the end so the total
// count is preserved.
//
// The function does NOT re-sort by score; position in the output encodes both
// relevance (original score order) and diversity.
func diversifyRecommendations(recs []Recommendation, maxPerGenre int) []Recommendation {
	if len(recs) == 0 {
		return recs
	}
	if maxPerGenre <= 0 {
		maxPerGenre = defaultMaxPerGenre
	}

	genreCounts := make(map[string]int)
	result := make([]Recommendation, 0, len(recs))
	deferred := make([]Recommendation, 0)

	for _, rec := range recs {
		dominant := getDominantGenre(rec.Item.Genres)
		if dominant == "" {
			// Items with no genre data are always accepted.
			result = append(result, rec)
			continue
		}

		if genreCounts[dominant] < maxPerGenre {
			result = append(result, rec)
			genreCounts[dominant]++
		} else {
			deferred = append(deferred, rec)
		}
	}

	// Append deferred items so the total count is preserved.
	result = append(result, deferred...)

	if len(deferred) > 0 {
		log.Printf("ℹ️ diversity: re-ranked %d items, deferred %d due to genre cap (max_per_genre=%d)",
			len(recs), len(deferred), maxPerGenre)
	}

	return result
}

// getDominantGenre returns the first (primary) genre of an item, normalized
// for consistent counting. Returns "" if there are no genres.
func getDominantGenre(genres []string) string {
	if len(genres) == 0 {
		return ""
	}

	trimmed := strings.TrimSpace(genres[0])
	if trimmed == "" && len(genres) > 1 {
		trimmed = strings.TrimSpace(genres[1])
	}

	return strings.ToLower(trimmed)
}
