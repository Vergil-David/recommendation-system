package books

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
var ErrInvalidBookID = errors.New("invalid book id")

type ListBooksResult struct {
	Items []models.Item
	Total int
	Page  int
	Limit int
}

func ListBooks(ctx context.Context, page int, limit int, random bool, genre string) (*ListBooksResult, error) {
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
	var (
		items []models.Item
		total int
		err   error
	)
	if genre != "" {
		items, total, err = repository.GetBooksByGenre(ctx, genre, limit, offset, random)
	} else if random {
		items, total, err = repository.GetBooksRandom(ctx, limit, offset)
	} else {
		items, total, err = repository.GetBooks(ctx, limit, offset)
	}
	if err != nil {
		return nil, err
	}

	return &ListBooksResult{
		Items: items,
		Total: total,
		Page:  page,
		Limit: limit,
	}, nil
}

func GetBookByID(ctx context.Context, bookID int64) (*models.Item, error) {
	if bookID < 1 {
		return nil, ErrInvalidBookID
	}
	return repository.GetBookByID(ctx, bookID)
}
