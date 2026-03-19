package importer

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"recommendation-system/internal/repository"
	"recommendation-system/internal/tmdb"
)

type Service struct {
	tmdbClient *tmdb.Client
}

type Stats struct {
	PagesRequested       int
	PagesProcessed       int
	MoviesFetched        int
	Inserted             int
	SkippedNoPoster      int
	SkippedNoReleaseDate int
	SkippedDuplicates    int
	GenresCreated        int
}

func NewService(tmdbClient *tmdb.Client) *Service {
	return &Service{tmdbClient: tmdbClient}
}

func (s *Service) ImportUkrainianMovies(ctx context.Context, pages int) (*Stats, error) {
	if pages < 1 {
		return nil, fmt.Errorf("pages must be >= 1")
	}

	stats := &Stats{PagesRequested: pages}

	genres, err := s.tmdbClient.GetMovieGenres(ctx)
	if err != nil {
		return nil, err
	}

	genreMap := make(map[int]int64, len(genres))
	for _, g := range genres {
		name := strings.TrimSpace(g.Name)
		if name == "" {
			continue
		}

		genreID, created, err := repository.EnsureGenre(ctx, name)
		if err != nil {
			return nil, fmt.Errorf("ensure genre %q: %w", name, err)
		}
		if created {
			stats.GenresCreated++
		}

		genreMap[g.ID] = genreID
	}

	log.Printf("🎬 TMDB genres received: %d, created: %d", len(genres), stats.GenresCreated)

	for page := 1; page <= pages; page++ {
		resp, err := s.tmdbClient.DiscoverUkrainianMovies(ctx, page)
		if err != nil {
			return nil, err
		}

		stats.PagesProcessed++
		stats.MoviesFetched += len(resp.Results)
		log.Printf("📄 page %d/%d: received %d movies", page, pages, len(resp.Results))

		for _, movie := range resp.Results {
			if strings.TrimSpace(movie.PosterPath) == "" {
				stats.SkippedNoPoster++
				continue
			}

			releaseDate := strings.TrimSpace(movie.ReleaseDate)
			if releaseDate == "" {
				stats.SkippedNoReleaseDate++
				continue
			}

			releaseYear, err := parseReleaseYear(releaseDate)
			if err != nil {
				stats.SkippedNoReleaseDate++
				continue
			}

			title := strings.TrimSpace(movie.Title)
			if title == "" {
				title = strings.TrimSpace(movie.OriginalTitle)
			}
			if title == "" {
				stats.SkippedDuplicates++
				continue
			}

			exists, err := repository.MovieExistsByTMDBIDOrTitleYear(ctx, movie.ID, title, releaseYear)
			if err != nil {
				return nil, fmt.Errorf("check duplicate (tmdb_id=%d): %w", movie.ID, err)
			}
			if exists {
				stats.SkippedDuplicates++
				continue
			}

			description := strings.TrimSpace(movie.Overview)
			if description == "" {
				description = title
			}

			metadata := buildMetadata(movie)
			itemID, err := repository.InsertMovieItem(ctx, repository.MovieImportInput{
				Type:        "movie",
				Title:       title,
				Description: description,
				ReleaseYear: releaseYear,
				ImageURL:    s.tmdbClient.ImageURL(movie.PosterPath),
				Metadata:    metadata,
			})
			if err != nil {
				return nil, fmt.Errorf("insert movie (tmdb_id=%d): %w", movie.ID, err)
			}

			for _, tmdbGenreID := range movie.GenreIDs {
				localGenreID, ok := genreMap[tmdbGenreID]
				if !ok {
					continue
				}
				if err := repository.LinkItemGenre(ctx, itemID, localGenreID); err != nil {
					return nil, fmt.Errorf("link item=%d genre=%d: %w", itemID, localGenreID, err)
				}
			}

			stats.Inserted++
		}

		if resp.TotalPages > 0 && page >= resp.TotalPages {
			log.Printf("ℹ️ stopped at last available TMDB page: %d", resp.TotalPages)
			break
		}
	}

	return stats, nil
}

func parseReleaseYear(releaseDate string) (int, error) {
	if len(releaseDate) < 4 {
		return 0, fmt.Errorf("invalid release_date: %q", releaseDate)
	}

	if t, err := time.Parse("2006-01-02", releaseDate); err == nil {
		return t.Year(), nil
	}

	year, err := strconv.Atoi(releaseDate[:4])
	if err != nil {
		return 0, fmt.Errorf("parse release year from %q: %w", releaseDate, err)
	}
	return year, nil
}

func buildMetadata(movie tmdb.Movie) map[string]any {
	return map[string]any{
		"source":            "tmdb",
		"tmdb_id":           movie.ID,
		"original_title":    movie.OriginalTitle,
		"original_language": movie.OriginalLanguage,
		"popularity":        movie.Popularity,
		"vote_average":      movie.VoteAverage,
		"vote_count":        movie.VoteCount,
		"release_date":      movie.ReleaseDate,
	}
}
