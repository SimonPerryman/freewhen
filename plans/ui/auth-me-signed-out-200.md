# GET /auth/me returns 200 null when signed out

Status: done

## Goal

A signed-out visitor's `GET /auth/me` currently returns `401`. Chrome logs
every non-2xx fetch as a console error, so every page load for a signed-out
visitor shows a red console error and Lighthouse docks Best Practices
("Browser errors were logged to the console"). Being signed out is a normal
state, not an error: `/auth/me` should answer "nobody" with a `200`.

## Decision: response shape

`200` with a bare JSON `null` body (not `{"user": null}`).

- The signed-in response is the bare `User` object, so the endpoint's type
  becomes `User | null` — no wrapper, no breaking change to the signed-in
  shape, and the e2e test's `body.email` read keeps working.
- The frontend's `request<T>()` already does `response.json()` →
  `toCamelCaseKeys`, and `convertKeys(null)` returns `null`, so
  `getMe(): Promise<User | null>` works with no special casing.
- `AuthProvider` already models signed-out as `user: null`.

## Plan

Backend
- Register `GET /auth/me` with `handleOptional` (optionalAuth middleware)
  instead of `handlePublic`. `Me` reads `UserFromContext` and responds
  `200 null` when absent. This drops the handler's duplicated
  cookie/session/user lookup and means a valid session also slides its
  expiry on `/auth/me` (same as every other optional/required route).
- Missing, expired, invalid or logged-out session cookies all → `200 null`.
  The old handler did not clear a bad cookie, so neither does the new one
  (no behaviour change there).
- Every other authenticated endpoint is untouched (`requireAuth` still 401s;
  `PATCH /auth/me` stays `handleAuth`).
- Route-coverage test: `GET /auth/me` → `authOptional`.
- Tests: integration `TestAuth_Me` (no cookie, garbage cookie, valid,
  after-logout → 200 null), plus a unit test for `Me` with/without a
  context user.

Frontend
- `getMe(): Promise<User | null>`.
- `AuthProvider.checkAuth` handles a resolved `null` (signed out). Keep
  tolerating a `401` silently (old backend during a deploy skew window).
- Tests: AuthProvider "resolved null → signed out, no log/console error";
  update mocks that model signed-out as a rejected 401 where it reads
  better to use `null`.
- e2e: add a check that signed-out `/auth/me` is `200` + `null`.

Docs: `docs/api.md`, `docs/onboarding.html`.

## Log

- 2026-11-12 10:00 — Created plan
- 2026-11-12 15:30 — Backend: `GET /auth/me` moved to `handleOptional`; `Me` reads the context user and writes `200 null` when absent. Route-coverage entry → `authOptional`. Unit tests for `Me`; integration `TestAuth_Me`/`TestAuth_Logout` cover no cookie, bogus cookie, after-logout (all 200 null) and that `/users/me/calendars` still 401s.
- 2026-11-12 15:45 — Frontend: `getMe(): Promise<User | null>`; AuthProvider keeps a silent 401 fallback for deploy skew. Tests for 200-null signed-out (no log/console error) and `getMe` parsing; signed-out mocks switched from rejected 401 to resolved null. New e2e `auth-me-signed-out.test.ts` (200 null on page load, no 401 console error; bogus cookie → 200 null).
- 2026-11-12 15:55 — Docs: `docs/api.md`, `docs/onboarding.html` updated. Status → done.
