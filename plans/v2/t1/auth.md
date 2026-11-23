# Authentication

Status: done

## Goal

Add user authentication so calendar creators have durable identity. Creators must sign in to create a calendar. Joiners remain anonymous (as V1) with optional sign-in later. This unlocks admin controls, "my calendars", finalise-a-date, and notifications in future work.

## Design Decisions

**Creator must sign in.** No anonymous creation fallback — avoids maintaining two code paths and solves the core problem (creator loses control when they close the tab).

**Sign-in methods:** Google OAuth first, email magic link second. Apple can come later. Google covers the majority of users and is the fastest flow (~2 seconds). Magic link covers users without Google accounts.

**Session strategy:** Server-side sessions with HTTP-only cookies, stored in Postgres. No JWTs — single backend, single database, no benefit from self-contained tokens. Sessions are revocable and simpler.

**Session lifetime:** 30-day rolling expiry. Each authenticated request resets the clock. Users stay signed in as long as they visit once a month.

**Calendar ownership:** `POST /calendars` becomes an authenticated endpoint. `calendars.owner_id` is `NOT NULL` — every calendar has an owner. No prod data exists, so no migration path needed.

**Cross-origin cookies:** Frontend (Vercel) and backend (Cloud Run) are on different origins. To avoid `SameSite=None` fragility (Safari ITP, Firefox ETP), serve the backend on an `api.` subdomain of the frontend domain (e.g. `api.example.com` if the frontend is `example.com`). This makes cookies same-site and avoids third-party cookie issues entirely. The actual domain doesn't matter and can change — it only affects DNS + env vars, no code changes.

**CSRF protection:** With same-site cookies (`SameSite=Lax`), GET requests are safe and POST/PUT requests from cross-origin forms are blocked by the browser. As an additional layer, the CORS middleware already restricts `Access-Control-Allow-Origin` to the configured frontend origin, which blocks cross-origin fetch requests. No CSRF tokens needed.

**Session cleanup:** Lazy — check expiry at read time, ignore expired rows. Optionally add a periodic goroutine (`time.Ticker`, hourly) to prune dead rows, but not required for launch.

**Lazy auth check:** The frontend auth context does NOT call `/auth/me` on every page load. Most users are joiners (unauthenticated) and don't need it. Instead, call `/auth/me` lazily — only when needed (e.g. rendering the header, or when the user tries to create a calendar).

**Return URL through OAuth:** Encode the return URL in the OAuth `state` parameter so users land back where they were after sign-in. The flow is: frontend sets return URL → backend encodes it in state → Google round-trip → backend extracts return URL from state → redirects to frontend with return URL.

## Database Schema

Migration `002_add_auth.sql`:

```sql
CREATE TABLE users (
    id         TEXT PRIMARY KEY,
    email      TEXT NOT NULL UNIQUE,
    name       TEXT NOT NULL DEFAULT '',
    provider   TEXT NOT NULL,           -- 'google' or 'email'
    provider_id TEXT NOT NULL DEFAULT '',-- Google sub ID, empty for email users
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE sessions (
    token      TEXT PRIMARY KEY,
    user_id    TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_sessions_user_id ON sessions(user_id);
CREATE INDEX idx_sessions_expires_at ON sessions(expires_at);

ALTER TABLE calendars ADD COLUMN owner_id TEXT NOT NULL REFERENCES users(id);
```

User IDs use the same 8-char crypto-random alphanumeric scheme as calendars and participants (via `store.generateID`).

## API Changes

### New endpoints

| Method | Path | Description |
|--------|------|-------------|
| GET | `/auth/google` | Redirect to Google OAuth consent screen. Accepts `?return_url=` query param, encodes it in OAuth state. |
| GET | `/auth/google/callback` | Handle Google OAuth callback, create/find user, create session, set cookie, redirect to frontend return URL (from state param) |
| POST | `/auth/magic-link` | Send a magic link email to the given address |
| GET | `/auth/magic-link/callback` | Verify magic link token, create/find user, create session, set cookie, redirect |
| GET | `/auth/me` | Return current user (from session cookie) or 401 |
| POST | `/auth/logout` | Delete session, clear cookie |

### Modified endpoints

| Method | Path | Change |
|--------|------|--------|
| POST | `/calendars` | Requires auth. Sets `owner_id` from session. Returns 401 if no valid session. |

### Unchanged endpoints

All other endpoints (`GET /calendars/{id}`, `POST /calendars/{id}/participants`, `PUT /calendars/{id}/availability`) remain public. Joiners don't need auth.

## Backend Implementation

### Phase 1: Session infrastructure

1. **Migration** — `002_add_auth.sql` with `users`, `sessions` tables and `calendars.owner_id` column
2. **Store layer** — `internal/store/user.go` and `internal/store/session.go`
   - `CreateUser(email, name, provider, providerID) → User`
   - `FindUserByEmail(email) → User`
   - `FindUserByProviderID(provider, providerID) → User`
   - `CreateSession(userID) → token` (generates crypto-random token, 30-day expiry)
   - `GetSession(token) → Session` (returns nil if expired)
   - `RefreshSession(token)` (bumps expiry 30 days)
   - `DeleteSession(token)`
   - `DeleteExpiredSessions()` (cleanup, can run periodically)
3. **Auth middleware** — `internal/server/middleware.go`
   - Reads `session` cookie, looks up session, attaches user to `r.Context()`
   - Two flavours: `requireAuth` (401 if no session) and `optionalAuth` (attaches user if present, continues either way)
   - Session cookie settings: `HttpOnly`, `Secure`, `SameSite=Lax`, `Path=/`, `Domain=.<frontend_domain>` (configured via env var)
4. **CORS update** — add `Access-Control-Allow-Credentials: true` to the CORS middleware (required for cookies to be sent with cross-origin requests, even on same-site subdomains)
5. **Wire up** — Apply `requireAuth` to `POST /calendars`, `optionalAuth` to other endpoints

### Phase 2: Google OAuth

1. **Config** — Add `GOOGLE_CLIENT_ID`, `GOOGLE_CLIENT_SECRET`, `GOOGLE_REDIRECT_URL`, `FRONTEND_URL` env vars
2. **Handler** — `internal/handler/auth.go`
   - `GET /auth/google` — accept `?return_url=` query param, generate state param (JSON with random nonce + return URL, stored in short-lived `HttpOnly; Secure; SameSite=Lax` cookie), redirect to Google
   - `GET /auth/google/callback` — verify state nonce matches cookie, exchange code for token, fetch user info from Google, create-or-find user, create session, set session cookie, extract return URL from state, redirect to `FRONTEND_URL + returnURL`
   - Validate return URL is a relative path (prevent open redirect)
3. **Dependencies** — `golang.org/x/oauth2` and `golang.org/x/oauth2/google` (standard, minimal)

### Phase 3: Magic link (deferred)

1. **Handler** — `POST /auth/magic-link` generates a signed, time-limited token, emails it
2. **Callback** — `GET /auth/magic-link/callback` verifies token, creates/finds user, creates session
3. **Email** — needs a transactional email provider (Resend, SES, or similar). Defer until we pick one.

### Phase 4: Calendar ownership

1. **Modify `POST /calendars`** — read user from context, set `owner_id`
2. **Modify `GET /calendars/{id}`** — include `owner_id` in response (frontend needs it to show admin UI later)

## Frontend Implementation

### Phase 1: Auth state

1. **API client** — add `getMe()`, `logout()` to `src/lib/api.ts`. Cookies are sent automatically with `credentials: 'include'`.
2. **Auth context** — `src/components/AuthProvider/AuthProvider.tsx`. Provides `{ user, isLoading, isAuthenticated, checkAuth }` to the tree. Does NOT call `/auth/me` on mount — exposes `checkAuth()` for components that need it.
3. **Wire up** — wrap the app in `<AuthProvider>`.

### Phase 2: Sign-in UI

1. **Sign-in page** — `src/app/login/page.tsx`. "Continue with Google" button (links to `GET /auth/google?return_url=<encoded_path>`). Clean, minimal, mobile-first.
2. **Create calendar flow** — if not signed in, redirect to `/login?return_url=/` (or wherever they were). After auth, the backend redirects back via the state param, and the user lands on the create page ready to go.
3. **Header** — show user name/avatar when signed in, "Sign in" link when not. Calls `checkAuth()` on mount to populate user state.

### Phase 3: Logout

1. **Logout button** — calls `POST /auth/logout`, clears auth context, redirects to home.

## Infrastructure

- **DNS:** Add `api.<domain>` domain mapping to the Cloud Run service. This makes backend cookies same-site with the frontend.
- **Google Cloud:** Set up OAuth consent screen and credentials. Store `GOOGLE_CLIENT_ID`, `GOOGLE_CLIENT_SECRET`, `GOOGLE_REDIRECT_URL` (= `https://api.<domain>/auth/google/callback`), `FRONTEND_URL`, and `COOKIE_DOMAIN` in Secret Manager.
- **Cloud Run:** Update the service to mount the new secrets as env vars.

## Build Order

1. Infrastructure (DNS subdomain, Google OAuth credentials)
2. Backend Phase 1 (migration, store, middleware, CORS) — foundation, can test independently
3. Backend Phase 2 (Google OAuth) — working sign-in flow
4. Frontend Phase 1 (auth state, API client)
5. Frontend Phase 2 (sign-in page, create calendar gate)
6. Backend Phase 4 (calendar ownership)
7. Frontend Phase 3 (logout, header)
8. Backend Phase 3 (magic link) — deferred, add when email provider is chosen

## Open Questions

- ~~**Email provider for magic links**~~ — Decided: **Resend** (3k/month free, simple HTTP API). See `plans/v2/t1/magic-link.md`.

## Log

- 2026-04-18 — Created plan
- 2026-04-19 — Updated plan: made owner_id NOT NULL (no prod data), added CSRF strategy (SameSite=Lax + CORS), added api.freewhen.example.com subdomain for same-site cookies, added return URL encoding in OAuth state, made /auth/me lazy, added CORS credentials header, added infrastructure section, clarified user ID format
- 2026-04-19 — Implemented backend phases 1, 2, 4 and frontend phases 1, 2, 3. Migration 003_add_auth.sql (users, sessions tables, calendars.owner_id). Store layer: user.go, session.go. Auth middleware (requireAuth, optionalAuth) with session cookie + rolling expiry. CORS updated with credentials header. Auth handler: Google OAuth login/callback with state nonce verification, /auth/me, /auth/logout. Calendar ownership: POST /calendars requires auth, sets owner_id. Frontend: API client with credentials:include, AuthProvider context (lazy checkAuth), login page with Google button, home page auth gate, Header component with sign-in/sign-out. All unit tests updated and passing. Backend Phase 3 (magic link) deferred per plan. Infrastructure (DNS, Google OAuth creds) still needed before deployment.
- 2026-04-19 — Backend Phase 3 (magic link) implemented in plans/v2/t1/magic-link.md. All code phases complete. Marked done.
