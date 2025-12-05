package models

import "github.com/pgvector/pgvector-go"

type Item struct {
	ID          int64           `json:"id"`
	Title       string          `json:"title"`
	Description string          `json:"description"`
	Category    string          `json:"category"`
	Genres      []string        `json:"genres"`
	Embedding   pgvector.Vector `json:"-"` // Вектор не показуємо в JSON
}
