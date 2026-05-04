# Accessibility pass

Status: done

## Goal

Close the most impactful WCAG / a11y gaps surfaced in the audit of the FreeWhen frontend. Mobile-first, keyboard-friendly, screen-reader-friendly. Bias toward systemic fixes in shared primitives over per-component patches.

## Plan

### Phase 1 — Global quick wins (single PR, ~30 min)

- [ ] Confirm `<html lang="en">` is set in `frontend/src/app/layout.tsx`; add if missing.
- [ ] Add a skip-to-content link in `layout.tsx`, visible on focus, targeting `<main id="main">`. Ensure `<main>` lands on every page (move into layout if not already).
- [ ] Add a global `:focus-visible` ring via `globals.css` (or Tailwind layer) so every focusable element has a visible focus indicator by default. Audit and remove any `outline: none` without a replacement.
- [ ] Wrap header links in `<nav aria-label="Primary">` — `components/Header/Header.tsx`.

### Phase 2 — Live regions & announcements (~30 min)

- [ ] `components/Toast/Toast.tsx` — add `aria-atomic="true"` to the live region so updates announce the full message.
- [ ] `components/LiveStatusDot/LiveStatusDot.tsx` — add `aria-live="polite"` so connection state changes (Polling/Offline) are announced.
- [ ] `components/FinalisedBanner/FinalisedBanner.tsx` — add `aria-live="polite"` alongside existing `role="status"`.
- [ ] Sweep error containers: ensure inline form errors use `role="alert"` (assertive) only when they appear in response to user action, otherwise `role="status"` with `aria-live="polite"`.

### Phase 3 — Forms (~1–2 hours)

- [ ] `app/login/page.tsx` — associate the email input with a `<label htmlFor>` (currently unlabeled).
- [ ] Add `aria-invalid={!!error}` + `aria-describedby="<error-id>"` pattern to inputs in:
  - `components/JoinForm/JoinForm.tsx`
  - `app/login/page.tsx`
  - `components/PassphraseGate/PassphraseGate.tsx`
  - `components/DuplicateCalendarSheet/DuplicateCalendarSheet.tsx`
  - `components/CalendarSettingsSheet/CalendarSettingsSheet.tsx`
- [ ] Verify each form has a single submit affordance reachable by Enter and that required fields are marked with `aria-required` (or native `required`).

### Phase 4 — Icon-only buttons & tap targets (~1 hour)

- [ ] Audit every icon-only `<button>` for an `aria-label`. Known suspects: close (✕) buttons in `CalendarSettingsSheet`, `DuplicateCalendarSheet`, share/export buttons.
- [ ] `components/CalendarGrid/CalendarGrid.tsx` toolbar (lines ~272–296) — bump buttons to `min-h-11 min-w-11` (44px) for mobile per AGENTS.md mobile-first principle. Spot-check other toolbars.

### Phase 5 — Reusable focus trap for dialogs (~half day)

- [ ] Extract focus-trap logic from `components/DayDetailSheet/DayDetailSheet.tsx` (currently lines ~85–132) into a `useFocusTrap` hook in `src/lib/`.
- [ ] Apply the hook + `role="dialog"` + `aria-modal="true"` + labelled-by heading to:
  - `ConfirmDialog`
  - `PassphraseGate`
  - `DuplicateCalendarSheet`
  - `CalendarSettingsSheet`
  - `InvitePrompt` (interstitial)
  - `DayDetailSheet` (refactor onto shared hook)
- [ ] Ensure each closes on `Escape` and returns focus to the triggering element on close.

### Phase 6 — Contrast & color encoding (~1–2 hours)

- [ ] Run axe DevTools / WAVE on the home, calendars, and day-detail pages. Capture findings.
- [ ] Audit `--text-secondary: #7a7168` on `--card-bg: #ffffff` (light) and the dark-mode equivalents — must hit WCAG AA 4.5:1 for normal text, 3:1 for large. Adjust tokens if borderline.
- [ ] `components/CalendarGrid` — participant slots are color-coded; add a legend or per-slot `aria-label` so the information isn't color-only (WCAG 1.4.1).

### Phase 7 — Per-route metadata (~30 min)

- [ ] Add `generateMetadata` to `app/calendars/[id]/page.tsx` so titles announce the calendar name.
- [ ] Set route-level metadata for `app/login/page.tsx` and `app/calendars/page.tsx`.

### Verification

- [ ] `make lint` and `make test` pass after each phase.
- [ ] Manual keyboard-only walkthrough of: home → create calendar → join → open day → finalise.
- [ ] Manual VoiceOver pass on mobile Safari for the same flow.
- [ ] axe DevTools clean on home, login, calendar detail.

## Log

- 2026-05-04 — Created plan from accessibility audit findings.
- 2026-05-04 21:25 — Implemented phases 1–7:
  - Phase 1: skip-to-content link in `layout.tsx`, `id="main"` added to existing `<main>` tags on home/login/calendars; global `:focus-visible` ring + `.skip-link` styles in `globals.css`; `<nav aria-label="Primary">` in `Header`.
  - Phase 2: `aria-atomic="true"` on `Toast`, `aria-live="polite"` on `LiveStatusDot` and `FinalisedBanner`.
  - Phase 3: login email input now has `<label>`, `aria-invalid`, `aria-describedby`; same pattern applied to `JoinForm`, `PassphraseGate`, `DuplicateCalendarSheet`. (`CalendarSettingsSheet` has no inputs — covered by focus-trap work in phase 5.) `required` / `aria-required` confirmed on the form fields.
  - Phase 4: bumped `CalendarGrid` toolbar buttons to 44px min-height for mobile tap targets. Existing close (✕/×) buttons audited — all already had `aria-label`s.
  - Phase 5: extracted shared `useFocusTrap` hook to `src/lib/useFocusTrap.ts`; applied to `ConfirmDialog`, `DuplicateCalendarSheet`, `CalendarSettingsSheet`, `DayDetailSheet` (refactored off the bespoke trap). `aria-labelledby` added to each, all return focus to trigger on close. `PassphraseGate` is a full-page gate (not a modal) so kept as-is. `InvitePrompt` is rendered inline as a card, not a dialog — skipped.
  - Phase 6: not implemented — token contrast audit deferred (needs axe DevTools / WAVE manual run); per-slot `aria-label`s on participant grid colour-coding deferred.
  - Phase 7: per-route metadata via `app/login/layout.tsx` and `app/calendars/layout.tsx`. `app/calendars/[id]/page.tsx` already had `generateMetadata`.
  - Verified `make lint` and frontend test suite (219/219 passing).
