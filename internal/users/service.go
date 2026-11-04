package users

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"

	"recommendation-system/internal/models"
	"recommendation-system/internal/repository"
)

const (
	defaultSearchLimit = 20
	maxSearchLimit     = 50
)

var (
	ErrUserInactive        = errors.New("user is inactive")
	ErrSearchQueryRequired = errors.New("search query is required")
)

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

func SearchUsers(ctx context.Context, currentUserID uuid.UUID, query string, limit int) ([]models.UserSearchResult, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, ErrSearchQueryRequired
	}

	if limit <= 0 {
		limit = defaultSearchLimit
	}
	if limit > maxSearchLimit {
		limit = maxSearchLimit
	}

	return repository.SearchUsersWithFriendshipStatus(ctx, currentUserID, query, limit)
}

// UpdateProfile updates the profile fields (display_name, avatar_url, bio) for the given user.
func UpdateProfile(ctx context.Context, userID uuid.UUID, update repository.ProfileUpdate) (*models.User, error) {
	user, err := repository.GetUserByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if !user.IsActive {
		return nil, ErrUserInactive
	}

	return repository.UpdateUserProfile(ctx, userID, update)
}
