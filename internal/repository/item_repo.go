package repository

import (
	"context"
	"fmt"
	"recommendation-system/internal/database"
	"recommendation-system/internal/models"

	"github.com/pgvector/pgvector-go"
)

// FindSimilar шукає записи, вектори яких найближчі до userVector
func FindSimilar(userVector []float32) ([]models.Item, error) {
	// <-> це оператор відстані. Чим менше число, тим схожіший фільм.
	query := `SELECT id, title, category, genres FROM items ORDER BY embedding <-> $1 LIMIT 5`

	rows, err := database.DB.Query(context.Background(), query, pgvector.NewVector(userVector))
	if err != nil {
		return nil, fmt.Errorf("помилка запиту: %w", err)
	}
	defer rows.Close()

	var items []models.Item
	for rows.Next() {
		var i models.Item
		// Зчитуємо дані з бази у структуру
		if err := rows.Scan(&i.ID, &i.Title, &i.Category, &i.Genres); err != nil {
			return nil, err
		}
		items = append(items, i)
	}

	return items, nil
}
