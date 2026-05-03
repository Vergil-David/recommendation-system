package movies

import (
	"context"
	"errors"

	"recommendation-system/internal/models"
	"recommendation-system/internal/repository"
)

const (
	defaultPage  = 1
	defaultLimit = 20
	maxLimit     = 50
)

var ErrInvalidPagination = errors.New("invalid pagination parameters")
var ErrInvalidMovieID = errors.New("invalid movie id")

type ListMoviesResult struct {
	Items []models.Item
	Total int
	Page  int
	Limit int
}

type GetMovieResult struct {
	ID          int64
	Title       string
	Description string
	ReleaseYear int
	ImageURL    string
	Metadata    map[string]any
}

func ListMovies(ctx context.Context, page int, limit int) (*ListMoviesResult, error) {
	if page < 0 || limit < 0 {
		return nil, ErrInvalidPagination
	}

	if page == 0 {
		page = defaultPage
	}
	if limit == 0 {
		limit = defaultLimit
	}
	if page < 1 || limit < 1 {
		return nil, ErrInvalidPagination
	}
	if limit > maxLimit {
		limit = maxLimit
	}

	offset := (page - 1) * limit
	items, total, err := repository.GetMovies(ctx, limit, offset)
	if err != nil {
		return nil, err
	}

	return &ListMoviesResult{
		Items: items,
		Total: total,
		Page:  page,
		Limit: limit,
	}, nil
}

func GetMovieByID(ctx context.Context, movieID int64) (*GetMovieResult, error) {
	if movieID < 1 {
		return nil, ErrInvalidMovieID
	}

	movie, err := repository.GetMovieByID(ctx, movieID)
	if err != nil {
		return nil, err
	}

	return &GetMovieResult{
		ID:          movie.ID,
		Title:       movie.Title,
		Description: movie.Description,
		ReleaseYear: movie.ReleaseYear,
		ImageURL:    movie.ImageURL,
		Metadata:    movie.Metadata,
	}, nil
}
