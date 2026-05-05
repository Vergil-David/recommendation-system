package repository

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"recommendation-system/internal/database"
	"recommendation-system/internal/models"
)

const (
	InteractionTypeViewed   = "viewed"
	InteractionTypeLiked    = "liked"
	InteractionTypeDisliked = "disliked"
	InteractionTypeFavorite = "favorite"
	InteractionTypeSkipped  = "skipped"
)

type InteractionState struct {
	Viewed   bool
	Liked    bool
	Disliked bool
	Favorite bool
	Skipped  bool
}

type UserLikedItem struct {
	UserID uuid.UUID
	Item   models.Item
}

type UserPositiveItem struct {
	UserID   uuid.UUID
	Item     models.Item
	Liked    bool
	Favorite bool
}

func UpsertInteraction(ctx context.Context, userID uuid.UUID, itemID int64, interactionType string) error {
	var query string

	switch interactionType {
	case InteractionTypeViewed:
		query = `
			insert into interactions (
				user_id, item_id, interaction_type, is_liked, viewed_at, updated_at,
				viewed, liked, disliked, favorite, skipped
			)
			values ($1, $2, 'viewed', false, now(), now(), true, false, false, false, false)
			on conflict (user_id, item_id) do update
			set
				interaction_type = 'viewed',
				viewed = true,
				viewed_at = now(),
				updated_at = now()
		`
	case InteractionTypeLiked:
		query = `
			insert into interactions (
				user_id, item_id, interaction_type, is_liked, viewed_at, updated_at,
				viewed, liked, disliked, favorite, skipped
			)
			values ($1, $2, 'liked', true, null, now(), false, true, false, false, false)
			on conflict (user_id, item_id) do update
			set
				interaction_type = 'liked',
				is_liked = true,
				liked = true,
				disliked = false,
				skipped = false,
				updated_at = now()
		`
	case InteractionTypeDisliked:
		query = `
			insert into interactions (
				user_id, item_id, interaction_type, is_liked, viewed_at, updated_at,
				viewed, liked, disliked, favorite, skipped
			)
			values ($1, $2, 'disliked', false, null, now(), false, false, true, false, false)
			on conflict (user_id, item_id) do update
			set
				interaction_type = 'disliked',
				is_liked = false,
				liked = false,
				disliked = true,
				favorite = false,
				skipped = false,
				updated_at = now()
		`
	case InteractionTypeFavorite:
		query = `
			insert into interactions (
				user_id, item_id, interaction_type, is_liked, viewed_at, updated_at,
				viewed, liked, disliked, favorite, skipped
			)
			values ($1, $2, 'favorite', false, null, now(), false, false, false, true, false)
			on conflict (user_id, item_id) do update
			set
				interaction_type = 'favorite',
				disliked = false,
				favorite = true,
				skipped = false,
				updated_at = now()
		`
	case InteractionTypeSkipped:
		query = `
			insert into interactions (
				user_id, item_id, interaction_type, is_liked, viewed_at, updated_at,
				viewed, liked, disliked, favorite, skipped
			)
			values ($1, $2, 'skipped', false, null, now(), false, false, false, false, true)
			on conflict (user_id, item_id) do update
			set
				interaction_type = 'skipped',
				is_liked = false,
				liked = false,
				disliked = false,
				favorite = false,
				skipped = true,
				updated_at = now()
		`
	default:
		return fmt.Errorf("unsupported interaction type: %s", interactionType)
	}

	_, err := database.DB.Exec(ctx, query, userID, itemID)
	return err
}

func GetUserInteractionState(ctx context.Context, userID uuid.UUID, itemID int64) (InteractionState, error) {
	query := `
		select
			coalesce(viewed, false),
			coalesce(liked, false),
			coalesce(disliked, false),
			coalesce(favorite, false),
			coalesce(skipped, false)
		from interactions
		where user_id = $1
		  and item_id = $2
	`

	var state InteractionState
	if err := database.DB.QueryRow(ctx, query, userID, itemID).Scan(
		&state.Viewed,
		&state.Liked,
		&state.Disliked,
		&state.Favorite,
		&state.Skipped,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return InteractionState{}, nil
		}
		return InteractionState{}, err
	}

	return state, nil
}

func GetUserInteractionStates(ctx context.Context, userID uuid.UUID, itemIDs []int64) (map[int64]InteractionState, error) {
	states := make(map[int64]InteractionState, len(itemIDs))
	if len(itemIDs) == 0 {
		return states, nil
	}

	for _, itemID := range itemIDs {
		states[itemID] = InteractionState{}
	}

	query := `
		select
			item_id,
			coalesce(viewed, false),
			coalesce(liked, false),
			coalesce(disliked, false),
			coalesce(favorite, false),
			coalesce(skipped, false)
		from interactions
		where user_id = $1
		  and item_id = any($2)
	`

	rows, err := database.DB.Query(ctx, query, userID, itemIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var itemID int64
		var state InteractionState
		if err := rows.Scan(
			&itemID,
			&state.Viewed,
			&state.Liked,
			&state.Disliked,
			&state.Favorite,
			&state.Skipped,
		); err != nil {
			return nil, err
		}
		states[itemID] = state
	}
	if rows.Err() != nil {
		return nil, rows.Err()
	}

	return states, nil
}

func GetUserItemsByInteractionType(ctx context.Context, userID uuid.UUID, interactionType string) ([]models.Item, error) {
	stateColumn, err := interactionStateColumn(interactionType)
	if err != nil {
		return nil, err
	}

	query := itemSelectProjection + `
		from interactions x
		join items i on i.id = x.item_id
		left join item_genres ig on ig.item_id = i.id
		left join genres g on g.id = ig.genre_id
		where x.user_id = $1
		  and x.` + stateColumn + ` = true
		group by i.id, x.updated_at
		order by x.updated_at desc nulls last, i.id desc
	`

	rows, err := database.DB.Query(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]models.Item, 0)
	for rows.Next() {
		item, err := scanItem(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if rows.Err() != nil {
		return nil, rows.Err()
	}

	return items, nil
}

func GetUserExcludedItemIDs(ctx context.Context, userID uuid.UUID) ([]int64, error) {
	query := `
		select distinct item_id
		from interactions
		where user_id = $1
		  and item_id is not null
		  and (
			coalesce(viewed, false)
			or coalesce(liked, false)
			or coalesce(disliked, false)
			or coalesce(favorite, false)
			or coalesce(skipped, false)
		  )
	`

	rows, err := database.DB.Query(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	itemIDs := make([]int64, 0)
	for rows.Next() {
		var itemID int64
		if err := rows.Scan(&itemID); err != nil {
			return nil, err
		}
		itemIDs = append(itemIDs, itemID)
	}
	if rows.Err() != nil {
		return nil, rows.Err()
	}

	return itemIDs, nil
}

func GetUserLikedItems(ctx context.Context, userID uuid.UUID) ([]models.Item, error) {
	return GetUserItemsByInteractionType(ctx, userID, InteractionTypeLiked)
}

func GetLikedItemsByUserIDs(ctx context.Context, userIDs []uuid.UUID) ([]UserLikedItem, error) {
	if len(userIDs) == 0 {
		return []UserLikedItem{}, nil
	}

	args := make([]any, 0, len(userIDs))
	placeholders := make([]string, 0, len(userIDs))
	for _, userID := range userIDs {
		args = append(args, userID)
		placeholders = append(placeholders, "$"+strconv.Itoa(len(args)))
	}

	query := `
		select
			x.user_id,
			i.id,
			i.title,
			coalesce(i.description, '') as description,
			coalesce(i.release_year, 0) as release_year,
			coalesce(i.image_url, '') as image_url,
			coalesce(array_remove(array_agg(distinct g.name), null), '{}'::text[]) as genres
		from interactions x
		join items i on i.id = x.item_id
		left join item_genres ig on ig.item_id = i.id
		left join genres g on g.id = ig.genre_id
		where x.user_id in (` + strings.Join(placeholders, ", ") + `)
		  and (
			coalesce(x.liked, false) = true
			or x.interaction_type = 'like'
		  )
		group by x.user_id, i.id
	`

	rows, err := database.DB.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]UserLikedItem, 0)
	for rows.Next() {
		likedItem, err := scanUserLikedItem(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, likedItem)
	}
	if rows.Err() != nil {
		return nil, rows.Err()
	}

	return items, nil
}

func GetPositiveItemsByUserIDs(ctx context.Context, userIDs []uuid.UUID) ([]UserPositiveItem, error) {
	if len(userIDs) == 0 {
		return []UserPositiveItem{}, nil
	}

	args := make([]any, 0, len(userIDs))
	placeholders := make([]string, 0, len(userIDs))
	for _, userID := range userIDs {
		args = append(args, userID)
		placeholders = append(placeholders, "$"+strconv.Itoa(len(args)))
	}

	query := `
		select
			x.user_id,
			i.id,
			i.title,
			coalesce(i.description, '') as description,
			coalesce(i.release_year, 0) as release_year,
			coalesce(i.image_url, '') as image_url,
			coalesce(array_remove(array_agg(distinct g.name), null), '{}'::text[]) as genres,
			coalesce(x.liked, false) as liked,
			coalesce(x.favorite, false) as favorite
		from interactions x
		join items i on i.id = x.item_id
		left join item_genres ig on ig.item_id = i.id
		left join genres g on g.id = ig.genre_id
		where x.user_id in (` + strings.Join(placeholders, ", ") + `)
		  and (
			coalesce(x.liked, false) = true
			or x.interaction_type = 'like'
			or coalesce(x.favorite, false) = true
		  )
		group by x.user_id, i.id, x.liked, x.favorite
	`

	rows, err := database.DB.Query(ctx, query, args...)
	if err != nil {
		log.Printf("❌ repository: get positive items query failed, user_count=%d err=%v", len(userIDs), err)
		return nil, err
	}
	defer rows.Close()

	items := make([]UserPositiveItem, 0)
	for rows.Next() {
		positiveItem, err := scanUserPositiveItem(rows)
		if err != nil {
			log.Printf("❌ repository: scan positive item failed, user_count=%d err=%v", len(userIDs), err)
			return nil, err
		}
		items = append(items, positiveItem)
	}
	if rows.Err() != nil {
		log.Printf("❌ repository: get positive items row iteration failed, user_count=%d err=%v", len(userIDs), rows.Err())
		return nil, rows.Err()
	}
	log.Printf("ℹ️ repository: positive items loaded, user_count=%d item_count=%d", len(userIDs), len(items))

	return items, nil
}

func interactionStateColumn(interactionType string) (string, error) {
	switch interactionType {
	case InteractionTypeViewed:
		return "viewed", nil
	case InteractionTypeLiked:
		return "liked", nil
	case InteractionTypeDisliked:
		return "disliked", nil
	case InteractionTypeFavorite:
		return "favorite", nil
	case InteractionTypeSkipped:
		return "skipped", nil
	default:
		return "", fmt.Errorf("unsupported interaction type: %s", interactionType)
	}
}

// ResetInteractionState toggles off a single interaction boolean for a (user, item) pair.
// This enables undo/toggle behavior: if a user clicks "liked" again, the like is removed.
func ResetInteractionState(ctx context.Context, userID uuid.UUID, itemID int64, interactionType string) error {
	stateColumn, err := interactionStateColumn(interactionType)
	if err != nil {
		return err
	}

	// Build the SET clause: reset the boolean and sync is_liked for the liked type.
	setClause := stateColumn + " = false"
	if interactionType == InteractionTypeLiked {
		setClause = "liked = false, is_liked = false"
	}

	query := `update interactions set ` + setClause + `, updated_at = now() where user_id = $1 and item_id = $2`
	_, err = database.DB.Exec(ctx, query, userID, itemID)
	return err
}

func scanUserLikedItem(row rowScanner) (UserLikedItem, error) {
	var likedItem UserLikedItem
	if err := row.Scan(
		&likedItem.UserID,
		&likedItem.Item.ID,
		&likedItem.Item.Title,
		&likedItem.Item.Description,
		&likedItem.Item.ReleaseYear,
		&likedItem.Item.ImageURL,
		&likedItem.Item.Genres,
	); err != nil {
		return UserLikedItem{}, err
	}
	return likedItem, nil
}

func scanUserPositiveItem(row rowScanner) (UserPositiveItem, error) {
	var positiveItem UserPositiveItem
	if err := row.Scan(
		&positiveItem.UserID,
		&positiveItem.Item.ID,
		&positiveItem.Item.Title,
		&positiveItem.Item.Description,
		&positiveItem.Item.ReleaseYear,
		&positiveItem.Item.ImageURL,
		&positiveItem.Item.Genres,
		&positiveItem.Liked,
		&positiveItem.Favorite,
	); err != nil {
		return UserPositiveItem{}, err
	}
	return positiveItem, nil
}
