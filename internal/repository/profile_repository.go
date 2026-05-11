package repository

import (
	"context"
	"fmt"
	"log"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/pgvector/pgvector-go"

	"recommendation-system/internal/database"
	"recommendation-system/internal/models"
)

// UserItemEmbedding holds an item embedding together with the interaction signal
// so the caller can apply per-signal weights (e.g. favorite = 2×, liked = 1×).
type UserItemEmbedding struct {
	ItemID    int64
	Embedding []float32
	Signal    string // InteractionTypeLiked or InteractionTypeFavorite
}

// GetUserPositiveItemEmbeddings returns embeddings for items that the user
// liked or favorited.  Each row carries the interaction signal so the caller
// can weight them differently.
func GetUserPositiveItemEmbeddings(ctx context.Context, userID uuid.UUID) ([]UserItemEmbedding, error) {
	query := `
		select
			i.id,
			i.embedding,
			case
				when coalesce(x.favorite, false) then 'favorite'
				else 'liked'
			end as signal
		from interactions x
		join items i on i.id = x.item_id
		where x.user_id = $1
		  and i.embedding is not null
		  and (
			coalesce(x.liked, false) = true
			or coalesce(x.favorite, false) = true
		  )
		order by x.updated_at desc
	`

	rows, err := database.DB.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("query user positive item embeddings: %w", err)
	}
	defer rows.Close()

	result := make([]UserItemEmbedding, 0)
	for rows.Next() {
		var itemID int64
		var emb pgvector.Vector
		var signal string
		if err := rows.Scan(&itemID, &emb, &signal); err != nil {
			return nil, fmt.Errorf("scan user item embedding: %w", err)
		}
		result = append(result, UserItemEmbedding{
			ItemID:    itemID,
			Embedding: emb.Slice(),
			Signal:    signal,
		})
	}
	if rows.Err() != nil {
		return nil, rows.Err()
	}

	return result, nil
}

// SaveUserProfileEmbedding writes the computed profile embedding vector into the users table.
func SaveUserProfileEmbedding(ctx context.Context, userID uuid.UUID, embedding []float32) error {
	query := `update users set profile_embedding = $1 where id = $2`
	_, err := database.DB.Exec(ctx, query, pgvector.NewVector(embedding), userID)
	if err != nil {
		return fmt.Errorf("save user profile embedding: %w", err)
	}
	return nil
}

// ClearUserProfileEmbedding sets profile_embedding to NULL (when the user has
// no more liked/favorite items).
func ClearUserProfileEmbedding(ctx context.Context, userID uuid.UUID) error {
	query := `update users set profile_embedding = null where id = $1`
	_, err := database.DB.Exec(ctx, query, userID)
	if err != nil {
		return fmt.Errorf("clear user profile embedding: %w", err)
	}
	return nil
}

// GetUserProfileEmbedding reads the precomputed profile vector.
// Returns nil slice when the user has no profile vector yet.
func GetUserProfileEmbedding(ctx context.Context, userID uuid.UUID) ([]float32, error) {
	query := `select profile_embedding from users where id = $1`
	var emb *pgvector.Vector
	if err := database.DB.QueryRow(ctx, query, userID).Scan(&emb); err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get user profile embedding: %w", err)
	}
	if emb == nil {
		return nil, nil
	}
	return emb.Slice(), nil
}

// FindSimilarByProfileVector performs a single pgvector nearest-neighbor query
// using the user's precomputed profile embedding.
func FindSimilarByProfileVector(ctx context.Context, profileVector []float32, limit int, excludeIDs []int64) ([]SimilarItem, error) {
	if limit <= 0 {
		return []SimilarItem{}, nil
	}

	query := itemSelectProjection + `,
		(i.embedding <-> $1) as distance
	` + itemSelectFromAndJoins + `
		where i.type = 'movie'
		  and i.embedding is not null
		  and not (i.id = any($2))
		group by i.id
		order by distance asc, i.created_at desc
		limit $3
	`
	args := []any{pgvector.NewVector(profileVector), excludeIDs, limit}

	rows, err := database.DB.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query similar by profile vector: %w", err)
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

	log.Printf("ℹ️ profile-vector search: returned %d candidates (limit=%d, excluded=%d)", len(items), limit, len(excludeIDs))
	return items, nil
}

// GetItemEmbeddingByID returns the raw embedding for a single item.
// Returns nil if the item has no embedding.
func GetItemEmbeddingByID(ctx context.Context, itemID int64) ([]float32, error) {
	query := `select embedding from items where id = $1 and embedding is not null`
	var emb pgvector.Vector
	if err := database.DB.QueryRow(ctx, query, itemID).Scan(&emb); err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get item embedding: %w", err)
	}
	return emb.Slice(), nil
}

// CountUserPositiveItems returns how many items the user has liked or favorited
// (for fast checks without loading full embeddings).
func CountUserPositiveItems(ctx context.Context, userID uuid.UUID) (int, error) {
	query := `
		select count(*)
		from interactions
		where user_id = $1
		  and (coalesce(liked, false) = true or coalesce(favorite, false) = true)
	`
	var count int
	if err := database.DB.QueryRow(ctx, query, userID).Scan(&count); err != nil {
		return 0, fmt.Errorf("count user positive items: %w", err)
	}
	return count, nil
}

// NegativeItemEmbedding holds an embedding from a disliked or skipped item.
type NegativeItemEmbedding struct {
	ItemID    int64
	Embedding []float32
}

// GetUserNegativeItemEmbeddings returns embeddings for items that the user
// disliked or skipped. These are used to penalize recommendation candidates
// that are semantically similar to content the user rejected.
func GetUserNegativeItemEmbeddings(ctx context.Context, userID uuid.UUID) ([]NegativeItemEmbedding, error) {
	query := `
		select
			i.id,
			i.embedding
		from interactions x
		join items i on i.id = x.item_id
		where x.user_id = $1
		  and i.embedding is not null
		  and (
			coalesce(x.disliked, false) = true
			or coalesce(x.skipped, false) = true
		  )
		order by x.updated_at desc
	`

	rows, err := database.DB.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("query user negative item embeddings: %w", err)
	}
	defer rows.Close()

	result := make([]NegativeItemEmbedding, 0)
	for rows.Next() {
		var itemID int64
		var emb pgvector.Vector
		if err := rows.Scan(&itemID, &emb); err != nil {
			return nil, fmt.Errorf("scan negative item embedding: %w", err)
		}
		result = append(result, NegativeItemEmbedding{
			ItemID:    itemID,
			Embedding: emb.Slice(),
		})
	}
	if rows.Err() != nil {
		return nil, rows.Err()
	}

	return result, nil
}

// GetItemEmbeddingsByIDs returns embeddings for a batch of item IDs in a
// single query.  Items without embeddings are silently omitted.
func GetItemEmbeddingsByIDs(ctx context.Context, itemIDs []int64) (map[int64][]float32, error) {
	result := make(map[int64][]float32, len(itemIDs))
	if len(itemIDs) == 0 {
		return result, nil
	}

	query := `
		select id, embedding
		from items
		where id = any($1)
		  and embedding is not null
	`

	rows, err := database.DB.Query(ctx, query, itemIDs)
	if err != nil {
		return nil, fmt.Errorf("batch get item embeddings: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var itemID int64
		var emb pgvector.Vector
		if err := rows.Scan(&itemID, &emb); err != nil {
			return nil, fmt.Errorf("scan item embedding: %w", err)
		}
		result[itemID] = emb.Slice()
	}
	if rows.Err() != nil {
		return nil, rows.Err()
	}

	return result, nil
}

// GetProfileEmbeddingsByUserIDs returns the profile embedding for each of the
// given user IDs.  Users without a profile embedding are silently omitted.
func GetProfileEmbeddingsByUserIDs(ctx context.Context, userIDs []uuid.UUID) (map[uuid.UUID][]float32, error) {
	result := make(map[uuid.UUID][]float32, len(userIDs))
	if len(userIDs) == 0 {
		return result, nil
	}

	query := `
		select id, profile_embedding
		from users
		where id = any($1)
		  and profile_embedding is not null
	`

	rows, err := database.DB.Query(ctx, query, userIDs)
	if err != nil {
		return nil, fmt.Errorf("batch get profile embeddings: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var userID uuid.UUID
		var emb pgvector.Vector
		if err := rows.Scan(&userID, &emb); err != nil {
			return nil, fmt.Errorf("scan profile embedding: %w", err)
		}
		result[userID] = emb.Slice()
	}
	if rows.Err() != nil {
		return nil, rows.Err()
	}

	return result, nil
}

// ItemWithEmbedding is used for returning items together with their embeddings.
type ItemWithEmbedding struct {
	Item      models.Item
	Embedding []float32
}
