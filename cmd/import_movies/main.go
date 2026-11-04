package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	"recommendation-system/internal/config"
	"recommendation-system/internal/database"
	"recommendation-system/internal/importer"
	"recommendation-system/internal/tmdb"
)

func main() {
	pages := flag.Int("pages", 10, "number of TMDB pages to import (minimum 5)")
	flag.Parse()

	if *pages < 5 {
		log.Printf("⚠️ pages=%d is below minimum, using 5", *pages)
		*pages = 5
	}

	cfg := config.Load()
	if cfg.TMDB.APIKey == "" {
		fatalf("TMDB_API_KEY is required")
	}

	database.InitDB(cfg.Database.URL)
	defer database.DB.Close()

	client := tmdb.NewClient(
		cfg.TMDB.APIKey,
		cfg.TMDB.BaseURL,
		cfg.TMDB.ImageBaseURL,
		cfg.TMDB.HTTPTimeout,
	)
	service := importer.NewService(client)

	stats, err := service.ImportUkrainianMovies(context.Background(), *pages)
	if err != nil {
		fatalf("import failed: %v", err)
	}

	fmt.Println("✅ Import finished")
	fmt.Printf("Pages requested: %d\n", stats.PagesRequested)
	fmt.Printf("Pages processed: %d\n", stats.PagesProcessed)
	fmt.Printf("Movies received: %d\n", stats.MoviesFetched)
	fmt.Printf("Inserted: %d\n", stats.Inserted)
	fmt.Printf("Skipped (no poster): %d\n", stats.SkippedNoPoster)
	fmt.Printf("Skipped (no release_date): %d\n", stats.SkippedNoReleaseDate)
	fmt.Printf("Skipped (duplicates): %d\n", stats.SkippedDuplicates)
	fmt.Printf("Genres created: %d\n", stats.GenresCreated)
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "Error: "+format+"\n", args...)
	os.Exit(1)
}
