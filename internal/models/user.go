package models

import (
	"time"

	"github.com/google/uuid"
)

type User struct {
	ID           uuid.UUID `json:"id" example:"3fa85f64-5717-4562-b3fc-2c963f66afa6"`
	Email        string    `json:"email" example:"user@example.com" format:"email"`
	Username     string    `json:"username" example:"johndoe"`
	PasswordHash string    `json:"-"`

	DisplayName *string `json:"display_name,omitempty" example:"John Doe"`
	AvatarURL   *string `json:"avatar_url,omitempty" example:"https://cdn.example.com/avatars/johndoe.png"`
	Bio         *string `json:"bio,omitempty" example:"Book lover and sci-fi fan."`

	Role          string     `json:"role" example:"user"`
	EmailVerified bool       `json:"email_verified" example:"true"`
	IsActive      bool       `json:"is_active" example:"true"`
	LastLoginAt   *time.Time `json:"last_login_at,omitempty" format:"date-time"`

	CreatedAt time.Time `json:"created_at" format:"date-time"`
	UpdatedAt time.Time `json:"updated_at" format:"date-time"`
}
