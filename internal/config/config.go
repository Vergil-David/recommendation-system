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
}

type ServerConfig struct {
	Port string
}

type DatabaseConfig struct {
	URL           string
	MigrationsURL string
}

type JWTConfig struct {
	Secret    string
	ExpiresIn time.Duration
}

func Load() *Config {
	if err := godotenv.Load(); err != nil {
		log.Println("⚠️ .env file not found, using system environment variables")
	}

	cfg := &Config{
		Server: ServerConfig{
			Port: getEnv("SERVER_PORT", "8080"),
		},
		Database: DatabaseConfig{
			URL:           mustEnv("DATABASE_URL"),
			MigrationsURL: getEnv("MIGRATIONS_DATABASE_URL", ""),
		},
		JWT: JWTConfig{
			Secret:    mustEnv("JWT_SECRET"),
			ExpiresIn: mustDuration("JWT_EXPIRES_IN", "1h"),
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
