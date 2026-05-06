package repository

import (
	"context"
	stdsql "database/sql"
	"errors"
	"fmt"
	"log"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"recommendation-system/internal/database"
	"recommendation-system/internal/models"
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
		on conflict (user_id, friend_id) do update
		set status = 'accepted'
	`

	_, err := database.DB.Exec(ctx, query, receiverID, requesterID)
	return err
}

func SearchUsersWithFriendshipStatus(ctx context.Context, currentUserID uuid.UUID, queryText string, limit int) ([]models.UserSearchResult, error) {
	displayNameSelect := "null::text"
	displayNameSearch := "false"
	displayNamePrefixSearch := "false"
	if database.HasUserColumn("display_name") {
		displayNameSelect = "u.display_name"
		displayNameSearch = "lower(coalesce(u.display_name, '')) like '%' || lower($2) || '%'"
		displayNamePrefixSearch = "lower(coalesce(u.display_name, '')) like lower($2) || '%'"
	}

	avatarURLSelect := "null::text"
	if database.HasUserColumn("avatar_url") {
		avatarURLSelect = "u.avatar_url"
	}

	activeFilter := ""
	if database.HasUserColumn("is_active") {
		activeFilter = "and u.is_active = true"
	}

	query := fmt.Sprintf(`
		select
			u.id,
			u.email,
			u.username,
			%s as display_name,
			%s as avatar_url,
			case
				when exists (
					select 1
					from friendships f
					where f.status = 'accepted'
					  and (
						(f.user_id = $1 and f.friend_id = u.id)
						or (f.user_id = u.id and f.friend_id = $1)
					  )
				) then '%s'
				when exists (
					select 1
					from friendships f
					where f.user_id = $1
					  and f.friend_id = u.id
					  and f.status = 'pending'
				) then '%s'
				when exists (
					select 1
					from friendships f
					where f.user_id = u.id
					  and f.friend_id = $1
					  and f.status = 'pending'
				) then '%s'
				else '%s'
			end as relation_status
		from users u
		where u.id <> $1
		  %s
		  and (
			lower(u.username::text) like '%%' || lower($2) || '%%'
			or lower(u.email::text) like '%%' || lower($2) || '%%'
			or %s
		  )
		order by
			case
				when lower(u.username::text) = lower($2) then 0
				when lower(u.username::text) like lower($2) || '%%' then 1
				when %s then 2
				when lower(u.email::text) like lower($2) || '%%' then 3
				else 4
			end,
			u.username
		limit $3
	`, displayNameSelect, avatarURLSelect,
		models.RelationStatusFriends,
		models.RelationStatusPendingSent,
		models.RelationStatusPendingReceived,
		models.RelationStatusNone,
		activeFilter,
		displayNameSearch,
		displayNamePrefixSearch,
	)

	rows, err := database.DB.Query(ctx, query, currentUserID, queryText, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	users := make([]models.UserSearchResult, 0)
	for rows.Next() {
		var user models.UserSearchResult
		var displayName stdsql.NullString
		var avatarURL stdsql.NullString

		if err := rows.Scan(&user.ID, &user.Email, &user.Username, &displayName, &avatarURL, &user.RelationStatus); err != nil {
			return nil, err
		}
		if displayName.Valid {
			value := displayName.String
			user.DisplayName = &value
		}
		if avatarURL.Valid {
			value := avatarURL.String
			user.AvatarURL = &value
		}
		users = append(users, user)
	}
	if rows.Err() != nil {
		return nil, rows.Err()
	}

	return users, nil
}

func ListIncomingFriendRequests(ctx context.Context, userID uuid.UUID) ([]models.IncomingFriendRequest, error) {
	displayNameSelect := "null::text"
	if database.HasUserColumn("display_name") {
		displayNameSelect = "u.display_name"
	}

	avatarURLSelect := "null::text"
	if database.HasUserColumn("avatar_url") {
		avatarURLSelect = "u.avatar_url"
	}

	query := fmt.Sprintf(`
		select
			u.id,
			u.username,
			%s as display_name,
			%s as avatar_url,
			f.status,
			f.created_at
		from friendships f
		join users u on u.id = f.user_id
		where f.friend_id = $1
		  and f.status = 'pending'
		order by f.created_at desc
	`, displayNameSelect, avatarURLSelect)

	rows, err := database.DB.Query(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	requests := make([]models.IncomingFriendRequest, 0)
	for rows.Next() {
		var request models.IncomingFriendRequest
		var displayName stdsql.NullString
		var avatarURL stdsql.NullString

		if err := rows.Scan(
			&request.FromUser.ID,
			&request.FromUser.Username,
			&displayName,
			&avatarURL,
			&request.Status,
			&request.CreatedAt,
		); err != nil {
			return nil, err
		}
		if displayName.Valid {
			value := displayName.String
			request.FromUser.DisplayName = &value
		}
		if avatarURL.Valid {
			value := avatarURL.String
			request.FromUser.AvatarURL = &value
		}
		requests = append(requests, request)
	}
	if rows.Err() != nil {
		return nil, rows.Err()
	}

	return requests, nil
}

func ListOutgoingFriendRequests(ctx context.Context, userID uuid.UUID) ([]models.OutgoingFriendRequest, error) {
	displayNameSelect := "null::text"
	if database.HasUserColumn("display_name") {
		displayNameSelect = "u.display_name"
	}

	avatarURLSelect := "null::text"
	if database.HasUserColumn("avatar_url") {
		avatarURLSelect = "u.avatar_url"
	}

	query := fmt.Sprintf(`
		select
			u.id,
			u.username,
			%s as display_name,
			%s as avatar_url,
			f.status,
			f.created_at
		from friendships f
		join users u on u.id = f.friend_id
		where f.user_id = $1
		  and f.status = 'pending'
		order by f.created_at desc
	`, displayNameSelect, avatarURLSelect)

	rows, err := database.DB.Query(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	requests := make([]models.OutgoingFriendRequest, 0)
	for rows.Next() {
		var request models.OutgoingFriendRequest
		var displayName stdsql.NullString
		var avatarURL stdsql.NullString

		if err := rows.Scan(
			&request.ToUser.ID,
			&request.ToUser.Username,
			&displayName,
			&avatarURL,
			&request.Status,
			&request.CreatedAt,
		); err != nil {
			return nil, err
		}
		if displayName.Valid {
			value := displayName.String
			request.ToUser.DisplayName = &value
		}
		if avatarURL.Valid {
			value := avatarURL.String
			request.ToUser.AvatarURL = &value
		}
		requests = append(requests, request)
	}
	if rows.Err() != nil {
		return nil, rows.Err()
	}

	return requests, nil
}

func ListAcceptedFriendIDs(ctx context.Context, userID uuid.UUID) ([]uuid.UUID, error) {
	query := `
		select friend_id
		from (
			select
				case
					when user_id = $1 then friend_id
					else user_id
				end as friend_id,
				max(created_at) as accepted_at
			from friendships
			where status = 'accepted'
			  and (user_id = $1 or friend_id = $1)
			group by 1
		) accepted_friends
		order by accepted_at desc
	`

	rows, err := database.DB.Query(ctx, query, userID)
	if err != nil {
		log.Printf("❌ repository: list accepted friend ids query failed, user_id=%s err=%v", userID, err)
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
		log.Printf("❌ repository: list accepted friend ids row iteration failed, user_id=%s err=%v", userID, rows.Err())
		return nil, rows.Err()
	}
	log.Printf("ℹ️ repository: accepted friend ids loaded, user_id=%s count=%d", userID, len(ids))

	return ids, nil
}

// DeleteFriendship removes both directions of an accepted friendship.
// Because EnsureAcceptedMirror creates rows in both directions,
// we must delete both (user_id, friend_id) and (friend_id, user_id).
func DeleteFriendship(ctx context.Context, userID, friendID uuid.UUID) (int64, error) {
	query := `
		delete from friendships
		where (user_id = $1 and friend_id = $2)
		   or (user_id = $2 and friend_id = $1)
	`

	tag, err := database.DB.Exec(ctx, query, userID, friendID)
	if err != nil {
		return 0, fmt.Errorf("delete friendship: %w", err)
	}

	return tag.RowsAffected(), nil
}
