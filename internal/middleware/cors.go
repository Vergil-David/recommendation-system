package middleware

import (
	"log"
	"net/http"
	"strings"

	"github.com/rs/cors"
)

func SetupCORS(handler http.Handler, frontendURL string) http.Handler {
	allowedOrigin := strings.TrimSpace(frontendURL)
	if allowedOrigin == "" {
		allowedOrigin = "http://localhost:5173"
	}

	c := cors.New(cors.Options{
		AllowedOrigins:   []string{allowedOrigin},
		AllowedMethods:   []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodOptions},
		AllowedHeaders:   []string{"Content-Type", "Authorization"},
		AllowCredentials: true,
	})

	base := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := strings.TrimSpace(r.Header.Get("Origin"))
		if origin != "" {
			log.Printf("🌐 CORS request origin=%s method=%s path=%s", origin, r.Method, r.URL.Path)
		}

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		handler.ServeHTTP(w, r)
	})

	return c.Handler(base)
}
