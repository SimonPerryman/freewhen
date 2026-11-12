# Frontend review fixes

Status: done

## Goal

Address findings from a read-only frontend review: a focus-jump bug in
`useFocusTrap`, undefined CSS tokens, dark-mode-breaking inline styles,
duplicated API base URL, toast accessibility, and thin test coverage.

## Plan

1. `useFocusTrap`: hold `onEscape` in a latest-ref so a fresh arrow per render
   no longer tears down the trap (focus jumped back to the name input when
   toggling "include participants" in DuplicateCalendarSheet). Add tests.
2. Replace undefined `var(--danger, …)` with the real `--error` token (defined
   for light and dark).
3. Unsubscribe page: CSS module with tokens; add `unsubscribe()` to `lib/api.ts`.
4. Single `API_BASE_URL` export in `lib/config.ts`, used by api.ts,
   useCalendarEvents.ts, ExportButtons.tsx.
5. Toast: keep the `role="status"` live region mounted permanently so
   announcements fire; pause auto-dismiss on focus-within as well as hover.
6. `error.tsx` / `not-found.tsx`: CSS module using tokens (drop nonexistent
   `--font-geist-sans`).
7. Delete `lib/example.test.ts`; AGENTS.md/onboarding say jsdom but vitest
   uses happy-dom — fix docs.
8. Tests for ConfirmDialog and RenameCalendarSheet.

## Log

- 2026-11-12 10:00 — Created plan; verified all eight findings against the code
- 2026-11-12 10:30 — useFocusTrap latest-ref fix + tests (re-render test fails on the old hook, passes now); `--danger` → `--error` in 4 CSS modules
- 2026-11-12 10:40 — `lib/config.ts` `API_BASE_URL`; `unsubscribe()` in api.ts (returns `res.ok`, since backend replies with HTML); unsubscribe/error/not-found moved to token-based CSS modules
- 2026-11-12 10:45 — Toast: `ToastRegion` live region always mounted by the provider; focus-within pauses auto-dismiss; tests added
- 2026-11-12 10:50 — ConfirmDialog + RenameCalendarSheet tests; removed example.test.ts; docs say happy-dom (not jsdom). oxlint, oxfmt, vitest (Node 24) all green
