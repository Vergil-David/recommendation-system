package repository

import (
	"context"
	"strings"

	"recommendation-system/internal/database"
	"recommendation-system/internal/models"
)

type SearchResult struct {
	Item  models.Item
	Type  string
	Score float64
}

// SearchItems does fuzzy multi-word search using pg_trgm word_similarity.
// itemType filters by "movie" or "book"; empty string returns both.
func SearchItems(ctx context.Context, query string, itemType string, limit int) ([]SearchResult, error) {
	query = strings.TrimSpace(query)
	if query == "" || limit <= 0 {
		return nil, nil
	}
	if limit > 50 {
		limit = 50
	}

	// word_similarity finds the best match of ANY word/phrase in query against target,
	// which makes it work for multi-word queries and tolerates typos naturally.
	sql := `
		SELECT
			i.id,
			i.title,
			COALESCE(i.description, '')   AS description,
			COALESCE(i.release_year, 0)   AS release_year,
			COALESCE(i.image_url, '')     AS image_url,
			i.type,
			COALESCE(array_remove(array_agg(DISTINCT g.name), NULL), '{}') AS genres,
			GREATEST(
				word_similarity($1, i.title),
				CASE WHEN i.title ILIKE '%' || $1 || '%' THEN 0.9 ELSE 0 END,
				CASE WHEN i.description ILIKE '%' || $1 || '%' THEN 0.4 ELSE 0 END
			) AS score
		FROM items i
		LEFT JOIN item_genres ig ON ig.item_id = i.id
		LEFT JOIN genres g       ON g.id = ig.genre_id
		WHERE (
			word_similarity($1, i.title) > 0.15
			OR i.title       ILIKE '%' || $1 || '%'
			OR i.description ILIKE '%' || $1 || '%'
			OR EXISTS (
				SELECT 1
				FROM item_genres ig2
				JOIN genres g2 ON g2.id = ig2.genre_id
				WHERE ig2.item_id = i.id
				  AND (g2.name ILIKE '%' || $1 || '%'
				       OR word_similarity($1, g2.name) > 0.3)
			)
		)
		AND ($2::text = '' OR i.type = $2)
		GROUP BY i.id, i.title, i.description, i.release_year, i.image_url, i.type
		ORDER BY score DESC, COALESCE(i.release_year, 0) DESC
		LIMIT $3
	`

	rows, err := database.DB.Query(ctx, sql, query, itemType, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []SearchResult
	for rows.Next() {
		var (
			item  models.Item
			itype string
			score float64
		)
		if err := rows.Scan(
			&item.ID,
			&item.Title,
			&item.Description,
			&item.ReleaseYear,
			&item.ImageURL,
			&itype,
			&item.Genres,
			&score,
		); err != nil {
			return nil, err
		}
		results = append(results, SearchResult{Item: item, Type: itype, Score: score})
	}
	return results, rows.Err()
}
