package repository

import (
	"context"
	"errors"

	"recommendation-system/internal/database"
	"recommendation-system/internal/models"

	"github.com/jackc/pgx/v5"
)

var ErrBookNotFound = errors.New("book not found")

type GenreCount struct {
	Name  string
	Count int
}

func GetItemGenres(ctx context.Context, itemType string, minCount int) ([]GenreCount, error) {
	query := `
		SELECT g.name, COUNT(DISTINCT ig.item_id) AS cnt
		FROM genres g
		JOIN item_genres ig ON ig.genre_id = g.id
		JOIN items i ON i.id = ig.item_id AND i.type = $1
		GROUP BY g.name
		HAVING COUNT(DISTINCT ig.item_id) >= $2
		ORDER BY cnt DESC, g.name
	`
	rows, err := database.DB.Query(ctx, query, itemType, minCount)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []GenreCount
	for rows.Next() {
		var gc GenreCount
		if err := rows.Scan(&gc.Name, &gc.Count); err != nil {
			return nil, err
		}
		result = append(result, gc)
	}
	return result, rows.Err()
}

func GetBooks(ctx context.Context, limit int, offset int) ([]models.Item, int, error) {
	return getItemsByType(ctx, "book", limit, offset, false, "")
}

func GetBookByID(ctx context.Context, bookID int64) (*models.Item, error) {
	query := itemSelectProjection + itemSelectFromAndJoins + `
		where i.id = $1
		  and i.type = 'book'
		group by i.id
	`

	item, err := scanItemRow(database.DB.QueryRow(ctx, query, bookID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrBookNotFound
		}
		return nil, err
	}

	return item, nil
}
