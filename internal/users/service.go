package users

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"recommendation-system/internal/models"
	"recommendation-system/internal/repository"
)

var ErrUserInactive = errors.New("user is inactive")

func GetMe(ctx context.Context, userID uuid.UUID) (*models.User, error) {
	user, err := repository.GetUserByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if !user.IsActive {
		return nil, ErrUserInactive
	}
	return user, nil
}
