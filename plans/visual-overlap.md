# Visual Overlap

Status: done

## Goal

Replace the lightweight colour-dot preview in the calendar grid (from step 7) with the real visual overlap: each day cell is divided into per-participant slots showing who is free, and days where everyone is free stand out clearly. This is step 8 of `v1-build-order.md`.

## Context

- Backend is complete. No API changes.
- Step 7 (`calendar-view.md`) is **done**. `CalendarGrid.tsx` ships with a placeholder: up to 4 colour dots for other participants (`otherAvailability.slice(0, 4)`) and the user's own selection painted as a full-cell background colour. Step 8 replaces both signals with a unified slot grid.
- Participants get a colour from a 10-colour palette, assigned by the backend in join order (see `backend/internal/store/participant.go`). Join/palette order is the stable sort key for slot positions.
- Grid is a rolling 5-week window; cell size is roughly constant across the grid.
- Participant count is bounded in practice but no hard cap — we design up to ~32.

## Plan

### 1. Slot layout function

Pure function `layoutSlots(n: number): { cols: number, rows: number }`:

- `n === 0` → `{ cols: 0, rows: 0 }` (cell renders date only; no slot container)
- `n ≤ 4` → `{ cols: 1, rows: n }` (full-width stripes stacked top-to-bottom)
- `n ≥ 5` → near-square: `cols = ceil(sqrt(n))`, `rows = ceil(n / cols)`

This collapses the old 2-col half-stripe tier into the near-square formula, which packs tighter. Examples: n=5 → 3×2 (1 empty), n=8 → 3×3 (1 empty), n=9 → 3×3 (0 empty), n=12 → 4×3 (0 empty), n=16 → 4×4 (0 empty), n=25 → 5×5 (0 empty), n=32 → 6×6 (4 empty). At n>32 the same formula applies — slots shrink but still render.

Packing rule: **row-major, top-to-bottom** (reads like text). Participant at index `i` (sorted by colour palette index, which mirrors join order) maps to `{ col: i % cols, row: floor(i / cols) }`. Deterministic and stable across every cell — the same participant always lives in the same slot position, so your eye can track them across the grid. First-to-join is always top-left.

Participant ordering: sort the `participants` array in the component by palette index. The palette is defined in the frontend as a constant array matching the backend's `colourPalette` order. Sort key: `COLOUR_PALETTE.indexOf(p.colour)` — unknown colours (e.g. palette grows or changes) sort to the end. This keeps the component robust to whatever order the API returns without requiring the backend to expose a numeric index.

```ts
const COLOUR_PALETTE = [
  "#E63946", "#457B9D", "#2A9D8F", "#E9C46A", "#F4A261",
  "#264653", "#A855F7", "#06B6D4", "#EC4899", "#84CC16",
];
```

### 2. Per-day classification

For each day in the visible window, compute:

- `freeParticipantIds: Set<string>` — from `CalendarWithDetails.availability`.
- `everyoneFree: boolean` — `participants.length >= 2 && freeParticipantIds.size === participants.length`.
- Silent participants (joined but never marked any day) count as "not free". No special handling.

### 3. Cell rendering

Retire both step-7 signals first:

- Remove the `.selected` CSS class (full-cell `color: #fff; border-color: transparent`) and the inline `style={{ backgroundColor: userColour, borderColor: userColour }}` path. The user's own selection is now represented by their slot, not the whole cell.
- Remove the `.dots` / `.dot` block and its `.slice(0, 4)` truncation.

Each cell then renders:

- Date number in the top-left corner (unchanged). Month label on day-1 cells (unchanged).
- When `n > 0`: a CSS grid of slots (`cols × rows`), one slot per participant index.
- Each slot filled with that participant's colour at:
  - Full opacity if `freeParticipantIds.has(participant.id)`.
  - Low opacity (~20%) otherwise — so every cell shows the full roster shape.
- Empty slots (trailing positions in the last row, e.g. n=8 in a 3×3 leaves 1 empty) render transparent.
- When `n === 0`: just the date number, no slot container.

Visual states layered on top:

- **Past dates**: preserve step-7 greying. For simplicity, past-date cells **skip slot rendering** entirely (just date number at reduced opacity). Avoids the opacity-compounding problem (past-cell 0.35 × faint-slot 0.20 ≈ invisible) and matches the reality that past dates aren't actionable. A later step can revisit if historical context becomes valuable.
- **Today**: bold date number (step-7 behaviour preserved).
- **Everyone-free**: thick accent border + subtle background tint. Suppressed when `participants.length < 2`.

Current user's own slot has no special marker — the `ParticipantList` row highlight is enough. Keeps the cell uncluttered. Known gap: on mobile or narrow viewports where the participant list scrolls below the fold, the user has no in-grid cue for which slot is theirs. Acceptable for V1; revisit in step 10 (polish) alongside the tap popover.

Cell `min-height` scales with slot count so small layouts aren't oversized and large ones aren't crushed. Approximate: `min-height = max(52px, dateLine + rows * slotHeight + gaps)` with `slotHeight` around 4–6px. Capped at ~120px so high participant counts (20+) don't blow out the grid height and break the 5-week visual rhythm — slots simply shrink beyond that point. Concrete values tuned during implementation; the important thing is that `min-height: 52px` (the current fixed value) is not sufficient above ~8 participants.

### 4. Accessibility label

Colour alone isn't enough (colour-blind users, low contrast). Mobile browsers don't surface `title` on long-press — iOS Safari ignores it, Chrome Android is inconsistent — so `title` alone leaves mobile with no fallback.

V1 approach:

- **Primary: `aria-label` per cell** with structured content like `"Apr 17, 2026. Free: Alice, Carol. Not free: Bob, Dave."`. Screen readers (including mobile VoiceOver / TalkBack) read this on focus. This is the real accessibility guarantee.
- **Secondary: `title`** with the same content for desktop hover tooltips. Best-effort; not relied on.
- A proper tap-and-hold popover is explicitly deferred to step 10 (polish). Documented as a known gap for sighted mobile users who aren't using a screen reader.

### 5. Click behaviour

Unchanged from step 7: tapping anywhere in the cell toggles the current user's availability for that date. Users cannot toggle other people's slots. Slot size does not change the click target — the whole cell is the target.

### 6. Components and files

Scope is larger than just `CalendarGrid` — the page component also changes because the current prop split (`selectedDates` + `otherAvailability` excluding the user) doesn't model a unified slot grid.

- `src/components/CalendarGrid.tsx` — new props, cell rendering replaced. New prop shape:

  ```ts
  interface CalendarGridProps {
    participants: Participant[];          // all, any order — component sorts by palette index
    freeByDate: Map<string, Set<string>>; // participantId set free on each date (includes current user)
    currentParticipantId: string | null;
    onToggle: (date: string) => void;
    joined: boolean;
  }
  ```

- `src/components/CalendarGrid.module.css` — remove `.selected`, `.dots`, `.dot`; add `.slotGrid`, `.slot`, `.slotFree`, `.slotFaint`, `.everyoneFree`.
- `src/app/calendars/[id]/page.tsx` — replace the `otherAvailability` memo with a `freeByDate` memo over *all* availability, pass the new props. Remove `userColour` plumbing (no longer needed — the user's colour is rendered as their slot).
- `src/lib/overlap.ts` (new) — `layoutSlots(n)` and `classifyDay(participants, freeSet)` pure functions. Keeps the logic unit-testable without rendering.

### 7. Tests

No `CalendarGrid.test.tsx` exists from step 7 — this step creates it fresh.

- `src/lib/overlap.test.ts`:
  - `layoutSlots` at boundary values: 0 → `{0,0}`; 1, 4 stay in stripes tier; 5, 8, 9, 12, 16, 17, 25, 32 use the sqrt formula with expected `{cols, rows}`.
  - Slot index → `(col, row)` mapping is row-major: participant 0 at `(0, 0)`, participant `cols` at `(0, 1)`, etc.
  - `classifyDay` identifies the free set correctly; `everyoneFree` is false when `participants.length < 2` (even if that sole participant is free), false when a silent participant exists, true when every current participant is in the free set and N ≥ 2.
  - Sorting with unknown colours: a participant whose colour isn't in `COLOUR_PALETTE` sorts after all known colours.
- `src/components/CalendarGrid.test.tsx` (new):
  - **n=0**: cells render just the date number, no slot container.
  - **n=1**: one full-width stripe, full opacity when free, faint when not; no everyone-free highlight even when the sole participant is free.
  - **n=3**: three stacked stripes, mix of free/faint states render correctly.
  - **n=5**: 3×2 grid renders 5 slots + 1 empty, confirming the layout tier transition from stripes.
  - **n=9**: 3×3 grid renders 9 slots in row-major order, no empty slots.
  - Everyone-free class applied only when every current participant is in the free set AND N ≥ 2.
  - `aria-label` includes date plus free / not-free participant names.
  - Past dates render no slots (just the date number at reduced opacity).
  - Clicking a cell calls `onToggle(date)` regardless of which slot was visually tapped.

### 8. Manual verification

- `make dev-backend` + `make dev-frontend`.
- Open a calendar with 1 participant → select some days, confirm no "everyone free" highlight.
- Add a second participant (incognito) → both mark the same day → that day gets the highlight.
- Grow to 5, 8, 9 participants (via multiple incognito tabs or a seeded fixture) → layout switches through the tiers correctly.
- Hover a cell → tooltip lists the right names.
- Past-date and today styling still work.

## Decisions

- **Adaptive slot layout**: 1-col stripes up to n=4, then near-square grid (`cols = ceil(sqrt(n))`) for n ≥ 5. Row-major packing so participant index → slot position is deterministic and stable across every cell. Alternative layouts (user-chosen) deferred to V2.
- **Faint slots for "not free"**, not blank. Shows the full roster shape in every cell.
- **Everyone-free emphasis: border + tint, no icon/label.** Keeps the grid calm.
- **Suppress everyone-free when N < 2** to avoid noise on solo calendars.
- **Silent participants count as "not free".** Simpler; matches the data model (no record = not available).
- **No "this is you" marker in cells.** `ParticipantList` row highlight is sufficient context. The step-7 full-cell background fill for the user's selection is retired. Known gap on narrow viewports where the list is below the fold — deferred to step 10.
- **Past dates skip slot rendering.** Just the date number, dimmed. Avoids the opacity-compounding problem and matches the fact that past dates aren't actionable.
- **`aria-label` is the accessibility contract, not `title`.** Mobile browsers don't show `title` on long-press; `aria-label` works with VoiceOver / TalkBack. A proper sighted-mobile popover is deferred to step 10.
- **Pure-function split** (`layoutSlots`, `classifyDay` in `src/lib/overlap.ts`) so layout and classification are unit-testable without DOM.
- **Unified `freeByDate` prop** replacing the step-7 user/others split. Simpler model; both the page component and `CalendarGrid` now treat the user as one participant among many.
- **Participant sort by palette index** inside `CalendarGrid` using a frontend `COLOUR_PALETTE` constant matching the backend. Unknown colours sort to end. Not assumed from API order.
- **Cell `min-height` scales with slot count, capped at ~120px.** The fixed 52px value is too small above ~8 participants; uncapped scaling would break the 5-week grid rhythm at high counts.
- **No count label per cell.** "N/M free" and similar summaries belong to step 9.
- **No API changes.** Step 8 is purely frontend.

## Log

- 2026-04-17 — Created plan
- 2026-04-17 — Revised after review of shipped step 7: new unified `freeByDate` prop (retired user/others split and full-cell selection fill); merged 5–8 and 9–16 layout tiers into sqrt-based near-square formula (row-major); added n=0 case; past dates skip slot rendering; `aria-label` (not `title`) is the a11y contract; participant sort by palette index inside the component; cell `min-height` scales with slot count; page component updated; tests cover n=0, 1, 3, 9
- 2026-04-17 — Plan audit fixes: added frontend COLOUR_PALETTE constant with explicit sort-key mechanism (indexOf, unknowns to end); fixed inline style to include borderColor; capped cell min-height at ~120px; noted no existing CalendarGrid test file; added n=5 test case for stripe→grid transition; documented narrow-viewport "this is you" gap as deferred to step 10; added unknown-colour sort test case
- 2026-04-17 14:34 — Implemented all sections. Created `src/lib/overlap.ts` with `layoutSlots`, `slotPosition`, `sortParticipants`, `classifyDay` and `COLOUR_PALETTE`. Rewrote `CalendarGrid.tsx` with new props (`participants`, `freeByDate`, `currentParticipantId`), slot grid rendering, `aria-label`/`title` per cell, past-date slot suppression, everyone-free border+tint. Updated `page.tsx` to build `freeByDate` memo replacing `otherAvailability`. Replaced CSS: removed `.selected`/`.dots`/`.dot`/`.dayNumberSelected`, added `.slotGrid`/`.slotFree`/`.slotFaint`/`.slotEmpty`/`.everyoneFree`. Tests: 13 unit tests in `overlap.test.ts`, 13 component tests in `CalendarGrid.test.tsx`. All 51 frontend tests pass, lint clean.
