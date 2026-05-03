package repository

import (
	"context"
	"encoding/json"
	"errors"

	"recommendation-system/internal/database"

	"github.com/jackc/pgx/v5"
)

var ErrMovieNotFound = errors.New("movie not found")

type MovieRecord struct {
	ID          int64
	Title       string
	Description string
	ReleaseYear int
	ImageURL    string
	Metadata    map[string]any
}

func GetMovieByID(ctx context.Context, movieID int64) (*MovieRecord, error) {
	query := `
		select
			id,
			coalesce(title, ''),
			coalesce(description, ''),
			coalesce(release_year, 0),
			coalesce(image_url, ''),
			coalesce(metadata, '{}'::jsonb)
		from items
		where id = $1
		  and type = 'movie'
		limit 1
	`

	var movie MovieRecord
	var metadataRaw []byte
	if err := database.DB.QueryRow(ctx, query, movieID).Scan(
		&movie.ID,
		&movie.Title,
		&movie.Description,
		&movie.ReleaseYear,
		&movie.ImageURL,
		&metadataRaw,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrMovieNotFound
		}
		return nil, err
	}

	if len(metadataRaw) > 0 {
		if err := json.Unmarshal(metadataRaw, &movie.Metadata); err != nil {
			return nil, err
		}
	}
	if movie.Metadata == nil {
		movie.Metadata = map[string]any{}
	}

	return &movie, nil
}
