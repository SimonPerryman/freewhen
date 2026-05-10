---
title: Create CTA refresh — color, label, icon
---

# Create CTA refresh

Status: done

## Goal

The homepage Create button currently uses a near-white fill that pulls focus from the headline, an "Sign in to create →" label that surfaces auth plumbing in the CTA, and a generic `→` icon. Tighten all three so the CTA reads as the page's payoff rather than a system control.

## Decision

- **Color**: coral fill — `--accent` (light) / `--dk-terra` (dark), hover `--accent-hover` / `#f0917e`. Text uses `--accent-text` (`#fff` light / `--dk-bg` dark).
- **Label**: always `Create` (or `Creating…` while submitting). No auth-conditional copy. Existing 401 → `/login` redirect handles logged-out users.
- **Icon**: none.

## Plan

1. **POC mock** — `plans/ui/mocks/create-cta-variants.html`. Compared coral/outline/cream × no-icon/`↗`/`→` in light + dark hero contexts. ✅
2. **Pick winners** — coral · no icon (above). ✅
3. **Implement** in `frontend/src/app/page.tsx` + `page.module.css`:
   - Drop the auth-conditional label. Button always says `Create` (or `Creating…` while submitting). Existing 401 → `/login` flow in `handleSubmit` already covers the unauth case, so no behavioral change is needed.
   - Update `.button` background/color to the chosen token (likely `--accent` / `--accent-text`, with `--accent-hover` on hover). Keep both light and `prefers-color-scheme: dark` branches consistent.
   - Update icon per the pick (likely remove it).
4. **Verify** — `make lint`, `make test-frontend`, eyeball the homepage in light + dark on mobile (375px) and desktop.

## Out of scope

- Other CTAs across the app (calendar pages, share button, etc). If the coral-CTA pattern wins we can do a follow-up sweep, but this plan is just the homepage hero.
- Auth-flow UX changes. The 401 redirect stays as-is.

## Log

- 2026-05-10 — Created plan + POC mock at `plans/ui/mocks/create-cta-variants.html`.
- 2026-05-10 — Decided: coral fill, label `Create`, no icon. Implementing on `ui/create-cta-coral`.
- 2026-05-10 — Implemented. `page.tsx`: collapsed `buttonLabel` to `submitting ? "Creating…" : "Create"`. `page.module.css`: `.button` now uses `--accent`/`--accent-text` with `--accent-hover` on hover; removed the `prefers-color-scheme: dark` override (tokens already invert). Updated the unauth-redirect test to match new label. `make lint` + `make test-frontend` clean (257/257).
