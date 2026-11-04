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

## Current migrations

- `0001_upgrade_users`: upgrades the existing `users` table with auth/profile fields, `citext`, unique indexes, and `updated_at` trigger.
- `0002_friendships_constraints`: adds friendship status/self/unique constraints and core friendship indexes.
- `0003_friend_requests_lists`: adds/backfills friendship timestamps, updated_at trigger, indexes for incoming/outgoing/friends queries, and trigram search indexes for `users.username`, `users.email`, and `users.display_name`.
