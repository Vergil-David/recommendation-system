# Friends Runtime Fix

## Summary

`GET /api/friends` was failing with `500 Internal Server Error` only when the current user already had accepted friends.

Requests with no friends still returned `200 {"friends":[]}` because the failing code path was only reached after `ListAcceptedFriendIDs` returned at least one UUID.

## Exact runtime cause

The issue was reproduced locally against the real database with a valid Bearer token and accepted friendship data.

Captured backend log:

```text
❌ get friends failed: user_id=<uuid> err=get users by ids: unable to encode []uuid.UUID{...} into text format for unknown type (OID 0): cannot find encode plan
```

Root cause:

- `internal/repository/user_repository.go:GetUsersByIDs`
- query used `where id = any($1)`
- argument was `[]uuid.UUID`
- runtime DB connection uses pgx simple protocol for the Supabase pooler
- under that protocol, pgx could not encode `[]uuid.UUID` for `ANY($1)` and returned an encode-plan error

## Route mapping

Runtime path confirmed:

1. Frontend dev request `GET /api/friends`
2. Vite proxy rewrites `/api/friends` -> `http://localhost:8080/friends`
3. Backend route `/friends` is registered in `cmd/api/main.go`
4. Route uses `auth.RequireAuth()`
5. Handler is `friends.GetFriendsHandler`
6. Current `userID` is read from Gin context via `auth.CtxUserIDKey`

## Fix

### 1. Stable user lookup for friend IDs

`GetUsersByIDs` no longer uses `ANY($1)` with a UUID slice.

It now builds a positional `IN ($1, $2, ...)` query and passes UUIDs as individual arguments, which works correctly with the current pgx runtime configuration.

### 2. Fallback for legacy one-direction accepted friendships

`ListAcceptedFriendIDs` now resolves accepted friends in both directions:

- direct row: `user_id = currentUser`
- reverse row: `friend_id = currentUser`

It returns the opposite side of the relation and deduplicates by friend ID. This keeps `GET /friends` working even if old data is missing the accepted mirror row.

Logical shape of the query:

```sql
SELECT CASE
  WHEN user_id = $1 THEN friend_id
  ELSE user_id
END AS friend_id
FROM friendships
WHERE status = 'accepted'
  AND (user_id = $1 OR friend_id = $1)
```

The implementation also groups/deduplicates and preserves newest-first ordering by accepted timestamp.

### 3. Better backend logs

`GetFriendsHandler` now logs:

- current `userID`
- failing stage
- original repository/runtime error

This keeps the client response generic (`500 internal server error`) while making the server logs useful for diagnosis.

## Migrations

Checked against the real database:

- before fix: migration version was `2`
- after applying pending migration: version is `3`, dirty state `false`

`0003_friend_requests_lists` is idempotent, so applying it was safe.

## Verification

Validated after the fix:

- no friends: `GET /friends` returns `200` with `{ "friends": [] }`
- A sends request to B
- B sees incoming request
- B accepts
- A `GET /friends` returns B
- B `GET /friends` returns A
- `GET /users/search`, incoming requests, outgoing requests, send request, and accept flow still work

## Frontend

No frontend design changes were required.

The existing Friends page already shows error and empty states in the current style. The main fix was backend/runtime.
