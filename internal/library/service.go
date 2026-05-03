package library

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"recommendation-system/internal/models"
	"recommendation-system/internal/repository"
)

func GetFavoriteMovies(ctx context.Context, userID uuid.UUID) ([]models.Item, error) {
	items, err := repository.GetUserItemsByInteractionType(ctx, userID, repository.InteractionTypeFavorite)
	if err != nil {
		return nil, fmt.Errorf("get favorite movies: %w", err)
	}
	return items, nil
}

func GetViewedMovies(ctx context.Context, userID uuid.UUID) ([]models.Item, error) {
	items, err := repository.GetUserItemsByInteractionType(ctx, userID, repository.InteractionTypeViewed)
	if err != nil {
		return nil, fmt.Errorf("get viewed movies: %w", err)
	}
	return items, nil
}

func GetLikedMovies(ctx context.Context, userID uuid.UUID) ([]models.Item, error) {
	items, err := repository.GetUserItemsByInteractionType(ctx, userID, repository.InteractionTypeLiked)
	if err != nil {
		return nil, fmt.Errorf("get liked movies: %w", err)
	}
	return items, nil
}
