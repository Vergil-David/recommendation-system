package repository

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5"

	"recommendation-system/internal/database"
	"recommendation-system/internal/models"
)

type MovieImportInput struct {
	Type        string
	Title       string
	Description string
	ReleaseYear int
	ImageURL    string
	Metadata    map[string]any
}

func MovieExistsByTMDBIDOrTitleYear(ctx context.Context, tmdbID int, title string, releaseYear int) (bool, error) {
	query := `
		select exists (
			select 1
			from items
			where type = 'movie'
			  and (
			    metadata->>'tmdb_id' = $1
			    or (title = $2 and release_year = $3)
			  )
		)
	`

	var exists bool
	if err := database.DB.QueryRow(ctx, query, fmt.Sprintf("%d", tmdbID), title, releaseYear).Scan(&exists); err != nil {
		return false, err
	}

	return exists, nil
}

func EnsureGenre(ctx context.Context, name string) (genreID int64, created bool, err error) {
	findQuery := `select id from genres where name = $1 limit 1`
	if err := database.DB.QueryRow(ctx, findQuery, name).Scan(&genreID); err == nil {
		return genreID, false, nil
	} else if err != pgx.ErrNoRows {
		return 0, false, err
	}

	insertQuery := `insert into genres (name) values ($1) returning id`
	if err := database.DB.QueryRow(ctx, insertQuery, name).Scan(&genreID); err == nil {
		return genreID, true, nil
	}

	// Concurrent insert fallback.
	if err := database.DB.QueryRow(ctx, findQuery, name).Scan(&genreID); err != nil {
		return 0, false, err
	}
	return genreID, false, nil
}

func InsertMovieItem(ctx context.Context, input MovieImportInput) (int64, error) {
	metadataJSON, err := json.Marshal(input.Metadata)
	if err != nil {
		return 0, fmt.Errorf("marshal metadata: %w", err)
	}

	query := `
		insert into items (type, title, description, release_year, image_url, metadata)
		values ($1, $2, $3, $4, $5, $6)
		returning id
	`

	var id int64
	if err := database.DB.QueryRow(
		ctx,
		query,
		input.Type,
		input.Title,
		input.Description,
		input.ReleaseYear,
		input.ImageURL,
		json.RawMessage(metadataJSON),
	).Scan(&id); err != nil {
		return 0, err
	}

	return id, nil
}

func LinkItemGenre(ctx context.Context, itemID, genreID int64) error {
	query := `
		insert into item_genres (item_id, genre_id)
		select $1, $2
		where not exists (
			select 1
			from item_genres
			where item_id = $1 and genre_id = $2
		)
	`

	_, err := database.DB.Exec(ctx, query, itemID, genreID)
	return err
}

func GetMovies(ctx context.Context, limit int, offset int) ([]models.Item, int, error) {
	countQuery := `select count(*) from items where type = 'movie'`

	var total int
	if err := database.DB.QueryRow(ctx, countQuery).Scan(&total); err != nil {
		return nil, 0, err
	}

	query := `
		select id, title, description, release_year, image_url
		from items
		where type = 'movie'
		order by created_at desc
		limit $1 offset $2
	`

	rows, err := database.DB.Query(ctx, query, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := make([]models.Item, 0, limit)
	for rows.Next() {
		var item models.Item
		if err := rows.Scan(
			&item.ID,
			&item.Title,
			&item.Description,
			&item.ReleaseYear,
			&item.ImageURL,
		); err != nil {
			return nil, 0, err
		}
		items = append(items, item)
	}
	if rows.Err() != nil {
		return nil, 0, rows.Err()
	}

	return items, total, nil
}
