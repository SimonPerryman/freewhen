# Quick wins: header readout and join form

Status: done

## Goal

Refactoring UI pass (PR A, header + join slices): say "who's responded" once, hide tools until they're needed, and make joining a one-field form.

## Plan

- Header: one "N of M responded" readout with avatar dots (hollow ring = joined, no dates yet). Remove the "N people" / "N days" pills (and `calendarMetaLabels`) and the participant-list count badge; the list toggle is now just "People".
- "Who's missing?" panel only renders when a participant hasn't marked a date (or the owner already has it open).
- Instruction strip in CalendarGrid disappears after the first tap (localStorage, try/catch).
- JoinForm: name only; email behind "+ Get notified when a date's picked"; Join always enabled, validates on submit (`noValidate`, inline error).
- InvitePrompt shows only once the viewer has joined.

## Log

- 2026-11-23 10:30 — Created plan
- 2026-11-23 10:50 — Implemented header readout, JoinForm, invite gating, strip dismissal, Who's missing gating; tests updated (vitest passes for components/lib/calendar page)
- 2026-11-24 19:40 — Audit: shipped in #99 ("N of M responded" readout, one-field join, gated Who's missing). Marked done.
