# Multi-Select / Range Tap

Status: done

## Goal

Let a participant mark many consecutive days free (or not free) without tapping each cell individually. Marking 14 days of availability on a phone currently takes 14 precise taps — a long, fiddly, error-prone task on the primary device we optimise for.

From `docs/future-ideas.md`:
> Multi-select / range tap — tapping 14 consecutive days on mobile is tedious; drag-to-select or "select all weekends" would speed things up

## Context

- Availability is a set, persisted with `PUT /calendars/:id/availability` sending the full `dates[]` (`backend`/frontend already batch-aware — see `CalendarPageClient.tsx:232-266`, `flushSave` + `queueSave`).
- `CalendarGrid` currently owns: tap-to-toggle (`handleDayClick`), a 300ms visual pulse, and a horizontal swipe gesture for week nav (`handleTouchStart`/`handleTouchEnd`, threshold 50px).
- Past dates render as non-interactive placeholders; finalised calendars disable toggling.
- Saving is already debounced/serialised through `pendingDatesRef`, so multi-toggle simply needs to call `onToggle` (or an equivalent batch callback) and the existing pipeline coalesces writes.

## Design Decisions

### 1. Interaction model — **A: two-tap range**

First tap enters "range pending" (visual highlight on start cell). Second tap completes the range — all cells between (inclusive) are set to match the intended target (see §2).

Options B (long-press drag) and C (click-drag / long-press-drag) were rejected primarily because both collide with the existing horizontal-swipe week-nav gesture on `CalendarGrid` (`handleTouchStart`/`handleTouchEnd`, 50px threshold). A also has the best a11y story (identical on mobile, desktop, keyboard) and is trivially unit-testable. If usage data later shows people want a faster inline gesture, layer B on top of the same `onRangeToggle` hook — no refactor needed.

**Entry point: persistent toolbar, not a modal "Range" button.** A transient mode toggle is less discoverable than a persistent control. Render three always-visible buttons above the week nav: `Range`, `Weekends`, `Weekdays`. The `Range` button uses `aria-pressed` to reflect whether range mode is active.

### 2. Semantics — **set-to-same-state, based on start cell at entry**

When range mode is entered, snapshot the start cell's current state. On commit:
- Start was "not free" → whole range becomes **free**.
- Start was "free" → whole range becomes **not free**.

The start cell itself is not toggled until commit; "state at entry" means the state before any range interaction, not after a preliminary toggle.

### 3. Quick actions — **Weekends / Weekdays, scoped to visible window**

Two buttons: `Weekends` and `Weekdays`.

- **Weekend definition:** Sat + Sun, independent of the user's start-of-week setting. Weekend is a cultural concept, not a grid artefact.
- **Scope:** the visible 5-week window only (match the `CalendarGrid`'s current view). Label the buttons with that scope in mind — users should not be able to silently touch offscreen dates.
- **Toggle rule:** if any matching visible cell is currently unselected, select all of them; otherwise deselect all. Matches the GitHub "select all" checkbox pattern — users have a mental model for it.
- Past/finalised cells are always excluded from the operation.

### 4. Scope of range

- Exclude past dates (same as today's tap behaviour).
- If a finalised date sits inside the range, skip it (cell is non-interactive anyway).
- If the calendar is locked/finalised globally, toolbar entry points are hidden/disabled (same rule as `joined && !locked`).

### 5. Visual affordance

- Persistent toolbar row above the week nav with three buttons. Must fit 375px — use short labels, or icons with labels (`↔ Range`, `Weekends`, `Weekdays`). No overflow menu: hiding the feature's main controls behind a menu defeats the discoverability goal.
- When in "range pending": start cell gets a persistent ring (`styles.rangeStart`); hovering/tapping subsequent cells previews the range with a lighter tint (`styles.rangePreview`); tap commits, tap-same-cell cancels, Esc/back cancels on desktop.
- Sticky hint (a live-region banner or toolbar replacement) while range-pending: *"Tap another date to finish, or tap the same date to cancel."* The ring alone is too subtle on mobile.

### 6. Interaction between range-pending and quick actions

If the user enters range mode, taps start, then taps `Weekends` or `Weekdays` — **cancel range mode and apply the quick action.** Simplest mental model; avoids surprising the user with a combined semantic.

## Plan

1. **Extend `CalendarGrid` API**: add optional `onRangeToggle(dates: string[], target: boolean)` prop alongside existing `onToggle`. Keep `onToggle` for single taps so tests and callers that don't opt in keep working.
2. **Range mode state in `CalendarGrid`**:
   - `rangeStart: string | null`, `rangeStartState: boolean` (snapshot of start cell's free/not-free at entry).
   - Persistent toolbar: first tap on the `Range` button activates range mode. First cell tap sets `rangeStart` + snapshots its state; second tap computes `[rangeStart..tap]` inclusive, filters past/finalised, and calls `onRangeToggle(dates, target)` with `target = !rangeStartState` (start was "not free" → target `true`; start was "free" → target `false`).
   - Escape/back/tap-start-again/quick-action-tap exits range mode.
3. **Quick actions** (`Weekends`, `Weekdays`): compute visible dates matching the predicate (future + non-finalised). If any matching cell is unselected → target `true`; otherwise target `false`. Call `onRangeToggle`. If range mode is pending when pressed, cancel range mode first.
4. **Parent wiring (`CalendarPageClient`)**: implement `handleRangeToggle(dates, target)` — mutates `selectedDates` set accordingly, calls existing `queueSave(Array.from(next))`. Reuses batching/retry path unchanged.
5. **Styles**: new `styles.rangeStart` (ring), `styles.rangePreview` (tint), `styles.toolbar`, `styles.toolbarHint` in `CalendarGrid.module.css`. Mobile-first — three toolbar buttons must fit 375px without wrapping (short labels or icon+label).
6. **Accessibility**:
   - `Range` button uses `aria-pressed` to reflect range mode.
   - Each day cell keeps its aria-label; when range mode active, render a live-region banner: "Tap another date to finish, or tap the same date to cancel."
   - Keyboard: Enter/Space on start cell sets start, on end cell commits (already works as cells are buttons).
7. **Analytics**: new events `availability_range_selected { count, direction }`, `availability_quick_action_used { action: "weekends" | "weekdays", direction }`.
8. **Tests**:
   - `CalendarGrid.test.tsx`: range-mode toggle enters/exits; start + end → `onRangeToggle` called with inclusive dates and derived target; cancellation flows (tap same, Esc, quick-action tap); past/finalised cells filtered; quick-action buttons compute correct visible sets with the "any unselected → select all, else deselect all" rule.
   - `CalendarPageClient.test.tsx`: range callback updates `selectedDates` and triggers one `setAvailability` call with the merged set (debounced, single network write).
9. **Docs**:
   - Mark Tier 2 row in `docs/v2-priorities.md` shipped once merged.
   - Note the interaction model in `docs/user-journeys.md` under the "mark availability" journey.

## Out of Scope

- Click-and-drag on desktop and long-press-drag on mobile (option B/C). If we add later, `onRangeToggle` is the extension point — no refactor needed.
- Saved "preferred pattern" (e.g. "I'm always free on Wednesdays") — that's a user-preference feature, not a selection gesture.
- Range selection for owner finalise/admin actions.

## Log

- 2026-04-20 HH:MM — Created plan. Pending design-decision confirmation from owner before implementation.
- 2026-04-20 HH:MM — Design decisions confirmed: two-tap range (A), set-to-same-state, persistent toolbar (no modal entry), Weekends=Sat+Sun independent of week start, visible-5-week scope for quick actions, "any unselected → select all else deselect all" toggle rule, quick-action tap cancels pending range. Ready to implement.
- 2026-04-20 19:01 — Implemented. Added `onRangeToggle`/`selectedDates` props to `CalendarGrid`; persistent toolbar (`Range`, `Weekends`, `Weekdays`) with `aria-pressed` on Range; range-pending preview + live-region hint; Escape/tap-start-again/quick-action cancellation. Wired `handleRangeToggle` in `CalendarPageClient` to mutate `selectedDates` and call existing `queueSave`. Added `styles.toolbar`, `styles.rangeStart`, `styles.rangePreview`, `styles.toolbarHint`. Analytics: `availability_range_selected` and `availability_quick_action_used`. Tests: 8 new cases in `CalendarGrid.test.tsx` + 1 integration case in `CalendarPageClient.test.tsx`. Docs: Tier 2 row in `docs/v2-priorities.md` marked shipped; `docs/user-journeys.md` "mark availability" journey documents the toolbar. Pre-existing date-flake on `past dates render as non-interactive placeholders` (today is Monday → visible grid has no past cells) left untouched; unrelated to this work.
- 2026-04-20 HH:MM — Decision reversal on the Range button. In use it proved non-intuitive: the label "Range" tells you *what* it does but not *how* (two taps), and the live-region hint only appears after you've already pressed it. No widely-known mobile pattern matches (Google / Apple / Outlook Calendar don't do range-select at all; When2meet uses drag, which we rejected for swipe-conflict reasons; iOS Photos uses a "Select" mode but for multi-select, not range). Plan: remove the Range button and its associated state (`rangeMode`, `rangeStart`, `rangeStartSelected`, `rangeHover`, `rangePreviewSet`, `handleRangeButton`, `handleRangeCellClick`, `exitRangeMode`, Escape handler, toolbar hint, `rangeStart`/`rangePreview` styles). Keep the `Weekends` / `Weekdays` quick-action buttons — they remain discoverable and useful. Keep `onRangeToggle` as an extension point and keep the `availability_range_selected` analytics event retired (only `availability_quick_action_used` stays active). Update `docs/user-journeys.md` to drop the two-tap range mention. Tests covering range-mode flows get deleted; quick-action tests stay. Pending implementation.
- 2026-04-20 19:45 — Removed the Range button and all associated state/handlers/styles from `CalendarGrid.tsx` + `CalendarGrid.module.css`. Dropped `useEffect` import (no longer used). Toolbar now contains just `Weekends` and `Weekdays`; switched layout to `justify-content: center` with natural-width buttons (removed `flex: 1`, widened padding to `6px 14px` for tap target). Removed unused `.toolbarButtonActive`, `.toolbarHint`, `.rangeStart`, `.rangePreview` CSS. Deleted 5 range-mode tests in `CalendarGrid.test.tsx`; updated toolbar-rendering test to stop asserting the Range button. Updated `docs/user-journeys.md` §5 to describe the two-control toolbar. 18 of 19 `CalendarGrid` tests pass (the pre-existing "past dates render as non-interactive placeholders" flake still fails when today is Monday, unchanged by this work); all 16 `CalendarPageClient` tests pass. Status → done.
