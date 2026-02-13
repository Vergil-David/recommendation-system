package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"recommendation-system/internal/database"
	"recommendation-system/internal/models"
)

func CreateUser(ctx context.Context, user *models.User) error {
	query := `
		insert into users (email, username, password_hash)
		values ($1, $2, $3)
		returning id
	`

	if err := database.DB.QueryRow(ctx, query, user.Email, user.Username, user.PasswordHash).Scan(&user.ID); err != nil {
		return err
	}

	applyUserDefaults(user)
	return nil
}

func GetUserByEmail(ctx context.Context, email string) (*models.User, error) {
	cols := []string{"id", "email", "username", "password_hash"}
	if database.HasUserColumn("role") {
		cols = append(cols, "role")
	}
	if database.HasUserColumn("email_verified") {
		cols = append(cols, "email_verified")
	}
	if database.HasUserColumn("is_active") {
		cols = append(cols, "is_active")
	}
	if database.HasUserColumn("last_login_at") {
		cols = append(cols, "last_login_at")
	}
	if database.HasUserColumn("created_at") {
		cols = append(cols, "created_at")
	}
	if database.HasUserColumn("updated_at") {
		cols = append(cols, "updated_at")
	}

	query := fmt.Sprintf(`
		select %s
		from users
		where email = $1
	`, strings.Join(cols, ", "))

	var user models.User
	var role sql.NullString
	var emailVerified sql.NullBool
	var isActive sql.NullBool
	var lastLogin sql.NullTime
	var createdAt sql.NullTime
	var updatedAt sql.NullTime

	dest := []any{&user.ID, &user.Email, &user.Username, &user.PasswordHash}
	if database.HasUserColumn("role") {
		dest = append(dest, &role)
	}
	if database.HasUserColumn("email_verified") {
		dest = append(dest, &emailVerified)
	}
	if database.HasUserColumn("is_active") {
		dest = append(dest, &isActive)
	}
	if database.HasUserColumn("last_login_at") {
		dest = append(dest, &lastLogin)
	}
	if database.HasUserColumn("created_at") {
		dest = append(dest, &createdAt)
	}
	if database.HasUserColumn("updated_at") {
		dest = append(dest, &updatedAt)
	}

	err := database.DB.QueryRow(ctx, query, email).Scan(dest...)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("user not found")
		}
		return nil, err
	}

	if role.Valid {
		user.Role = role.String
	}
	if emailVerified.Valid {
		user.EmailVerified = emailVerified.Bool
	}
	if isActive.Valid {
		user.IsActive = isActive.Bool
	}
	if lastLogin.Valid {
		user.LastLoginAt = &lastLogin.Time
	}
	if createdAt.Valid {
		user.CreatedAt = createdAt.Time
	}
	if updatedAt.Valid {
		user.UpdatedAt = updatedAt.Time
	}

	applyUserDefaults(&user)
	return &user, nil
}

func UpdateLastLogin(ctx context.Context, userID uuid.UUID) error {
	if !database.HasUserColumn("last_login_at") {
		return nil
	}
	query := `
		update users
		set last_login_at = now()
		where id = $1
	`

	_, err := database.DB.Exec(ctx, query, userID)
	return err
}

func applyUserDefaults(user *models.User) {
	if user.Role == "" {
		user.Role = "user"
	}
	if !database.HasUserColumn("email_verified") {
		user.EmailVerified = false
	}
	if !database.HasUserColumn("is_active") {
		user.IsActive = true
	}
}
