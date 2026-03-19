package config

import (
	"log"
	"os"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	Server   ServerConfig
	Database DatabaseConfig
	JWT      JWTConfig
	TMDB     TMDBConfig
}

type ServerConfig struct {
	Port        string
	FrontendURL string
}

type DatabaseConfig struct {
	URL           string
	MigrationsURL string
}

type JWTConfig struct {
	Secret    string
	ExpiresIn time.Duration
}

type TMDBConfig struct {
	APIKey       string
	BaseURL      string
	ImageBaseURL string
	HTTPTimeout  time.Duration
}

func Load() *Config {
	if err := godotenv.Load(); err != nil {
		log.Println("⚠️ .env file not found, using system environment variables")
	}

	cfg := &Config{
		Server: ServerConfig{
			Port:        getEnv("SERVER_PORT", "8080"),
			FrontendURL: getEnv("FRONTEND_URL", "http://localhost:5173"),
		},
		Database: DatabaseConfig{
			URL:           mustEnv("DATABASE_URL"),
			MigrationsURL: getEnv("MIGRATIONS_DATABASE_URL", ""),
		},
		JWT: JWTConfig{
			Secret:    mustEnv("JWT_SECRET"),
			ExpiresIn: mustDuration("JWT_EXPIRES_IN", "1h"),
		},
		TMDB: TMDBConfig{
			APIKey:       getEnv("TMDB_API_KEY", ""),
			BaseURL:      getEnv("TMDB_BASE_URL", "https://api.themoviedb.org/3"),
			ImageBaseURL: getEnv("TMDB_IMAGE_BASE_URL", "https://image.tmdb.org/t/p/w500"),
			HTTPTimeout:  mustDuration("TMDB_HTTP_TIMEOUT", "15s"),
		},
	}

	if cfg.Database.MigrationsURL == "" {
		cfg.Database.MigrationsURL = cfg.Database.URL
	}

	return cfg
}

func mustEnv(key string) string {
	val := os.Getenv(key)
	if val == "" {
		log.Fatalf("❌ %s is required", key)
	}
	return val
}

func getEnv(key, def string) string {
	val := os.Getenv(key)
	if val == "" {
		return def
	}
	return val
}

func mustDuration(key, def string) time.Duration {
	val := getEnv(key, def)
	d, err := time.ParseDuration(val)
	if err != nil {
		log.Fatalf("❌ invalid duration for %s: %v", key, err)
	}
	return d
}
