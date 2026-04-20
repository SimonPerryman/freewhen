# Fix: duplicate PUT /availability on every toggle

Status: done

## Goal

Stop sending two identical `PUT /calendars/:id/availability` requests for every single toggle or range/quick-action click. Each user action should produce one network write (coalesced with any still-pending writes, as today).

## Symptoms

In dev, every tap on a day cell, every range commit, and every Weekends/Weekdays click results in two back-to-back PUTs with identical payloads (confirmed via network log on mobile Safari and desktop Chrome). The second request is redundant — same participant, same dates.

Prod impact is the same code path, but StrictMode is dev-only — so prod sends one request. The dev-only double-fire still surfaces real bugs (e.g. retry/echo handling) and wastes debug time.

## Root cause

`setSelectedDates` is called with an updater function in two places (`CalendarPageClient.tsx`):

- `handleToggle` (line ~270) — single-tap path.
- `handleRangeToggle` (line ~297) — range + quick-action path.

Both updaters call `queueSave(Array.from(next))` **inside** the reducer body. React 18/19 StrictMode invokes state updater functions **twice** in development to check for purity — and our updater has a side effect. The trace:

1. First reducer call → `pendingDatesRef = dates`; `savingRef` is false → `flushSave()` starts, sets `savingRef = true`, clears `pendingDatesRef`, awaits the first PUT.
2. React re-invokes the reducer (StrictMode purity check) → `pendingDatesRef = dates` again; `savingRef` is now true → no new `flushSave` spawned.
3. First PUT resolves; `flushSave`'s `while` loop sees the re-queued `pendingDatesRef` and fires a second PUT.
4. Second PUT resolves; loop exits; `savingRef = false`.

This is a straightforward "side effect in a setState updater" bug. Updater functions must be pure.

## Plan

1. **Add a `selectedDatesRef`** in `CalendarPageClient` that mirrors `selectedDates` via a one-line effect (`useEffect(() => { selectedDatesRef.current = selectedDates }, [selectedDates])`).
2. **Refactor `handleToggle`** to:
   - Read current set from `selectedDatesRef.current`, compute `next` outside the reducer.
   - Call `setSelectedDates(next)` with the computed value (not a function).
   - Call `queueSave(Array.from(next))` after the setState.
   - Move the hidden-view gate-unlock side effect (the `firstSubmitRef` / `setTimeout` block) out of the reducer too.
3. **Refactor `handleRangeToggle`** the same way.
4. **Add a regression test** in `CalendarPageClient.test.tsx`: render under `<StrictMode>` (or simulate by spying on `setAvailability`), fire one toggle, assert `setAvailability` was called exactly once. Do the same for a range commit and a Weekends click.
5. **Verify manually** on mobile Safari + desktop Chrome that the network tab shows a single PUT per interaction after the fix.

## Non-goals

- Changing the debounce/coalesce semantics of `queueSave` / `flushSave`. The serialized-save pipeline stays exactly as it is.
- Changing the API shape (still a full-state `dates[]` array).
- Touching the range-tap UX (discoverability, labels, hints) — separate concern.

## Out of scope / follow-ups

- `handleFinalise`, `handleUnfinalise`, `handleVisibilityToggle`, `handleRenameCommit`, `handleRemoveParticipant`, `handleJoin` — these call `setCalendar` with updater functions but no side effect inside the updater (side effects are in the surrounding async code). Skim during the fix to confirm none have the same smell; fix if any do.

## Log

- 2026-04-20 HH:MM — Created plan. Root-caused to side effect (`queueSave`) inside `setSelectedDates` updater; StrictMode double-invoke produces the duplicate PUT. Fix is mechanical: hoist side effect out of reducer.
- 2026-04-20 19:43 — Implemented: added `selectedDatesRef` mirroring `selectedDates`; rewrote `handleToggle` and `handleRangeToggle` to compute `next` from the ref and call `setSelectedDates(next)` with a value (not an updater); moved the hidden-view gate-unlock side effect out of the reducer. Confirmed the out-of-scope `setCalendar` callers (finalise/unfinalise/visibility/rename/remove/join) have no side effects inside their updaters. Added two `<StrictMode>`-wrapped regression tests (single toggle + range toggle) asserting exactly one `setAvailability` call per interaction. All 16 CalendarPageClient tests pass.
