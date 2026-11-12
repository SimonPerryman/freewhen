# Public display name + mobile-safe header

Status: done

## Goal

Two coupled problems in the header:

1. On mobile the header shows `FreeWhen | My calendars | alex.morgan+resend… | Sign out`. With a long email the "My calendars" link wraps to two lines.
2. The email leaks as identity everywhere — it's PII per our logging rules, yet we're rendering it prominently in chrome. Users have no way to present themselves as "Simon" instead of a raw email.

Fix both by:

- Letting users set a display name on their account (`users.name` column already exists, just no UI).
- Collapsing account identity into a small avatar-menu so the header stays to two visual elements on any width: logo + menu.

## Plan

### Backend

1. `internal/limits/limits.go` — add `MaxUserNameLength = 50` (matches participant name cap).
2. `internal/store/user.go` — add `UpdateUserName(ctx, userID, name) error`. Server-side trims and rejects empty/over-cap strings.
3. `internal/handler/auth.go` — add `UpdateMe` that reads the authenticated user from context, decodes `{name}`, validates, calls `UpdateUserName`, responds with the fresh user.
4. `internal/server/server.go` — mount `PATCH /auth/me` behind `requireAuth`.

### Frontend

1. `src/lib/limits.ts` — mirror `MAX_USER_NAME_LENGTH = 50`.
2. `src/lib/api.ts` — `updateMe({ name })`.
3. `AuthProvider` — expose a `setUser` so the menu can reflect the saved name without a round-trip `checkAuth`.
4. New `src/components/UserMenu/` component:
   - Trigger: round avatar showing initial + (on ≥ 480px) display name + chevron. Pulsing accent dot when name is unset, prompting the user to set it.
   - Panel: dropdown on desktop, bottom sheet with handle on mobile.
   - Panel content: name row (display + inline edit), email row (muted, truncated), My calendars link, Sign out.
5. Simplify `Header` to logo + UserMenu. Remove the old inline layout that caused the wrap.

### Out of scope

- Retroactively renaming existing `Participant` records when user renames themselves. Participant names are per-calendar by design — user display name is a separate concept we already support alongside it.
- First-login name prompt modal. The menu surfaces a "Set your name" affordance; we don't need to block login.

## Log

- 2026-04-20 — Created plan.
- 2026-04-20 — Backend: added `MaxUserNameLength=50`, `store.UpdateUserName`, `PATCH /auth/me` handler behind `requireAuth`. All unit tests pass.
- 2026-04-20 — Frontend: mirrored `MAX_USER_NAME_LENGTH`, added `updateMe` to api client, exposed `setUser` on AuthProvider, built `UserMenu` (avatar trigger → bottom sheet on mobile / dropdown on desktop, inline name edit, email row, My-calendars, sign-out, Esc-to-close, click-outside-to-close, pulsing dot when name is unset). Header simplified to logo + `My calendars` (desktop only) + `UserMenu`. Type-check, oxlint, oxfmt, and full vitest suite (163 tests) green. Manual browser QA still pending.
