# Quick wins: note counter, day sheet, duplicate and settings sheets

Status: done

## Goal

Refactoring UI pass (PR A), sheets slice: show the limit only when it matters, show actions only when possible, quiet the destructive ones, and clean copied names.

## Plan

- Shared `CharCounter` (hidden until 20 left, "N left"; at the limit bold text + icon + red border; screen-reader announcements at 20/10/0). `TextField` gets an opt-in `counter` prop; used by Rename and Duplicate sheets.
- DayDetailSheet: single "Note (optional)" label, counter via CharCounter, Reset/"Save note" only when the note changed, "Prefer this date" hint until the first preference (localStorage `freewhen:prefer-hint-seen`), "Remove my availability" as a quiet link with an Undo toast (restores note and vote).
- DuplicateCalendarSheet: `stripFormatting()` on the copied name.
- CalendarSettingsSheet: groups separated by spacing (no rules), plain labels, existing ✕ close, new "Delete calendar" quiet red link; delete is deferred 6s behind an Undo toast (no backend change), then navigates home.

## Log

- 2026-11-23 10:30 — Created plan
- 2026-11-23 11:10 — Implemented CharCounter, TextField counter, DayDetailSheet changes, stripFormatting, settings Delete + Undo; vitest updated/added
