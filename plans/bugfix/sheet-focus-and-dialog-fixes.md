# Sheet focus and dialog fixes

Status: done

## Goal

Fix four issues a full-site e2e run found in the modal sheets/dialogs:

1. Escape on a confirm dialog opened from the Settings sheet also closes the
   Settings sheet — every `useFocusTrap` listens for keydown on `document`.
2. Focus isn't restored to the opener when a sheet closes (lands on `<body>`).
3. `DuplicateCalendarSheet` has no ✕ close button, unlike the other sheets.
4. Dark mode: the disabled primary Button (e.g. DayDetail "Save") looks
   enabled — `opacity: 0.6` over a bright accent still reads as a live fill.

## Plan

- `useFocusTrap`: keep a module-level stack of active traps; only the topmost
  handles Escape and Tab. Restore focus on deactivation only when the trap
  was topmost and the opener is still connected.
- Find why the opener isn't captured (focus already on `<body>` when the trap
  mounts) and fix the cause.
- `DuplicateCalendarSheet`: pass `closeLabel` to `Sheet`.
- `Button`: replace the opacity-only disabled style with explicit disabled
  tokens (recessed fill + muted text + border) defined for light and dark in
  `globals.css`.
- Tests: unit tests for trap stacking + focus restore and Button disabled;
  e2e: Escape on confirm leaves Settings open, focus returns to the opener.
- Screenshots at 375px light+dark of Settings+confirm, Duplicate, DayDetail.

## Log

- 2026-11-12 18:45 — Created plan
- 2026-11-12 18:52 — Reproduced bugs 1 and 2 with a Playwright probe. Bug 2's
  root cause: the lazy overlays (`next/dynamic` with SSR on and no `loading`)
  have no Suspense boundary of their own, so the first open suspends up to
  the route's `loading.tsx` — the page is hidden, the opener blurs, and the
  trap captures `<body>` as the element to restore. Later opens worked.
- 2026-11-12 18:55 — Gave DayDetail/Duplicate/owner overlays
  `{ loading: () => null }` (own Suspense boundary); added the trap stack,
  connected-only restore, and pulling Tab back into the top trap if focus
  escaped; ✕ on the duplicate sheet; disabled Button tokens.
- 2026-11-12 19:00 — Unit tests (trap stacking/restore, disabled tokens +
  ≥3:1 label contrast in both themes) and e2e assertions in
  create-calendar / submit-availability.
- 2026-11-12 19:05 — Screenshots showed the first dark disabled palette still
  read like a secondary button; dimmed the dark tokens (fill #211d19, label
  #736a5f, 3.15:1).
- 2026-11-12 19:15 — Gate green locally: vitest 447/447, tsc, oxlint, oxfmt
  (changed files), next build, Playwright 13/13. Done.
