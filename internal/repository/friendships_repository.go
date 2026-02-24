package repository

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"recommendation-system/internal/database"
)

var ErrFriendRequestAlreadyExists = errors.New("friend request already exists")

func CreateFriendRequest(ctx context.Context, fromUserID, toUserID uuid.UUID) error {
	query := `
		insert into friendships (user_id, friend_id, status, created_at)
		values ($1, $2, 'pending', now())
	`

	_, err := database.DB.Exec(ctx, query, fromUserID, toUserID)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return ErrFriendRequestAlreadyExists
		}
		return err
	}

	return nil
}

func GetFriendshipStatus(ctx context.Context, fromUserID, toUserID uuid.UUID) (status string, found bool, err error) {
	query := `
		select status
		from friendships
		where user_id = $1 and friend_id = $2
	`

	if err := database.DB.QueryRow(ctx, query, fromUserID, toUserID).Scan(&status); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", false, nil
		}
		return "", false, err
	}

	return status, true, nil
}

func UpdateFriendRequestStatus(ctx context.Context, fromUserID, toUserID uuid.UUID, status string) error {
	query := `
		update friendships
		set status = $3
		where user_id = $1 and friend_id = $2
	`

	_, err := database.DB.Exec(ctx, query, fromUserID, toUserID, status)
	return err
}

func EnsureAcceptedMirror(ctx context.Context, requesterID, receiverID uuid.UUID) error {
	query := `
		insert into friendships (user_id, friend_id, status, created_at)
		values ($1, $2, 'accepted', now())
		on conflict (user_id, friend_id) do nothing
	`

	_, err := database.DB.Exec(ctx, query, receiverID, requesterID)
	return err
}

func ListAcceptedFriendIDs(ctx context.Context, userID uuid.UUID) ([]uuid.UUID, error) {
	query := `
		select friend_id
		from friendships
		where user_id = $1 and status = 'accepted'
		order by created_at desc
	`

	rows, err := database.DB.Query(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	ids := make([]uuid.UUID, 0)
	for rows.Next() {
		var friendID uuid.UUID
		if err := rows.Scan(&friendID); err != nil {
			return nil, err
		}
		ids = append(ids, friendID)
	}
	if rows.Err() != nil {
		return nil, rows.Err()
	}

	return ids, nil
}
