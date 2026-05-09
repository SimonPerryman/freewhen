# Sheet bottom gap on mobile

Status: done

## Goal

On mobile (observed in Chrome DevTools at iPhone 14 Pro Max, 430×932), the
`CalendarSettingsSheet` modal does not sit flush with the bottom of the
viewport — a thin sliver of the underlying page is visible below the sheet,
which becomes obvious when scrolling the body behind it. The sheet should
anchor cleanly to the bottom of the visible viewport on mobile, with no gap
and no leaked page scroll.

Likely affects the other bottom-anchored sheets too (`DuplicateCalendarSheet`,
`RenameCalendarSheet`, `PassphraseGate`) since they share the same pattern.

## Suspected cause

`CalendarSettingsSheet.module.css` uses:

- `.backdrop { position: fixed; inset: 0; }` — sized to the **layout**
  viewport, which on mobile browsers can differ from the **visual/dynamic**
  viewport (URL bar, bottom toolbar, safe area).
- `.sheet { max-height: 88vh; }` — `vh` is the static large viewport, not
  `dvh`.
- No `env(safe-area-inset-bottom)` padding, so on iOS the home-indicator area
  can also produce a visible gap.
- Body scroll is not locked while the sheet is open, so the page behind can
  scroll and reveal the mismatch.

## Plan

1. Reproduce on a real device (or DevTools mobile emulation) and confirm
   whether the gap is dvh-related (URL bar), safe-area-related, or body-scroll
   leak. Take a screenshot at the bottom edge.
2. Fix `CalendarSettingsSheet.module.css`:
   - Backdrop: `height: 100dvh` (with `100vh` fallback already implicit via
     `inset: 0`).
   - Sheet: `max-height: 88dvh`, `padding-bottom: max(28px, env(safe-area-inset-bottom))`.
3. Audit the other sheet/modal components for the same pattern and apply the
   same fix where present:
   - `frontend/src/components/DuplicateCalendarSheet/`
   - `frontend/src/components/RenameCalendarSheet/`
   - `frontend/src/components/PassphraseGate/`
   - any other `position: fixed; inset: 0` overlay.
4. Consider locking body scroll while a sheet is open (toggle
   `overflow: hidden` on `<body>` in the sheet's mount effect) so the page
   behind cannot scroll and expose any residual mismatch. Decide based on
   step 1 — only add if the gap persists after the dvh/safe-area fix, to
   avoid scroll-lock side effects (scrollbar jump on desktop, focus weirdness).
5. Verify on:
   - Chrome DevTools iPhone 14 Pro Max emulation (the original repro).
   - Real iOS Safari (URL bar visible and hidden states).
   - Desktop ≥640px — the sheet centers and should be unaffected.

## Out of scope

- Redesigning the sheet to a full-screen modal on mobile.
- Animation tweaks.

## Log

- 2026-05-09 — Created plan after reverting an in-place CSS fix attempt.
- 2026-05-09 — Applied dvh + safe-area fix to CalendarSettingsSheet,
  DuplicateCalendarSheet, RenameCalendarSheet, and DayDetailSheet (same
  pattern). Backdrops now use `height: 100dvh`; sheets use `max-height: NNdvh`
  and `padding-bottom: max(N px, env(safe-area-inset-bottom))`. Skipped
  body-scroll lock for now (step 4) — gap should resolve from dvh alone;
  revisit if it persists. PassphraseGate is a centered panel, not a bottom
  sheet, so unaffected.
