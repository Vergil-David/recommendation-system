package auth

import (
	"context"
	"errors"
	"log"

	"github.com/jackc/pgx/v5/pgconn"

	"recommendation-system/internal/models"
	"recommendation-system/internal/repository"
	"recommendation-system/internal/security"
)

var (
	jwtService *security.JWTService

	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrUserInactive       = errors.New("user is inactive")
	ErrJWTNotConfigured   = errors.New("jwt service not configured")
	ErrEmailTaken         = errors.New("email already registered")
	ErrUsernameTaken      = errors.New("username already taken")
)

func Init(service *security.JWTService) {
	jwtService = service
}

// Register реєструє нового користувача
func Register(ctx context.Context, email, username, password string) (*models.User, string, error) {
	if len(password) < 8 {
		log.Printf("❌ register: password too short, email=%s username=%s", email, username)
		return nil, "", errors.New("password too short")
	}

	hash, err := security.HashPassword(password)
	if err != nil {
		log.Printf("❌ register: password hash failed, email=%s username=%s err=%v", email, username, err)
		return nil, "", err
	}

	user := &models.User{
		Email:        email,
		Username:     username,
		PasswordHash: hash,
	}

	err = repository.CreateUser(ctx, user)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			log.Printf("❌ register: unique violation, email=%s username=%s constraint=%s detail=%s", email, username, pgErr.ConstraintName, pgErr.Detail)
			switch pgErr.ConstraintName {
			case "users_email_uq":
				return nil, "", ErrEmailTaken
			case "users_username_uq":
				return nil, "", ErrUsernameTaken
			}
		}
		if errors.As(err, &pgErr) {
			log.Printf("❌ register: db error, code=%s message=%s detail=%s where=%s", pgErr.Code, pgErr.Message, pgErr.Detail, pgErr.Where)
		} else {
			log.Printf("❌ register: create user failed, email=%s username=%s err=%v", email, username, err)
		}
		return nil, "", err
	}

	if jwtService == nil {
		log.Printf("❌ register: jwt service not configured")
		return nil, "", ErrJWTNotConfigured
	}

	token, err := jwtService.Generate(user.ID, user.Role)
	if err != nil {
		log.Printf("❌ register: token generation failed, user_id=%s err=%v", user.ID, err)
		return nil, "", err
	}

	log.Printf("✅ register: success, user_id=%s email=%s username=%s", user.ID, email, username)
	return user, token, nil
}

// Login авторизує користувача за email та паролем
func Login(ctx context.Context, email, password string) (*models.User, string, error) {
	user, err := repository.GetUserByEmail(ctx, email)
	if err != nil {
		log.Printf("❌ login: get user failed, email=%s err=%v", email, err)
		return nil, "", ErrInvalidCredentials
	}

	if !user.IsActive {
		log.Printf("❌ login: user inactive, user_id=%s email=%s", user.ID, email)
		return nil, "", ErrUserInactive
	}

	if !security.CheckPassword(user.PasswordHash, password) {
		log.Printf("❌ login: invalid password, user_id=%s email=%s", user.ID, email)
		return nil, "", ErrInvalidCredentials
	}

	if err = repository.UpdateLastLogin(ctx, user.ID); err != nil {
		log.Printf("❌ login: update last login failed, user_id=%s err=%v", user.ID, err)
		return nil, "", err
	}

	if jwtService == nil {
		log.Printf("❌ login: jwt service not configured")
		return nil, "", ErrJWTNotConfigured
	}

	token, err := jwtService.Generate(user.ID, user.Role)
	if err != nil {
		log.Printf("❌ login: token generation failed, user_id=%s err=%v", user.ID, err)
		return nil, "", err
	}

	log.Printf("✅ login: success, user_id=%s email=%s", user.ID, email)
	return user, token, nil
}
