package repository

import (
	"context"
	"errors"
	"fmt"

	"recommendation-system/internal/database"
	"recommendation-system/internal/models"

	"github.com/jackc/pgx/v5"
	"github.com/pgvector/pgvector-go"
)

var ErrItemNotFound = errors.New("item not found")

type SimilarItem struct {
	Item     models.Item
	Distance float64
}

type rowScanner interface {
	Scan(dest ...any) error
}

const (
	itemSelectProjection = `
		select
			i.id,
			i.title,
			coalesce(i.description, '') as description,
			coalesce(i.release_year, 0) as release_year,
			coalesce(i.image_url, '') as image_url,
			coalesce(array_remove(array_agg(distinct g.name), null), '{}'::text[]) as genres
	`
	itemSelectFromAndJoins = `
		from items i
		left join item_genres ig on ig.item_id = i.id
		left join genres g on g.id = ig.genre_id
	`
)

// FindSimilar preserves the original repository entrypoint for pgvector-based search.
func FindSimilar(userVector []float32) ([]models.Item, error) {
	return FindSimilarWithLimit(context.Background(), userVector, 5)
}

func FindSimilarWithLimit(ctx context.Context, userVector []float32, limit int) ([]models.Item, error) {
	if limit <= 0 {
		limit = 5
	}

	query := itemSelectProjection + itemSelectFromAndJoins + `
		where i.type = 'movie'
		  and i.embedding is not null
		group by i.id
		order by i.embedding <-> $1, i.created_at desc
		limit $2
	`

	rows, err := database.DB.Query(ctx, query, pgvector.NewVector(userVector), limit)
	if err != nil {
		return nil, fmt.Errorf("query similar items: %w", err)
	}
	defer rows.Close()

	items := make([]models.Item, 0, limit)
	for rows.Next() {
		item, err := scanItem(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if rows.Err() != nil {
		return nil, rows.Err()
	}

	return items, nil
}

func GetItemByID(ctx context.Context, itemID int64) (*models.Item, error) {
	query := itemSelectProjection + itemSelectFromAndJoins + `
		where i.id = $1
		group by i.id
	`

	item, err := scanItemRow(database.DB.QueryRow(ctx, query, itemID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrItemNotFound
		}
		return nil, err
	}

	return item, nil
}

func GetSimilarItemsByItemID(ctx context.Context, itemID int64, limit int, excludeIDs []int64) ([]SimilarItem, error) {
	if limit <= 0 {
		return []SimilarItem{}, nil
	}

	query := itemSelectProjection + `,
		(i.embedding <-> source.embedding) as distance
	from (
		select embedding
		from items
		where id = $1
		  and embedding is not null
	) source
	join items i on i.embedding is not null
	left join item_genres ig on ig.item_id = i.id
	left join genres g on g.id = ig.genre_id
	where i.type = 'movie'
	  and i.id <> $1
	  and not (i.id = any($2))
	group by i.id, source.embedding
	order by distance asc, i.created_at desc
	limit $3
`
	args := []any{itemID, excludeIDs, limit}

	rows, err := database.DB.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query similar items by item id: %w", err)
	}
	defer rows.Close()

	items := make([]SimilarItem, 0, limit)
	for rows.Next() {
		item, distance, err := scanSimilarItem(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, SimilarItem{
			Item:     item,
			Distance: distance,
		})
	}
	if rows.Err() != nil {
		return nil, rows.Err()
	}

	return items, nil
}

func GetPopularOrNewestMovies(ctx context.Context, limit int, excludeIDs []int64) ([]models.Item, error) {
	if limit <= 0 {
		return []models.Item{}, nil
	}

	query := itemSelectProjection + itemSelectFromAndJoins + `
		where i.type = 'movie'
		  and not (i.id = any($1))
		group by i.id
		order by
			coalesce(nullif(i.metadata->>'popularity', '')::double precision, 0) desc,
			coalesce(i.release_year, 0) desc,
			i.created_at desc
		limit $2
`
	args := []any{excludeIDs, limit}

	rows, err := database.DB.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query popular or newest movies: %w", err)
	}
	defer rows.Close()

	items := make([]models.Item, 0, limit)
	for rows.Next() {
		item, err := scanItem(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if rows.Err() != nil {
		return nil, rows.Err()
	}

	return items, nil
}

// GetTrendingMovies returns movies ranked by total unique user interaction
// count in the last N days. This solves cold-start by showing what's currently
// hot among all users.
func GetTrendingMovies(ctx context.Context, limit int, days int, excludeIDs []int64) ([]models.Item, error) {
	if limit <= 0 {
		return []models.Item{}, nil
	}
	if days <= 0 {
		days = 30
	}

	query := itemSelectProjection + `,
		count(distinct intr.user_id) as engagement_count
	` + itemSelectFromAndJoins + `
		join interactions intr on intr.item_id = i.id
		where i.type = 'movie'
		  and intr.updated_at >= now() - ($1 || ' days')::interval
		  and not (i.id = any($2))
		group by i.id
		order by engagement_count desc, coalesce(i.release_year, 0) desc
		limit $3
`
	args := []any{days, excludeIDs, limit}

	rows, err := database.DB.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query trending movies: %w", err)
	}
	defer rows.Close()

	items := make([]models.Item, 0, limit)
	for rows.Next() {
		var item models.Item
		var engagementCount int
		if err := rows.Scan(
			&item.ID,
			&item.Title,
			&item.Description,
			&item.ReleaseYear,
			&item.ImageURL,
			&item.Genres,
			&engagementCount,
		); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if rows.Err() != nil {
		return nil, rows.Err()
	}

	return items, nil
}

func scanItem(row rowScanner) (models.Item, error) {
	item, err := scanItemRow(row)
	if err != nil {
		return models.Item{}, err
	}
	return *item, nil
}

func scanItemRow(row rowScanner) (*models.Item, error) {
	var item models.Item
	if err := row.Scan(
		&item.ID,
		&item.Title,
		&item.Description,
		&item.ReleaseYear,
		&item.ImageURL,
		&item.Genres,
	); err != nil {
		return nil, err
	}
	return &item, nil
}

func scanSimilarItem(row rowScanner) (models.Item, float64, error) {
	var item models.Item
	var distance float64
	if err := row.Scan(
		&item.ID,
		&item.Title,
		&item.Description,
		&item.ReleaseYear,
		&item.ImageURL,
		&item.Genres,
		&distance,
	); err != nil {
		return models.Item{}, 0, err
	}
	return item, distance, nil
}
