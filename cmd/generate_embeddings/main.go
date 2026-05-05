package main

import (
	"context"
	"flag"
	"log"
	"time"

	"recommendation-system/internal/config"
	"recommendation-system/internal/database"
	"recommendation-system/internal/embeddings"
	"recommendation-system/internal/repository"
)

func main() {
	forceAll := flag.Bool("force", false, "Regenerate embeddings for all items, including those that already have one")
	dimension := flag.Int("dim", embeddings.DefaultDimension, "Embedding vector dimension")
	flag.Parse()

	cfg := config.Load()
	database.InitDB(cfg.Database.URL)
	defer database.DB.Close()

	ctx := context.Background()

	// --- Report current state ---
	total, withEmbedding, err := repository.CountItemsWithEmbedding(ctx)
	if err != nil {
		log.Fatalf("❌ Failed to count items: %v", err)
	}
	log.Printf("📊 Current state: total=%d, with_embedding=%d, without=%d", total, withEmbedding, total-withEmbedding)

	if !*forceAll && withEmbedding == total && total > 0 {
		log.Printf("✅ All items already have embeddings. Use -force to regenerate.")
		return
	}

	// --- Fetch all items ---
	items, err := repository.GetAllItemsForEmbedding(ctx)
	if err != nil {
		log.Fatalf("❌ Failed to fetch items: %v", err)
	}
	log.Printf("📥 Fetched %d items for embedding generation (dim=%d)", len(items), *dimension)

	if len(items) == 0 {
		log.Printf("ℹ️ No items found. Import movies first.")
		return
	}

	// --- Phase 1: Fit (compute IDF from corpus) ---
	generator := embeddings.NewGenerator(*dimension)
	itemTexts := make([]embeddings.ItemText, len(items))
	for i, item := range items {
		itemTexts[i] = embeddings.ItemText{
			Title:       item.Title,
			Description: item.Description,
			Genres:      item.Genres,
		}
	}
	generator.Fit(itemTexts)
	log.Printf("📐 IDF model fitted on %d documents", len(items))

	// --- Phase 2: Transform + write to DB ---
	start := time.Now()
	updated := 0
	skipped := 0
	failed := 0

	for i, item := range items {
		vector := generator.Transform(itemTexts[i])

		if err := repository.UpdateItemEmbedding(ctx, item.ID, vector); err != nil {
			log.Printf("❌ item_id=%d title=%q: %v", item.ID, item.Title, err)
			failed++
			continue
		}

		updated++
		if (i+1)%100 == 0 || i+1 == len(items) {
			log.Printf("⏳ Progress: %d/%d (updated=%d, skipped=%d, failed=%d)",
				i+1, len(items), updated, skipped, failed)
		}
	}

	elapsed := time.Since(start)
	log.Printf("✅ Done in %s: updated=%d, skipped=%d, failed=%d",
		elapsed.Round(time.Millisecond), updated, skipped, failed)

	// --- Verify ---
	_, withEmbeddingAfter, _ := repository.CountItemsWithEmbedding(ctx)
	log.Printf("📊 Final state: %d/%d items have embeddings", withEmbeddingAfter, total)
}
