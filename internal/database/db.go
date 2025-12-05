package database

import (
	"context"
	"fmt"
	"os"

	"github.com/jackc/pgx/v5"
)

// Глобальна змінна, через яку ми будемо робити запити до БД
var DB *pgx.Conn

func InitDB() {
	// Отримуємо посилання з .env
	dbUrl := os.Getenv("DATABASE_URL")
	if dbUrl == "" {
		fmt.Println("❌ Помилка: DATABASE_URL не знайдено в .env")
		os.Exit(1)
	}

	// Підключаємось
	var err error
	DB, err = pgx.Connect(context.Background(), dbUrl)
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ Не вдалося підключитися до бази: %v\n", err)
		os.Exit(1)
	}

	// Перевіряємо пінг
	err = DB.Ping(context.Background())
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ База не відповідає: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("✅ Успішне підключення до Supabase PostgreSQL!")
}
