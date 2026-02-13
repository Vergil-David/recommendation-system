package models

import "github.com/pgvector/pgvector-go"

type Item struct {
	ID          int64           `json:"id" example:"1"`
	Title       string          `json:"title" example:"Dune"`
	Description string          `json:"description" example:"Epic science fiction novel."`
	Category    string          `json:"category" example:"book"`
	Genres      []string        `json:"genres" example:"[\"sci-fi\",\"adventure\"]"`
	Embedding   pgvector.Vector `json:"-"` // Вектор не показуємо в JSON
}
