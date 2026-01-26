package database

import (
	"context"
	"fmt"
	"os"

	"github.com/jackc/pgx/v5"
)

var DB *pgx.Conn

func InitDB(databaseURL string) {
	var err error

	DB, err = pgx.Connect(context.Background(), databaseURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ Failed to connect to database: %v\n", err)
		os.Exit(1)
	}

	if err = DB.Ping(context.Background()); err != nil {
		fmt.Fprintf(os.Stderr, "❌ Database ping failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("✅ Database connection established")
}
