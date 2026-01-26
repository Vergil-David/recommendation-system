package auth

import (
	"context"
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"

	"recommendation-system/internal/models"
	"recommendation-system/internal/repository"
)

// Секретний ключ для підпису токенів (в реальному проекті брати з .env!)
var jwtKey = []byte("super_secret_key_for_coursework")

// HashPassword перетворює пароль "12345" на "kdfjgh348t..."
func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), 14)
	return string(bytes), err
}

// CheckPasswordHash перевіряє, чи підходить пароль до хешу
func CheckPasswordHash(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

// GenerateJWT створює токен для юзера
func GenerateJWT(userID string) (string, error) {
	// Створюємо "наповнення" токена
	claims := jwt.MapClaims{
		"user_id": userID,
		"exp":     time.Now().Add(time.Hour * 72).Unix(), // Токен живе 72 години
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(jwtKey)
}

// Register реєструє нового користувача
func Register(ctx context.Context, email, username, password string) (*models.User, error) {
	if len(password) < 8 {
		return nil, errors.New("password too short")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	user := &models.User{
		Email:        email,
		Username:     username,
		PasswordHash: string(hash),
	}

	err = repository.CreateUser(ctx, user)
	if err != nil {
		return nil, err
	}

	return user, nil
}

// Login авторизує користувача за email та паролем
func Login(ctx context.Context, email, password string) (*models.User, error) {
	user, err := repository.GetUserByEmail(ctx, email)
	if err != nil {
		return nil, errors.New("invalid credentials")
	}

	err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password))
	if err != nil {
		return nil, errors.New("invalid credentials")
	}

	return user, nil
}
