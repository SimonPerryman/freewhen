# Calendar Creation Page

Status: done

## Goal

Build the landing/creation page at `/` — a simple form that creates a new calendar via the API and redirects the user to its view on success.

This is step 6 of `v1-build-order.md`, the first UI feature that exercises the frontend API client. The shareable-link copy button lives on the calendar view and is built in step 7.

## Context

- Backend exposes `POST /calendars` with body `{ name }` → returns `Calendar { id, name, createdAt }`.
- Frontend API client already implemented in `src/lib/api.ts`: `createCalendar(req)` handles snake/camel conversion and throws `ApiError` on non-2xx.
- Frontend is Next.js 16 (App Router) + React 19. Existing `src/app/page.tsx` is the default template and will be replaced.
- `src/components/` is empty; test tooling is Vitest + jsdom with `@` alias to `src/` and no React Testing Library installed yet.
- Backend accepts an empty `name`: the column is `name TEXT NOT NULL DEFAULT ''` (migrations/001_create_tables.sql) and the handler passes the decoded string straight to the store with no validation. Frontend can send whatever the user typed, trimmed, without substituting a default.

## Plan

### 1. Replace the landing page — `src/app/page.tsx`

- Convert to a client component (`"use client"`) since the form needs state + router navigation.
- Layout: centred card with a heading, a single text input for the optional event name, and a submit button.
- Form state via `useState`: `name`, `submitting`, `error`.
- On submit:
  - Call `createCalendar({ name: name.trim() })` — backend accepts empty strings, so no default substitution.
  - On success, `router.push(\`/calendars/${calendar.id}\`)` using `next/navigation`.
  - On `ApiError`, render the message inline; clear `submitting`.
- Disable the button while `submitting`.

### 2. Styling

- CSS modules, consistent with the existing `page.module.css` pattern. Scaffold doesn't include Tailwind and we're not introducing it here.
- Keep visuals minimal: system font stack, centred column, 480px max width, large readable input.

### 3. Tests

- `src/app/page.test.tsx` — component test for the creation page:
  - Renders the form.
  - Submits with a name, calls `createCalendar` (mocked), navigates on success.
  - Renders API error message on failure.
- Requires `@testing-library/react` + `@testing-library/user-event` + `@testing-library/jest-dom` — add these as devDependencies. Also wire a `vitest.setup.ts` that imports `@testing-library/jest-dom/vitest` and reference it from `vitest.config.ts` (`test.setupFiles`).

### 4. Env + runtime sanity

- `make dev-backend` + `make dev-frontend`, create a calendar through the UI, confirm it lands on `/calendars/<id>` (which will 404 until step 7 — acceptable).
- Check the network call hits `http://localhost:8080/calendars` with the correct body.

## Decisions

- Styling: CSS modules (no Tailwind at this step).
- Copy-link button: lives on `/calendars/[id]` and is built in step 7, not here.

## Log

- 2026-04-16 00:00 — Created plan
- 2026-04-16 00:00 — Confirmed from backend: empty name is valid (column default `''`, no handler validation). Removed open question on name defaulting.
- 2026-04-17 00:08 — Locked styling decision (CSS modules) and deferred `CopyLinkButton` to step 7; tightened scope to just the creation page.
- 2026-04-17 00:13 — Added @testing-library/{react,user-event,jest-dom} devDeps; wired vitest.setup.ts (jest-dom matchers + afterEach cleanup) into vitest.config.ts.
- 2026-04-17 00:13 — Implemented `src/app/page.tsx` as a client component (useState form, trimmed-name submit, router.push on success, inline ApiError message) and refreshed `page.module.css` to a centred 480px card with dark-mode vars.
- 2026-04-17 00:13 — Added `src/app/page.test.tsx` covering form render, trimmed-name submit + navigation, and ApiError inline rendering. All 13 frontend tests pass; oxlint clean.
