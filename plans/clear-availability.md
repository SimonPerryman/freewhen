# Clear Availability

Status: done

## Goal

Let a participant wipe all their availability in one action and start over, instead of tapping every selected cell off individually (or using Weekdays/Weekends twice with the right starting state).

From `docs/future-ideas.md` (Participant QoL):
> Undo / clear all availability and start over

From `docs/v2-priorities.md` Tier 3:
> Clear availability — Small QoL.

## Context

- Availability is stored client-side as `selectedDates: Set<string>` in `CalendarPageClient.tsx:81`, persisted via `queueSave(Array.from(next))` which debounces/coalesces writes to `PUT /calendars/:id/availability` (see `flushSave`/`queueSave` at `CalendarPageClient.tsx:313-363`).
- Per-date notes live in `notesByDate` and are deleted whenever a date is deselected (`handleRangeToggle` at `CalendarPageClient.tsx:477-486`). Clearing availability must also clear notes for the same dates.
- The existing quick-action toolbar (`Weekends`, `Weekdays`) lives above the week nav in `CalendarGrid` and is the natural home for a `Clear` action. Toolbar visibility already follows the `joined && !locked` rule.
- Past-date cells are non-interactive and their availability is preserved server-side; `Clear` should only touch future (editable) dates, matching the semantics of every other participant-side mutation.
- Backend accepts the full `dates[]` set on each PUT — no new endpoint needed; clearing is just a save with the future dates removed.

## Design Decisions

### 1. Scope — **future dates only, whole calendar (not just visible window)**

`Clear` wipes every editable (future, non-finalised) date the user has marked, across the entire calendar, not just the currently-visible 5-week window. Rationale: the *feature* users ask for is "start over"; scoping to the visible window would leave stale selections a week away that the user can't see, which is exactly the frustration this is meant to solve. The Weekends/Weekdays actions are window-scoped because they're *additive selection helpers*; `Clear` is a reset.

Past dates are preserved (same rule as every other edit path). Finalised dates are skipped if the calendar is finalised globally the toolbar is hidden anyway.

### 2. Placement — **third button in the existing toolbar**

Add `Clear` next to `Weekends` / `Weekdays`. Keeps all availability-wide actions in one place. Three short buttons still fit 375px (current two buttons use `justify-content: center` with natural width — verify the third fits; fall back to slightly tighter padding if needed).

Button is **disabled** when the user has no future selected dates (nothing to clear). Disabled state is important feedback — it answers "did it work?" for a user who taps it on an empty calendar.

### 3. Confirmation — **post-action undo toast, no pre-action confirm**

Clearing fires immediately and shows a toast: `"Cleared 12 dates. Undo"` with a 6s auto-dismiss. Tapping `Undo` restores the previous `selectedDates` set and `notesByDate` entries, then triggers a new `queueSave` with the restored set.

Rationale:
- `window.confirm` on mobile is a system dialog that breaks the PWA aesthetic (on iOS it shows the domain, looks like an error), and it's a pre-commit prompt for a reversible action — the wrong shape.
- Undo toast is the standard mobile pattern (Gmail, iOS Mail, Linear) and solves "did I mean to do that" + "can I get it back" with one element.
- No existing toast primitive — we build a minimal one (§7). It has at least two other near-term reuses (admin "remove participant", future send-status feedback from `plans/notifications.md`).

Edge case: if the user triggers another availability mutation while the toast is live, the `Undo` would restore stale state. Simplest resolution: the toast's `Undo` handler snapshots the pre-clear set at toast-creation time and unconditionally sets that as the new selection. If the user made intervening edits, `Undo` wins (it's an explicit action). Document in tests.

### 4. Visual grouping — **Clear separated from the selection helpers**

`Weekends`/`Weekdays` are *additive selection helpers*; `Clear` is a *reset*. Rendering three identical buttons implies they're equivalent.

Approach: push `Clear` to the far right of the toolbar with `margin-left: auto`, and give it a subtle de-emphasis — outline-only (no filled background), muted text colour. Not red: clearing is reversible via the toast, so destructive-red overstates it. The visual hierarchy should read as "two primary helpers, one secondary reset."

Verify the three buttons fit 375px before merge — current buttons use natural width with `padding: 6px 14px`. If tight, tighten Clear's padding rather than all three.

### 5. Notes — **cleared alongside dates**

Any per-date note on a cleared date is removed from `notesByDate` (same pattern as `handleRangeToggle` when `target === false`). Server-side notes for those dates disappear implicitly because the PUT replaces the date set. `Undo` restores these notes from the snapshot.

### 6. `isFutureAndEditable` definition

A date is clearable if it is **today or later** (in the user's local timezone, matching how the grid computes past-vs-future) **and** not an individually-finalised cell. Today is clearable — "future-only" in this doc means "not before today," not "strictly after today."

### 7. Toast primitive — **minimal, single-slot, imperative**

New component `src/components/Toast/Toast.tsx` + context in `src/lib/toast.tsx` (or similar). Deliberately small:
- **Single slot** — one toast at a time. A second `showToast()` call replaces the current one. No queue, no stacking. (If we ever need stacking, that's a refactor, not a regression.)
- **Imperative API**: `const { showToast, dismiss } = useToast()`. `showToast({ message: string, action?: { label: string, onClick: () => void }, durationMs?: number })`. Default duration 6000ms; action makes the toast stay until dismissed or clicked.
- **Provider** mounted once near the app root (likely `src/app/layout.tsx` or the calendar page shell — pick the tightest scope that covers everywhere a toast might fire; start with layout if there are no SSR concerns).
- **Visuals**: fixed bottom-centre on mobile (16px from bottom, safe-area inset aware), bottom-right on desktop (≥768px). Opaque background, subtle shadow, enter/exit via `transform: translateY(...)` + opacity transition. No toast library — ~80 lines of code.
- **A11y**: `role="status"` with `aria-live="polite"` for neutral messages; the action button is a real `<button>` with an accessible label. Auto-dismiss timer pauses on hover (desktop only; not worth the complexity on touch).
- **Tests**: primitive tests in `Toast.test.tsx` — show/replace/dismiss, action click fires callback + dismisses, auto-dismiss timer, hover pause.

Explicitly *not* included: queueing multiple toasts, severity variants (success/error/info colouring), close `×` button, toast positioning config. These can be layered on when the second or third consumer actually needs them.

### 8. Analytics

- `availability_cleared { count, notes_cleared }` — `count` is dates removed, `notes_cleared` is how many of those had a note attached (losing a typed note is higher user cost than losing a tap; worth being able to distinguish).
- `availability_clear_undone { count }` — fires when the user taps `Undo`. Ratio of undo to clear is the clearest signal of whether the toast pattern is serving users.

## Plan

1. **Toast primitive** (build first — other steps depend on it):
   - Create `src/components/Toast/Toast.tsx`, `Toast.module.css`, and `src/lib/toast.tsx` with the provider + `useToast` hook (imperative API described in Design §7).
   - Mount `<ToastProvider>` in `src/app/layout.tsx` (or the calendar page shell if there's a layout-level concern — decide when wiring).
   - Tests in `Toast.test.tsx`: show, replace, dismiss, action click, auto-dismiss timer, hover pause on desktop.
2. **`CalendarGrid` toolbar**: add a third button `Clear` in `CalendarGrid.tsx`. Props: new optional `onClear?: () => void` and `canClear: boolean`. Render `Clear` last, separated visually (`margin-left: auto`, outline-only style — see Design §4). `aria-disabled` + `disabled` when `!canClear`.
3. **`CalendarPageClient.handleClearAvailability`**:
   - Compute `cleared = [...selectedDatesRef.current].filter(isFutureAndEditable)`. If empty, no-op (button should already be disabled; safety net).
   - Snapshot `prevSelected = new Set(selectedDatesRef.current)` and `prevNotes = new Map(notesByDate)` **before** mutating.
   - Build `next` without the cleared dates; update `selectedDatesRef` + `setSelectedDates`. Remove the cleared dates from `notesByDate`.
   - `queueSave(Array.from(next))`.
   - Fire `availability_cleared { count: cleared.length, notes_cleared: <count of cleared dates that had a note> }`.
   - `showToast({ message: pluralise(cleared.length), action: { label: "Undo", onClick: () => undoClear(prevSelected, prevNotes) } })`.
4. **`undoClear(prevSelected, prevNotes)`**:
   - Unconditionally restore `selectedDatesRef`/`setSelectedDates` from `prevSelected` and `setNotesByDate(prevNotes)`. Intervening edits lose (documented trade-off — see Design §3).
   - `queueSave(Array.from(prevSelected))`.
   - Fire `availability_clear_undone { count: prevSelected.size - next.size }` (the number of dates restored).
5. **`canClear` derivation**: `selectedDates.size > 0 && any selected date matches isFutureAndEditable`. Compute once per render (small set).
6. **Styles**: new `styles.toolbarButtonSecondary` on `CalendarGrid.module.css` for the outline-only Clear variant + `margin-left: auto` on the Clear button's wrapper.
7. **Tests**:
   - `CalendarGrid.test.tsx`: Clear renders; disabled when `canClear === false`; enabled otherwise; click calls `onClear`; Clear is visually separated (class assertion sufficient).
   - `CalendarPageClient.test.tsx`: clicking Clear → future dates + notes removed, one `setAvailability` call with remaining set, past dates preserved, toast shown with `Undo` action. Undo → state + notes restored, second `setAvailability` call with the pre-clear set. Intervening toggle between Clear and Undo → Undo still restores to pre-clear snapshot (documents the trade-off).
8. **Docs**:
   - Mark Tier 3 row in `docs/v2-priorities.md` shipped.
   - Mention in `docs/user-journeys.md` "mark availability" journey.
   - Add a short note in `AGENTS.md` under Frontend Conventions pointing future feature work at the toast primitive instead of `window.confirm` or new inline banners.

## Out of Scope

- Clearing per-range or per-week scope — the Weekends/Weekdays toggle-off path already covers that.
- Server-side "clear" endpoint — unnecessary; the existing PUT handles it.
- Admin clearing another participant's availability — different feature (falls under admin controls).
- Toast features beyond the single-slot imperative primitive: queueing, severity variants, manual-close `×`, positioning config. Layer on when a second consumer actually needs them.
- Migrating existing `window.confirm` usages (e.g. admin remove-participant) to the new toast pattern. Worth doing, but a separate follow-up so the clear-availability plan stays focused.

## Log

- 2026-04-21 HH:MM — Created plan. Initial design: `window.confirm` for safety, plain-styled third toolbar button, whole-calendar scope.
- 2026-04-21 HH:MM — Design review feedback: `window.confirm` breaks the PWA aesthetic on mobile and is the wrong shape for a reversible action. Swapped to post-action undo toast. No existing toast primitive in the codebase, so added a minimal single-slot imperative toast as step 1 of the plan (reusable by admin remove-participant and future notifications work). Also: Clear button visually separated from Weekends/Weekdays (outline-only, pushed right) since it's a reset rather than a selection helper; added `notes_cleared` dimension to analytics; defined `isFutureAndEditable` as today-or-later; documented the Undo-wins-over-intervening-edits trade-off. Pending owner sign-off before implementation.
- 2026-04-21 14:25 — Implemented end-to-end. Added `src/components/Toast/Toast.tsx` + `src/lib/toast.tsx` (single-slot imperative primitive, 6s default, hover-pause, action-click callback) and mounted `<ToastProvider>` in `src/app/layout.tsx`. Added a third `Clear` button to the `CalendarGrid` toolbar (secondary style via `toolbarButtonSecondary`, `margin-left: auto`), disabled when no future dates are selected. `CalendarPageClient` now exposes `handleClearAvailability` with a pre-clear snapshot of `selectedDates` + `notesByDate` for the `Undo` action, fires `availability_cleared { count, notes_cleared }` and `availability_clear_undone { count }`. Tests: `Toast.test.tsx` (show/replace/dismiss/auto-dismiss/action-click/hover-pause), CalendarGrid Clear render/disable/click/visual-class, CalendarPageClient clear-with-undo and empty-canClear paths. Docs: marked shipped in `docs/v2-priorities.md`, added Clear to `docs/user-journeys.md` §5, added Frontend Conventions note in `AGENTS.md` pointing future work at the Toast primitive.
