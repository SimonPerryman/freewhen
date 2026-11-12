# E2E verify: full-site browser pass

Status: done

## Goal

Drive the whole site in a real browser after today's merges (participant
tokens/authz, server-rendered calendar page, CalendarPageClient split,
vote-toggle fix, enforced CSP, `/auth/me` 200-null, 12-char IDs + rate limit,
design tokens + Button/TextField, shared Sheet primitive) at 375px and 1280px,
light and dark, and persist the invariants not yet covered by `frontend/e2e`.

## Plan

1. Start the e2e stack by hand (e2e-server on :8090, `next build && next start` on :3100).
2. Explore with throwaway Playwright scripts (outside the repo): landing/login
   signed out, create calendar, anonymous join + picks + note + rapid vote
   toggles + reload, magic-link claim, owner sheets (settings, rename,
   duplicate with/without participants, day detail, confirm dialogs),
   passphrase gate, finalise, .ics / Google export. Screenshot QA each screen.
3. Persist into the closest existing specs:
   - `submit-availability`: rapid vote toggles settle to the last state;
     day-detail focus stays in the sheet; Escape closes it.
   - `create-calendar`: settings sheet focus + Escape; rename persists.
   - `finalise`: passphrase gate (wrong → error, right → unlock), export
     refused without access, `.ics` link carries `access=` and downloads.
4. Run the full Playwright suite; report findings (no product fixes here).

## Log

- 2026-11-12 17:05 — Created plan; stack up on :8090/:3100, healthy.
- 2026-11-12 17:25 — Exploration done at 4 viewport/scheme combos; all core flows pass. Findings reported in the PR (no product fixes in this PR).
- 2026-11-12 17:35 — Added spec lines to submit-availability, create-calendar, finalise. Rapid vote toggles need a poll before reload (saves are serialized client-side).
- 2026-11-12 17:40 — Full Playwright suite green (13/13).
