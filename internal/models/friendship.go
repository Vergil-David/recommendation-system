package models

import (
	"time"

	"github.com/google/uuid"
)

const (
	RelationStatusNone            = "none"
	RelationStatusPendingSent     = "pending_sent"
	RelationStatusPendingReceived = "pending_received"
	RelationStatusFriends         = "friends"
)

type PublicUser struct {
	ID          uuid.UUID `json:"id" example:"3fa85f64-5717-4562-b3fc-2c963f66afa6"`
	Email       string    `json:"email,omitempty" example:"user@example.com" format:"email"`
	Username    string    `json:"username" example:"johndoe"`
	DisplayName *string   `json:"display_name,omitempty" example:"John Doe"`
	AvatarURL   *string   `json:"avatar_url,omitempty" example:"https://cdn.example.com/avatars/johndoe.png"`
}

type UserSearchResult struct {
	ID             uuid.UUID `json:"id" example:"3fa85f64-5717-4562-b3fc-2c963f66afa6"`
	Email          string    `json:"email" example:"user@example.com" format:"email"`
	Username       string    `json:"username" example:"johndoe"`
	DisplayName    *string   `json:"display_name,omitempty" example:"John Doe"`
	AvatarURL      *string   `json:"avatar_url,omitempty" example:"https://cdn.example.com/avatars/johndoe.png"`
	RelationStatus string    `json:"relation_status" example:"none"`
}

type IncomingFriendRequest struct {
	FromUser  PublicUser `json:"from_user"`
	Status    string     `json:"status" example:"pending"`
	CreatedAt time.Time  `json:"created_at" format:"date-time"`
}

type OutgoingFriendRequest struct {
	ToUser    PublicUser `json:"to_user"`
	Status    string     `json:"status" example:"pending"`
	CreatedAt time.Time  `json:"created_at" format:"date-time"`
}
