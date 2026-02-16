package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"

	"recommendation-system/internal/config"
	"recommendation-system/internal/database"
)

func main() {
	cfg := config.Load()

	migrationsDir, err := resolveMigrationsDir()
	if err != nil {
		fatalf("failed to resolve migrations directory: %v", err)
	}

	if _, err := os.Stat(migrationsDir); err != nil {
		fatalf("migrations directory unavailable at %s: %v", migrationsDir, err)
	}

	sourceURL := fmt.Sprintf("file://%s", filepath.ToSlash(migrationsDir))
	fmt.Printf("Using migrations dir: %s\n", migrationsDir)
	fmt.Printf("Using migrations DB URL: %s\n", database.MaskDatabaseURL(cfg.Database.MigrationsURL))

	m, err := migrate.New(sourceURL, cfg.Database.MigrationsURL)
	if err != nil {
		fatalf("failed to initialize migrate: %v", err)
	}
	defer closeMigrate(m)

	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "up":
		if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
			fatalf("migration up failed: %v", err)
		}
		fmt.Println("Migrations applied")
	case "down":
		if len(os.Args) < 3 {
			fatalf("down requires number of steps: go run ./cmd/migrate down 1")
		}

		steps, err := strconv.Atoi(os.Args[2])
		if err != nil || steps <= 0 {
			fatalf("invalid down steps %q, expected positive integer", os.Args[2])
		}

		if err := m.Steps(-steps); err != nil && !errors.Is(err, migrate.ErrNoChange) {
			fatalf("migration down failed: %v", err)
		}
		fmt.Printf("Rolled back %d step(s)\n", steps)
	case "version":
		version, dirty, err := m.Version()
		if err != nil {
			if errors.Is(err, migrate.ErrNilVersion) {
				fmt.Println("version: none, dirty: false")
				return
			}
			fatalf("failed to read migration version: %v", err)
		}
		fmt.Printf("version: %d, dirty: %t\n", version, dirty)
	default:
		printUsage()
		os.Exit(1)
	}
}

func closeMigrate(m *migrate.Migrate) {
	srcErr, dbErr := m.Close()
	if srcErr != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to close source: %v\n", srcErr)
	}
	if dbErr != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to close database connection: %v\n", dbErr)
	}
}

func resolveMigrationsDir() (string, error) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		return "", errors.New("runtime.Caller failed")
	}

	baseDir := filepath.Join(filepath.Dir(thisFile), "..", "..")
	migrationsDir := filepath.Join(baseDir, "internal", "database", "migrations")
	return filepath.Abs(migrationsDir)
}

func printUsage() {
	fmt.Println("Usage:")
	fmt.Println("  go run ./cmd/migrate up")
	fmt.Println("  go run ./cmd/migrate down <steps>")
	fmt.Println("  go run ./cmd/migrate version")
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "Error: "+format+"\n", args...)
	os.Exit(1)
}
