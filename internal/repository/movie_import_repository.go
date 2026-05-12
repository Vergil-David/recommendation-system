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
	return getItemsByType(ctx, "movie", limit, offset, false, "")
}

func GetMoviesRandom(ctx context.Context, limit int, offset int) ([]models.Item, int, error) {
	return getItemsByType(ctx, "movie", limit, offset, true, "")
}

func GetMoviesByGenre(ctx context.Context, genre string, limit int, offset int, random bool) ([]models.Item, int, error) {
	return getItemsByType(ctx, "movie", limit, offset, random, genre)
}

func GetBooksRandom(ctx context.Context, limit int, offset int) ([]models.Item, int, error) {
	return getItemsByType(ctx, "book", limit, offset, true, "")
}

func GetBooksByGenre(ctx context.Context, genre string, limit int, offset int, random bool) ([]models.Item, int, error) {
	return getItemsByType(ctx, "book", limit, offset, random, genre)
}

func getItemsByType(ctx context.Context, itemType string, limit, offset int, random bool, genre string) ([]models.Item, int, error) {
	order := `order by i.created_at desc`
	if random {
		order = `order by md5(i.id::text)`
	}

	var (
		total int
		query string
		args  []any
	)

	if genre != "" {
		// Count only items that have the requested genre
		if err := database.DB.QueryRow(ctx, `
			select count(distinct i.id)
			from items i
			join item_genres ig on ig.item_id = i.id
			join genres g on g.id = ig.genre_id
			where i.type = $1 and g.name = $2
		`, itemType, genre).Scan(&total); err != nil {
			return nil, 0, err
		}

		query = itemSelectProjection + itemSelectFromAndJoins + `
			where i.type = $1
			  and exists (
				select 1 from item_genres ig2
				join genres g2 on g2.id = ig2.genre_id
				where ig2.item_id = i.id and g2.name = $2
			  )
			group by i.id
			` + order + `
			limit $3 offset $4
		`
		args = []any{itemType, genre, limit, offset}
	} else {
		if err := database.DB.QueryRow(ctx, `select count(*) from items where type = $1`, itemType).Scan(&total); err != nil {
			return nil, 0, err
		}

		query = itemSelectProjection + itemSelectFromAndJoins + `
			where i.type = $1
			group by i.id
			` + order + `
			limit $2 offset $3
		`
		args = []any{itemType, limit, offset}
	}

	rows, err := database.DB.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := make([]models.Item, 0, limit)
	for rows.Next() {
		item, err := scanItem(rows)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, item)
	}
	if rows.Err() != nil {
		return nil, 0, rows.Err()
	}

	return items, total, nil
}
