package database

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"os"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var DB *pgxpool.Pool
var usersColumns map[string]bool

func InitDB(databaseURL string) {
	var err error
	redacted := redactDatabaseURL(databaseURL)
	fmt.Printf("🔎 DB URL: %s\n", redacted)

	if host := extractHost(databaseURL); host != "" {
		ips, dnsErr := net.LookupIP(host)
		if dnsErr != nil {
			fmt.Fprintf(os.Stderr, "⚠️ DNS lookup failed for %s: %v\n", host, dnsErr)
		} else {
			var ipStrs []string
			for _, ip := range ips {
				ipStrs = append(ipStrs, ip.String())
			}
			fmt.Printf("🔎 DNS %s -> %s\n", host, strings.Join(ipStrs, ", "))
		}
	}

	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ Failed to parse database url: %v\n", err)
		os.Exit(1)
	}
	// Disable prepared statement cache to avoid 42P05 with poolers (e.g., PgBouncer transaction mode).
	cfg.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeSimpleProtocol
	cfg.ConnConfig.StatementCacheCapacity = 0

	DB, err = pgxpool.NewWithConfig(context.Background(), cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ Failed to connect to database: %v\n", err)
		os.Exit(1)
	}

	if err = DB.Ping(context.Background()); err != nil {
		fmt.Fprintf(os.Stderr, "❌ Database ping failed: %v\n", err)
		os.Exit(1)
	}

	loadUsersColumns()
	fmt.Println("✅ Database connection established")
}

func redactDatabaseURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return "<invalid database url>"
	}
	if u.User != nil {
		u.User = url.User(u.User.Username())
	}
	return u.String()
}

func extractHost(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	return u.Hostname()
}

func HasUserColumn(name string) bool {
	if usersColumns == nil {
		return false
	}
	return usersColumns[strings.ToLower(name)]
}

func loadUsersColumns() {
	usersColumns = make(map[string]bool)
	rows, err := DB.Query(context.Background(), `
		select column_name
		from information_schema.columns
		where table_schema = 'public' and table_name = 'users'
	`)
	if err != nil {
		fmt.Fprintf(os.Stderr, "⚠️ failed to read users schema: %v\n", err)
		return
	}
	defer rows.Close()

	var cols []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			fmt.Fprintf(os.Stderr, "⚠️ failed to scan users column: %v\n", err)
			continue
		}
		usersColumns[strings.ToLower(name)] = true
		cols = append(cols, name)
	}
	if rows.Err() != nil {
		fmt.Fprintf(os.Stderr, "⚠️ users schema read error: %v\n", rows.Err())
		return
	}

	if len(cols) == 0 {
		fmt.Println("⚠️ users table not found or has no columns")
		return
	}
	fmt.Printf("🔎 users columns: %s\n", strings.Join(cols, ", "))
}
