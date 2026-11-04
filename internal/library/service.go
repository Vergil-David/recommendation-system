package library

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"recommendation-system/internal/models"
	"recommendation-system/internal/repository"
)

func GetFavoriteMovies(ctx context.Context, userID uuid.UUID, itemType string) ([]models.Item, error) {
	items, err := repository.GetUserItemsByInteractionTypeAndKind(ctx, userID, repository.InteractionTypeFavorite, itemType)
	if err != nil {
		return nil, fmt.Errorf("get favorites: %w", err)
	}
	return items, nil
}

func GetViewedMovies(ctx context.Context, userID uuid.UUID, itemType string) ([]models.Item, error) {
	items, err := repository.GetUserItemsByInteractionTypeAndKind(ctx, userID, repository.InteractionTypeViewed, itemType)
	if err != nil {
		return nil, fmt.Errorf("get viewed: %w", err)
	}
	return items, nil
}

func GetLikedMovies(ctx context.Context, userID uuid.UUID, itemType string) ([]models.Item, error) {
	items, err := repository.GetUserItemsByInteractionTypeAndKind(ctx, userID, repository.InteractionTypeLiked, itemType)
	if err != nil {
		return nil, fmt.Errorf("get liked: %w", err)
	}
	return items, nil
}
