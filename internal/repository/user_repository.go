package repository

import (
	"context"
	"errors"

	"recommendation-system/internal/database"
	"recommendation-system/internal/models"
)

func CreateUser(ctx context.Context, user *models.User) error {
	query := `
		insert into users (email, username, password_hash)
		values ($1, $2, $3)
		returning id, role, status, created_at, updated_at
	`

	return database.DB.QueryRow(
		ctx,
		query,
		user.Email,
		user.Username,
		user.PasswordHash,
	).Scan(
		&user.ID,
		&user.Role,
		&user.Status,
		&user.CreatedAt,
		&user.UpdatedAt,
	)
}

func GetUserByEmail(ctx context.Context, email string) (*models.User, error) {
	query := `
		select id, email, username, password_hash, role, status, created_at, updated_at
		from users
		where email = $1
	`

	var user models.User

	err := database.DB.QueryRow(ctx, query, email).Scan(
		&user.ID,
		&user.Email,
		&user.Username,
		&user.PasswordHash,
		&user.Role,
		&user.Status,
		&user.CreatedAt,
		&user.UpdatedAt,
	)

	if err != nil {
		return nil, errors.New("user not found")
	}

	return &user, nil
}
