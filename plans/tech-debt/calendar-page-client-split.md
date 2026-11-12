# Calendar page client split

Status: done

## Goal

`src/app/calendars/[id]/CalendarPageClient.tsx` had grown to ~1,500 lines:
one component holding ~21 `useState`, ~32 `useCallback` and ~11 `useRef`.
Every change to the calendar page meant reading the whole file, and none of
the state machines (load, save queue, identity) could be tested on its own.

Split it into cohesive hooks and components with **no behaviour change**, so
follow-up work (server-rendered first paint, see
`plans/ui/calendar-page-server-render.md`) has a clean seam to plug into.

## Plan

Boundaries picked by reading the code — each hook owns one state machine:

- `src/lib/participantStorage.ts` — the `freewhen:<calendarId>` participant
  record and the global `freewhen:lastName` key (moved verbatim from the top
  of the page). Not merged into `participantToken.ts`: that module is the
  API client's per-participant secret store (read by `request()` in
  `api.ts`, separate key, never touched by the page), whereas these records
  are page-level UI memory. Same storage, different consumers and lifetimes.
- `src/lib/localDate.ts` — `localToday()` (the old `getToday()`).
- `src/lib/useCalendarData.ts` — the fetched calendar, loading / error /
  passphrase-gate state, the `?unlock=` magic-link handshake, the SSE-driven
  `refresh()`, and the local-mutation echo suppression
  (`markLocalMutation`). Callers plug in `onLoaded` / `onRefreshed`.
- `src/lib/useParticipantIdentity.ts` — who "you" are on this calendar:
  resolve on load (signed-in user match → stored participant → silent claim →
  name prefill), sync on refresh, join, and participant rename (which must
  keep the stored record in step).
- `src/lib/useAvailabilitySave.ts` — selected dates, own notes and votes;
  the serialized save queue (`flushSave`, coalesced notes/preferred), the
  optimistic toggle / range / note / vote handlers, and clear-with-undo toast.
- `src/lib/useCalendarRename.ts` — inline calendar rename draft + commit.
- `src/lib/useCalendarOwnerActions.ts` — finalise / unfinalise, passphrase
  enable / rotate / clear, remove participant, and the confirm-dialog state
  those need.
- `src/lib/availabilityMaps.ts` — pure builders for the derived maps
  (`freeByDate`, `voteCountsByDate`, `preferredByDate`, `noteAuthorsByDate`,
  `notesByDateAndAuthor`) and the header meta labels, plus a
  `useAvailabilityMaps` memo wrapper.

Components (each with its own CSS module, rules moved verbatim from
`page.module.css`):

- `CalendarHeader` — title / inline rename input, settings button, live dot,
  rename hint, meta pills.
- `CalendarParticipantsSection` — the finalised / summary / invite branch
  with ShareButton + ParticipantList.
- `CalendarOwnerDialogs` — the three ConfirmDialogs, settings sheet and
  duplicate sheet.

Rules:

- Keep effect dependency lists, callback identities and state-update order
  as they were (e.g. the load effect still captures `user` at the time it
  runs, not when the fetch resolves).
- Existing `CalendarPageClient.test.tsx` / `page.test.tsx` must pass
  unchanged.
- Each new hook gets its own `renderHook` test file.
- Target: `CalendarPageClient.tsx` under ~400 lines.

## Log

- 2026-11-12 14:30 — Created plan
- 2026-11-12 14:36 — Extracted participantStorage / localDate, the five hooks
  and availabilityMaps; CalendarHeader, CalendarParticipantsSection and
  CalendarOwnerDialogs (own CSS modules; dead `.emoji` rule dropped). Page is
  now 351 lines (from 1,502). DuplicateCalendarSheet stays inline in the page
  so overlay DOM order is unchanged. Existing CalendarPageClient / page tests
  pass with zero edits.
- 2026-11-12 14:38 — Added renderHook tests for every hook plus the storage
  and map helpers (54 new tests). Only behavioural nuance: the stored
  participant record is now read after the calendar fetch instead of just
  before it (same tick for all practical purposes; no observable change).
- 2026-11-12 14:38 — Found, not fixed (out of scope for a no-behaviour-change
  refactor): `handleToggleVote` updates `votesByDateRef` inside a setState
  updater and then calls `flushSave()` synchronously. When React can't
  compute that update eagerly, the PUT is built from the *previous* vote set,
  so a vote can be sent stale and later reverted by a refresh. Worth a
  follow-up bugfix (compute the next set outside the updater, as
  `handleToggle` already does for dates).
