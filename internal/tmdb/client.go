package tmdb

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type Client struct {
	apiKey       string
	baseURL      string
	imageBaseURL string
	httpClient   *http.Client
}

type Movie struct {
	ID               int     `json:"id"`
	Title            string  `json:"title"`
	Overview         string  `json:"overview"`
	ReleaseDate      string  `json:"release_date"`
	PosterPath       string  `json:"poster_path"`
	OriginalTitle    string  `json:"original_title"`
	OriginalLanguage string  `json:"original_language"`
	Popularity       float64 `json:"popularity"`
	VoteAverage      float64 `json:"vote_average"`
	VoteCount        int     `json:"vote_count"`
	GenreIDs         []int   `json:"genre_ids"`
}

type DiscoverMoviesResponse struct {
	Page       int     `json:"page"`
	TotalPages int     `json:"total_pages"`
	Results    []Movie `json:"results"`
}

type Genre struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type GenresResponse struct {
	Genres []Genre `json:"genres"`
}

func NewClient(apiKey, baseURL, imageBaseURL string, timeout time.Duration) *Client {
	return &Client{
		apiKey:       strings.TrimSpace(apiKey),
		baseURL:      strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		imageBaseURL: strings.TrimRight(strings.TrimSpace(imageBaseURL), "/"),
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
}

func (c *Client) ImageURL(posterPath string) string {
	trimmed := strings.TrimSpace(posterPath)
	if trimmed == "" {
		return ""
	}
	if strings.HasPrefix(trimmed, "/") {
		return c.imageBaseURL + trimmed
	}
	return c.imageBaseURL + "/" + trimmed
}

func (c *Client) GetMovieGenres(ctx context.Context) ([]Genre, error) {
	endpoint, err := c.withPath("/genre/movie/list")
	if err != nil {
		return nil, err
	}

	q := endpoint.Query()
	q.Set("api_key", c.apiKey)
	q.Set("language", "uk-UA")
	endpoint.RawQuery = q.Encode()

	var out GenresResponse
	if err := c.doJSON(ctx, endpoint.String(), &out); err != nil {
		return nil, fmt.Errorf("tmdb get genres: %w", err)
	}
	return out.Genres, nil
}

func (c *Client) DiscoverUkrainianMovies(ctx context.Context, page int) (*DiscoverMoviesResponse, error) {
	if page < 1 {
		return nil, fmt.Errorf("invalid page %d: must be >= 1", page)
	}

	endpoint, err := c.withPath("/discover/movie")
	if err != nil {
		return nil, err
	}

	q := endpoint.Query()
	q.Set("api_key", c.apiKey)
	q.Set("with_origin_country", "UA")
	q.Set("with_original_language", "uk")
	q.Set("include_adult", "false")
	q.Set("sort_by", "popularity.desc")
	q.Set("page", strconv.Itoa(page))
	endpoint.RawQuery = q.Encode()

	var out DiscoverMoviesResponse
	if err := c.doJSON(ctx, endpoint.String(), &out); err != nil {
		return nil, fmt.Errorf("tmdb discover movies page=%d: %w", page, err)
	}

	return &out, nil
}

func (c *Client) withPath(path string) (*url.URL, error) {
	base, err := url.Parse(c.baseURL)
	if err != nil {
		return nil, fmt.Errorf("invalid TMDB base URL: %w", err)
	}
	base.Path = strings.TrimRight(base.Path, "/") + path
	return base, nil
}

func (c *Client) doJSON(ctx context.Context, endpoint string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return fmt.Errorf("unexpected status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}

	return nil
}
