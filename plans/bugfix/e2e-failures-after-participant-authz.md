# E2E failures after participant authorization

Status: done

## Goal

Two Playwright specs fail on `main` since participant writes became
authorized by session user or `X-Participant-Token` (#74) and owner authz
moved into the service (#77/#79):

- `claim-participant` — the claim request returns 403.
- `finalise` — the owner setting another participant's availability is
  rejected.

For each, decide whether the test leans on behaviour that was intentionally
removed (rewrite it to the intended flow) or whether the real UI flow is
broken (fix product code, with tests). The #74 security decisions stay: the
owner cannot write other participants' availability, and a claim requires the
participant's token.

## Plan

1. Reproduce both failures; drive the claim flow through the UI to see
   whether a browser that joined anonymously and then signed in actually
   claims its row.
2. Fix whatever the UI path gets wrong; add unit tests.
3. Rewrite the specs to exercise the intended flows (claim from the browser
   that joined; each participant sets their own availability with their own
   credentials).
4. Run the full e2e suite, frontend vitest + oxlint, backend tests if touched.

## Log

- 2026-11-12 10:00 — Created plan
- 2026-11-12 10:20 — Reproduced both. `finalise`: the owner PUTs Bob's
  availability with the owner session → 403. Intentional since #74 (owner has
  no override on others' availability) — test outdated.
- 2026-11-12 10:30 — `claim-participant`: the spec POSTs the claim from
  `page.request` without the participant token → 403. Claim needing the
  token is intentional, but driving the real UI (join anonymously → sign in →
  reload) showed the page *never* sends a claim at all: the load effect
  resolves identity with the `user` of the render that started it, and on a
  full page load (always, with the server-rendered calendar from #78) the
  calendar is ready before `/auth/me` answers, so identity resolves as
  signed-out. Same race skips reconnect-by-user. Real bug.
- 2026-11-12 10:45 — Fix: `CalendarPageClient`'s `onLoaded` awaits
  `checkAuth()` (joins the AuthProvider's in-flight /auth/me) and passes the
  settled user to `resolveFromLoad(data, signedInUser)`. Added SSR-page tests
  for late auth (silent claim + reconnect); both fail without the fix.
- 2026-11-12 10:55 — Rewrote `claim-participant` to claim through the UI
  (reload after sign-in, assert the page's claim request succeeds and the row
  shows "(you)"), plus a negative check that a signed-in user without the
  token gets 403. Rewrote `finalise` so each participant sets their own
  availability (owner by session, Bob with his join-issued token) and assert
  the owner gets 403 writing Bob's.
- 2026-11-12 11:10 — Full e2e suite 7/7 green (twice), vitest 413/413,
  oxlint clean. Status → done.
