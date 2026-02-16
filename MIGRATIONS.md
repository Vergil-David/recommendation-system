# Database Migrations

This project uses `golang-migrate` with SQL files in `internal/database/migrations`.

## Commands

```bash
go run ./cmd/migrate version
go run ./cmd/migrate up
go run ./cmd/migrate down 1
```

## Connection URL notes

- `MIGRATIONS_DATABASE_URL` should be a direct Postgres URL (for Supabase: `db.<project-ref>.supabase.co:5432`).
- `DATABASE_URL` can remain a runtime/pooler URL.
- If `MIGRATIONS_DATABASE_URL` is not set, migrations fall back to `DATABASE_URL`.
