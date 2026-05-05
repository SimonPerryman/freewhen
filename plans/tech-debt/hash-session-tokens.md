# Hash session tokens at rest

Status: done

## Goal

Session tokens are currently stored plaintext in `sessions.token` (`backend/internal/store/session.go:30`). Magic-link tokens are already hashed (`backend/internal/store/magic_link.go`, migration `004_add_magic_link_tokens.sql`) — sessions should follow the same pattern.

Why this matters: a session token is a long-lived (30-day) bearer credential. If the row data is ever exposed — Neon backup leak, future SQL injection in another handler, compromised analytics export, operator with read access — every plaintext token in the table is a live, valid login for the corresponding user. Hashing closes that gap: an attacker with row access sees only `sha256(token)`, which is not a valid cookie value and cannot be reversed.

SHA-256 (not bcrypt/argon2) is the right primitive here: tokens are 32 random bytes (256 bits of entropy), so brute-force is infeasible without a slow hash, and we don't want bcrypt's latency on every authenticated request.

## Plan

1. **Migration `019_session_token_hash.sql`**:
   - Add `token_hash BYTEA` column to `sessions`.
   - Drop the existing primary key on `token`, add `PRIMARY KEY (token_hash)` once backfilled.
   - Existing sessions cannot be backfilled (we don't have the plaintext); the simplest correct approach is to truncate the `sessions` table as part of the migration. Users get logged out once and re-authenticate via magic link. Acceptable cost for a small user base; documented in the migration comment.
   - Drop the `token` column.
2. **`internal/store/session.go`**:
   - Add `hashToken(string) []byte` (mirror `magic_link.go`'s `hashToken`).
   - `CreateSession`: generate raw token, insert `hashToken(token)`, return raw token to caller.
   - `GetSession`, `RefreshSession`, `DeleteSession`: hash incoming token, query by `token_hash`.
   - `DeleteExpiredSessions` unchanged (no token comparison).
3. **`internal/model/model.go`**: `Session.Token` becomes `Session.TokenHash []byte` (or drop the field entirely from the struct since handlers only use `UserID` / `ExpiresAt`). Audit handler usage to avoid leaking the hash.
4. **Tests**: update `store_test.go` and any handler tests that read `Session.Token`. Add a regression test asserting `sessions` row stores hash, not plaintext.
5. **Integration test**: full login → cookie → authenticated request flow against embedded Postgres, to confirm round-trip works after the migration.

## Out of scope (covered by separate plan)

Cookie attribute hardening, session fixation on login, session rotation on privilege change, magic-link replay window, CSRF on mutating session endpoints — those land in `plans/tech-debt/auth-audit.md`.

## Log

- 2026-05-05 10:30 — Created plan. Modeled on existing `magic_link_tokens` hashing pattern.
- 2026-05-05 15:45 — Implemented. Migration `019_session_token_hash.sql` truncates `sessions`, drops `token` PK, adds `token_hash TEXT PRIMARY KEY`. `internal/store/session.go` hashes via `hashToken` on every insert/lookup/delete/refresh. Dropped `Token` field from `model.Session` (no handler depended on it; middleware now passes the raw cookie value to `RefreshSession`). Added `session_test.go` with round-trip, hashed-at-rest regression, and delete-invalidation tests. `make lint` clean, `make test` 223/223, `make test-integration` green.
