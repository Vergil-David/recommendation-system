package repository

import (
	"context"
	"fmt"

	"github.com/pgvector/pgvector-go"

	"recommendation-system/internal/database"
)

// ItemEmbeddingData holds the text fields needed to generate an embedding.
type ItemEmbeddingData struct {
	ID          int64
	Title       string
	Description string
	Genres      []string
}

// GetAllItemsForEmbedding fetches all movie items with their text data and genres.
func GetAllItemsForEmbedding(ctx context.Context) ([]ItemEmbeddingData, error) {
	query := `
		select
			i.id,
			coalesce(i.title, ''),
			coalesce(i.description, ''),
			coalesce(array_remove(array_agg(distinct g.name), null), '{}'::text[])
		from items i
		left join item_genres ig on ig.item_id = i.id
		left join genres g on g.id = ig.genre_id
		where i.type = 'movie'
		group by i.id
		order by i.id
	`

	rows, err := database.DB.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query items for embedding: %w", err)
	}
	defer rows.Close()

	items := make([]ItemEmbeddingData, 0)
	for rows.Next() {
		var item ItemEmbeddingData
		if err := rows.Scan(&item.ID, &item.Title, &item.Description, &item.Genres); err != nil {
			return nil, fmt.Errorf("scan item for embedding: %w", err)
		}
		items = append(items, item)
	}
	if rows.Err() != nil {
		return nil, rows.Err()
	}

	return items, nil
}

// UpdateItemEmbedding writes a computed embedding vector into the items table.
func UpdateItemEmbedding(ctx context.Context, itemID int64, embedding []float32) error {
	query := `update items set embedding = $1 where id = $2`
	_, err := database.DB.Exec(ctx, query, pgvector.NewVector(embedding), itemID)
	if err != nil {
		return fmt.Errorf("update embedding for item %d: %w", itemID, err)
	}
	return nil
}

// CountItemsWithEmbedding returns (total movies, movies with embedding).
func CountItemsWithEmbedding(ctx context.Context) (total int, withEmbedding int, err error) {
	query := `
		select
			count(*),
			count(*) filter (where embedding is not null)
		from items
		where type = 'movie'
	`
	if err := database.DB.QueryRow(ctx, query).Scan(&total, &withEmbedding); err != nil {
		return 0, 0, fmt.Errorf("count items with embedding: %w", err)
	}
	return total, withEmbedding, nil
}
