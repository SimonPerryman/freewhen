# Comments / Notes on Dates

Status: done

## Goal

Let a participant attach a short free-text note to a date to qualify their availability — e.g. "free but only after 6pm", "might be late", "leaving by 9". Today the grid is binary free/not-free, which loses signal the group actually needs to pick a date. Notes reduce side-channel conversations ("wait, when are you free on Thu?") without adding a new primitive like voting or time-of-day.

From `docs/future-ideas.md`:
> Comments / notes on specific dates ("I can do this but only evening")

## Context

- Availability is stored in a composite-key table `availability(participant_id, calendar_id, date)` — no extra columns today (`backend/internal/store/migrations/001_create_tables.sql`).
- Writes go through `PUT /calendars/{id}/availability` with `{participant_id, dates[]}` — the store **bulk-replaces** the participant's rows in a transaction (`backend/internal/store/availability.go:10-53`).
- Live updates: the SSE frame carries only `{"calendar_id":"..."}` as a refresh pointer; clients refetch `GET /calendars/{id}` on receipt (`backend/internal/handler/events.go:143-149`). No payload shape change needed.
- Frontend tracks availability as `selectedDates: Set<string>` and debounces writes via `queueSave` / `flushSave` (`frontend/src/app/calendars/[id]/CalendarPageClient.tsx:232-315`).
- Cells render in `CalendarGrid.tsx`. Mobile-first, 375px baseline. Past + finalised cells are non-interactive. The grid already owns a horizontal swipe gesture for week nav (50px threshold) and a toolbar with Range / Weekends / Weekdays modes from `plans/v2/t2/multi-select-range-tap.md`.

## Design Decisions

### 1. Scope — **notes attach to (participant, date) pairs the participant has marked available**

A note qualifies *your own* availability on *a date you said you're free*. Rationale:
- Maps cleanly onto the existing `availability` row — one extra nullable column, no new table.
- Matches the source quote ("I can do this but only evening") — the note is a *conditional yes*, not a general comment.
- Notes on unavailable dates ("I'm at a wedding") would need a separate row type and is a different feature — defer to a future "unavailability reasons" proposal if demand shows up.
- If a participant unselects a date, the note is removed with the row. That's consistent with bulk-replace semantics and avoids orphaned notes.

### 2. One note per (participant, date), short free text

- Single note per cell — editing replaces. No threading, no replies. If a participant wants richer discussion, that's a chat feature, not a calendar feature.
- Max length **140 Unicode code points** (pending owner confirmation — see §Open questions). Enforced server-side with `char_length(note) <= 140`; input capped client-side using `Array.from(str).length` so the client counter matches the server rule for emoji and combining marks. Do not use JS `.length` (UTF-16 code units) — it overcounts emoji and drifts from the DB check.
- Plain text, trimmed. Newlines stripped to a single space server-side so the single-line cell preview never wraps unexpectedly (final rule pending owner confirmation).
- All-whitespace notes rejected as 400 after trim.
- No markdown, no link parsing.

### 3. Backend — **extend `PUT /availability`, with "absent = don't touch" semantics**

Request body grows to:
```json
{"participant_id": "...", "dates": ["2026-04-20", "2026-04-22"], "notes": {"2026-04-20": "only after 6pm"}}
```

Semantics (changed from earlier draft — see Log):
- `notes` **absent or `null` means "leave existing notes as-is"**, not "clear all". Protects against a stale cached client bundle silently nuking everyone's notes on their next availability edit.
- To explicitly clear all notes, clients send `"notes": {}`.
- A key in `notes` that isn't in `dates` is a 400 (note-without-date is incoherent).
- A date in `dates` without a `notes` key means "no change to this date's note" when `notes` is partial; send `"notes": {"2026-04-20": ""}` (after trim → treated as clear) to remove a specific note. *Or*, equivalently, removing the date from `dates` will delete the row and the note with it.
- Empty-string note (after trim) → `NULL` in DB (clears the note without removing availability).

### 4. Schema — **add nullable `note TEXT` to `availability`**

```sql
ALTER TABLE availability ADD COLUMN note TEXT;
ALTER TABLE availability ADD CONSTRAINT availability_note_length_chk
  CHECK (note IS NULL OR char_length(note) <= 140);
```

- Nullable — existing rows are untouched.
- Check constraint enforces the length limit at the DB layer too, so any code path that bypasses the handler still can't store oversize notes.
- Length is code-point-counted; frontend must count the same way (see §2).

### 5. SSE — **no payload change**

The refresh-pointer design already handles this. Server publishes `availability.changed` (existing event) on any note edit; clients refetch and see the new note in the `availability[]` response.

**Write cadence guard:** the editor MUST save on blur or explicit "Save", **not on every keystroke**, otherwise rapid typing fans out an SSE storm to all viewers. Reuse the existing `queueSave` debounce (≥ 500 ms) and ensure the editor uses the same path.

### 6. Read shape — **note rides on the existing `availability[]` entry**

`GET /calendars/{id}` response gains an optional `note` field on each availability row:
```json
{"participant_id": "...", "calendar_id": "...", "date": "2026-04-20", "note": "only after 6pm"}
```
Go model uses `Note *string` with `json:"note,omitempty"` so null notes are suppressed; TS type uses `note?: string`. Visible to anyone who can read the calendar — same privacy rule as availability itself.

### 7. UI — **day sheet on cell tap, colour dot indicator**

**Affordance:** tap opens a day sheet.
- **Tap on an unselected cell** → marks current participant free (unchanged fast path).
- **Tap on a cell the current participant has already marked free** → opens a **day sheet** (bottom sheet on mobile, centred modal on desktop) instead of untoggling. The sheet contains:
  - "Who's free" — list of free participants with colour swatch, name, and their note (if any).
  - **Your note editor** — textarea with 140-char counter, save/cancel. Save triggers a single `queueSave` with `dates` unchanged + `notes: { [thisDate]: text }`.
  - **"Remove my availability"** button — explicit action that replaces the previous untoggle gesture. Destructive-styled (red text), confirmation-free (recoverable by re-tapping).
- Explicitly rejected alternatives:
  - **Long-press** — collides with the horizontal swipe week-nav gesture; browsers cancel long-press on slight touch-move, which is exactly what swipe-detection requires.
  - **Pencil icon on the cell** — 375px / 7 cols ≈ 50px per cell. The cell already packs a month label, date number, slot grid, and (now) a note dot. No room for another affordance.
  - **A "Notes" toolbar mode** — adds a third mode alongside Range / Weekends / Weekdays and doesn't give us a home for reading others' notes.
- Read-only when calendar is finalised (same rule as availability toggles). The sheet still opens on tap but hides the editor and "Remove" button.
- Accessibility: sheet is a modal dialog (`role="dialog"`, `aria-modal="true"`, focus trap, Esc closes, close button with `aria-label="Close day details"`).

**Indicator:** small filled dot in the top-right corner of the cell.
```css
.noteDot {
  position: absolute;
  top: 4px;
  right: 4px;
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: var(--note-dot-colour);
  box-shadow: 0 0 0 1px var(--card-bg);
}
```
- **Colour rule:** participant colour when exactly one participant has a note on that date; `var(--text-secondary)` (neutral) when more than one. The expanded sheet is where per-participant identity lives.
- Rejected: border changes (already overloaded by `today`/`everyoneFree`/`finalised`/`rangeStart`), asterisk overlays (font-dependent glyph noise).

### 8. Finalised calendars — **notes ship in the finalised banner in v1**

Promoted from nice-to-have to in-scope.
- Notes are read-only once the calendar is finalised, same rule as availability.
- The finalised banner surfaces all participants' notes for the finalised date inline: `Sarah — only after 6pm`. This is the single moment notes become load-bearing (an attendee needs that context before showing up); deferring it strips 40% of the feature's value for 10% of the cost.
- Participants without a note are not listed there.

### 9. Privacy

- Notes are visible to every viewer of the calendar. Same boundary as availability — if you can see who's free, you can see their qualifier. No per-note privacy toggle.

## Resolved Decisions

Prior round (Q1–Q4 from draft 1) — resolved in §§7–8 above. Second round resolved 2026-04-20:

1. **Tap-on-own-free-cell opens the day sheet (§7).** The explicit "Remove my availability" button in the sheet replaces the tap-tap-tap un-toggle path. Rationale: recoverable (one re-tap restores), more discoverable than a hidden gesture, and avoids fragmenting the gesture vocabulary with swipe-left or long-press-backdrop alternatives.
2. **Note dot: finalised cells YES, past cells NO.** Past cells render as empty placeholders today — a dot there is noise users can't act on. Finalised cells keep the dot for visual-language consistency with active cells (the finalised banner surfaces the note text itself; the dot remains the at-a-glance indicator).
3. **API: "absent notes = don't touch" (§3).** Versioned endpoint rejected as over-engineering for a two-person-controlled client/server. Deploy coordination rejected as fragile. Don't-touch semantics are also forward-compatible with future clients that don't know the `notes` field exists.
4. **Newlines rejected entirely (§2).** Notes are single-line qualifiers. Strip `\r\n` client-side in the textarea handler; server rejects any note containing `\r` or `\n` with 400. Rationale: allowing-and-stripping creates preview divergence where the user sees one thing and the server stores another.
5. **140 code points stands (§2).** Sample qualifier "free but only after 6pm, and I have to leave by 9 for the last train" is 72 chars; 140 accommodates most real qualifiers with headroom. Demand for longer notes is the signal to ship the structured time-of-day picker (Out of Scope), not to raise the ceiling.
6. **Indicator colour: participant colour when exactly one note on the date, `var(--text-secondary)` when >1 (§7).** Confirmed as written.

## Plan

Ordered to unblock backend + API first, then wire the frontend, so the two halves can be reviewed independently.

1. **Migration** — add `00N_add_availability_note.sql` migration with the `ALTER TABLE` + check constraint. Embedded via `embed.FS`, picked up by `store.Migrate()` on boot.
2. **Model** — add `Note *string \`json:"note,omitempty"\`` to `model.Availability` (`backend/internal/model/model.go`).
3. **Store** — update `store.SetAvailability` to accept `notes map[string]string` (empty string or missing key → note is `NULL`). Insert `note` column in the bulk INSERT. Read path (`ListAvailability` or equivalent) must select the new column.
4. **Service** — extend `service.SetAvailability` signature to pass notes through. Publish the same `EventAvailabilityChanged` — no new event type.
5. **Handler** — extend the PUT request decoder to accept optional `notes map[string]string`. Validate:
   - `notes` absent/null → skip all note writes for this request (§3 "don't touch" rule).
   - `notes` present → for each key, trim whitespace; empty-after-trim means clear (`NULL`). Non-empty trimmed value: reject with 400 if > 140 code points.
   - 400 if any `notes` key is not in `dates`.
   - Strip newlines (`\r`, `\n`) to single spaces before length check (pending Open Q4).
6. **Backend tests** — cases for: note round-trips; note cleared when date removed; note cleared by explicit `""`; notes preserved when `notes` absent from request; unknown-date key returns 400; oversize note (code-point count, including emoji) returns 400; whitespace-only note returns 400; finalised calendar rejects note writes with 409; note preserved across an unrelated participant's availability edit.
7. **Frontend types** — add `note?: string` to the availability entry type (`frontend/src/lib/types.ts`) and `notes?: Record<string, string>` to `SetAvailabilityRequest`.
8. **Frontend state** — add `notesByDate: Map<string, string>` to `CalendarPageClient`, populated from the fetched calendar. Include it in `queueSave` payload alongside `dates` **only when the user edits a note** — otherwise omit the `notes` field entirely so the backend's "don't touch" rule applies.
9. **`CalendarGrid` note dot** — render `.noteDot` absolutely positioned in the top-right of each free-and-noted cell. Colour per §7 (participant colour when one note, neutral when many). Styles in `CalendarGrid.module.css`, mobile-first.
10. **Day sheet component** — new `DayDetailSheet` component (`frontend/src/components/DayDetailSheet/DayDetailSheet.tsx`). Bottom sheet on mobile via CSS (`position: fixed; bottom: 0`), centred modal on ≥ 640px. Contains: "who's free" list, note editor with counter, "remove my availability" button. Read-only mode when finalised. `role="dialog"`, focus trap, Esc + backdrop-tap close.
11. **`CalendarGrid` tap routing** — tap on an unselected cell still calls `onToggle` (mark free). Tap on a cell the current participant has marked free opens `DayDetailSheet` instead. Range-mode taps unchanged. Pass a new `onOpenDay(date)` prop.
12. **Note editor integration** — sheet save calls a new `onSaveNote(date, text)` prop on `CalendarGrid` → `CalendarPageClient.handleSaveNote` → updates `notesByDate` and calls `queueSave` with `notes: { [date]: text }` (only the edited key; backend "don't touch" handles the rest).
13. **Finalised banner** — update the finalised-date banner to render each participant's note inline (`Sarah — only after 6pm`). Styles in the existing banner component.
14. **Frontend tests**
    - `CalendarGrid.test.tsx`: note dot renders when current participant has a note; neutral dot when >1 participant has notes; tap-on-own-free-cell opens sheet (via mocked prop); tap-on-unselected-cell still toggles; read-only when finalised.
    - `DayDetailSheet.test.tsx`: renders free participants with notes; counter reflects code-point count (test with emoji); save calls `onSaveNote` with trimmed text; empty input saves as clear; 140+ input is capped; remove-availability calls the expected prop; Esc closes; focus returns to the triggering cell on close.
    - `CalendarPageClient.test.tsx`: editing a note fires a single debounced `setAvailability` with `notes` containing only the edited key; removing a date from selection does not require `notes` to be sent (backend clears it with the row); absent `notes` in a round-trip doesn't wipe existing notes.
15. **Analytics** — `availability_note_added { length }`, `availability_note_edited { length }`, `availability_note_cleared`. `length` is code-point count. **Never log the note body.** Also add `day_sheet_opened { has_notes: bool }` to measure sheet engagement.
16. **Docs**
    - Mark the Tier 2 row in `docs/v2-priorities.md` shipped.
    - Add the day-sheet affordance and note indicator to `docs/user-journeys.md` under "mark availability" / "view someone's availability".
    - Update `docs/api.md`: `PUT /availability` request shape (with the "absent = don't touch" semantics spelled out), `GET /calendars/{id}` response's `availability[]` gains `note`.

## Out of Scope

- Notes on dates the participant is NOT available on ("can't because: wedding"). Deferred — different data shape, different product conversation.
- Threaded replies / emoji reactions on notes.
- @-mentions or notifications when someone edits a note.
- Rich text, markdown, link previews.
- Per-note privacy (hide from certain participants).
- Owner/admin editing another participant's note.
- **Structured time-of-day picker** (e.g. "available after 18:00"). This is likely the correct long-term UX for the most common qualifier in the source quote, and would reduce notes to a general "anything else" escape hatch. Intentionally out of scope here so we can ship qualifying context fast; a future proposal can introduce a structured picker without contradicting this feature.

## Log

- 2026-04-20 HH:MM — Created plan. Pending owner confirmation on §Open questions (UI affordance, indicator style, read UX, finalised-banner inclusion) before implementation.
- 2026-04-20 HH:MM — Review pass. Resolved Q1–Q4: day sheet on tap (not long-press — collides with week-nav swipe), colour dot indicator, finalised-banner inclusion in v1. Tightened API contract to "absent notes = don't touch" (protects against stale cached clients silently wiping notes), pinned length counting to Unicode code points (was drifting between `char_length` and JS `.length`), added trim/whitespace/newline rules, added SSE write-cadence guard (save on blur, not keystroke). Promoted finalised-banner notes from nice-to-have to v1 scope. Added "structured time-of-day picker" to Out of Scope to flag the likely successor feature. New opens: tap behaviour change (removes quick double-tap-untoggle), past/finalised indicator rule, silent-wipe mitigation preference, multi-line policy, 140 vs 200 char ceiling, indicator colour policy.
- 2026-04-20 19:51 — Resolved second round of open questions. Tap-on-own-free-cell opens the day sheet (explicit "Remove my availability" button replaces un-toggle gesture). Dot on finalised cells, not on past cells. Kept "absent notes = don't touch" API semantics. Newlines rejected entirely (strip client-side, reject server-side) — no multi-line. 140 code-point ceiling stands. Indicator colour rule confirmed (participant colour for 1, neutral for >1). Ready for implementation.
- 2026-04-20 20:12 — Implemented end-to-end.
    - Backend: migration `011_add_availability_note.sql` adds nullable `note TEXT` + `char_length <= 140` check. `model.Availability.Note *string` with `omitempty`. Store's `SetAvailability(participantID, dates, noteOverrides)` merges existing notes transactionally — reads current notes, deletes rows, re-inserts with override-or-preserve per date. Handler validates: absent `notes` → nil map (don't-touch), per-date trim, empty → clear, reject whitespace-only / newlines / >140 code points / unknown-date keys.
    - Backend tests: 9 new integration tests for round-trip, preserve-on-nil-overrides, empty-clears, row-delete-drops-note, participant isolation; 7 new handler tests for absent/partial/empty/oversize/whitespace/newline/unknown-date validation paths.
    - Frontend: `MAX_NOTE_LENGTH` + `codePointCount` in `lib/limits.ts`. `SetAvailabilityRequest.notes?: Record<string,string>` — client omits the field unless the user actually edited a note. `CalendarPageClient` state: `notesByDate`, `openDayDate`, coalesced `pendingNotesRef` so rapid keystrokes fan out one PUT not many.
    - UI: `CalendarGrid` renders a 6px top-right dot on free-and-noted cells (colour rule: author when 1, neutral when >1). Tap routing split — unselected cell still toggles; already-selected own cell opens `DayDetailSheet`. Sheet is bottom-sheet on mobile, centred modal ≥ 640px; `role="dialog"`, focus trap, Esc close, code-point counter, client-side newline stripping + 140-cap truncate, explicit "Remove my availability" button. Read-only on finalised. Finalised banner inlines `Name — note text` for participants with notes.
    - Analytics: `availability_note_added` / `_edited` / `_cleared` (with `length` = code-point count; note body never logged) and `day_sheet_opened { has_notes }`.
    - Docs: `api.md` updated (GET response + PUT request with "don't touch" semantics). `v2-priorities.md` Tier 2 row marked shipped. `user-journeys.md` § 5 (mark availability) + § 7 (finalised) document the sheet, dot, and banner-notes affordances.
    - All 163 frontend tests pass; all backend unit tests pass (integration tests require a live DB and are gated by the `integration` build tag).
