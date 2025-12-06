package auth

import (
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
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
