package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("⚠️ .env not found, using system env")
	}

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("❌ DATABASE_URL is required")
	}

	cfg, err := pgxpool.ParseConfig(dbURL)
	if err != nil {
		log.Fatalf("❌ parse config: %v", err)
	}
	cfg.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeSimpleProtocol
	cfg.ConnConfig.StatementCacheCapacity = 0

	pool, err := pgxpool.NewWithConfig(context.Background(), cfg)
	if err != nil {
		log.Fatalf("❌ connect: %v", err)
	}
	defer pool.Close()

	if err := pool.Ping(context.Background()); err != nil {
		log.Fatalf("❌ ping: %v", err)
	}
	fmt.Println("✅ Connected to database")

	// Count before
	var itemCount int
	_ = pool.QueryRow(context.Background(), "SELECT COUNT(*) FROM public.items").Scan(&itemCount)
	var interactionCount int
	_ = pool.QueryRow(context.Background(), "SELECT COUNT(*) FROM public.interactions").Scan(&interactionCount)
	fmt.Printf("📊 Before: items=%d, interactions=%d\n", itemCount, interactionCount)

	// Step 1: truncate items (interactions cascade automatically)
	_, err = pool.Exec(context.Background(), "TRUNCATE TABLE public.items RESTART IDENTITY CASCADE")
	if err != nil {
		log.Fatalf("❌ truncate items: %v", err)
	}
	fmt.Println("🗑️  items truncated (interactions removed by CASCADE)")

	// Step 2: clear profile embeddings (now stale)
	tag, err := pool.Exec(context.Background(), "UPDATE public.users SET profile_embedding = NULL")
	if err != nil {
		log.Fatalf("❌ clear profile embeddings: %v", err)
	}
	fmt.Printf("🧹 profile_embedding cleared for %d users\n", tag.RowsAffected())

	fmt.Println("✅ Done. Ready to import new movies.")
}
