# Finalise as the primary action; quieter finalised state

Status: done

## Goal

Refactoring UI review, PR B (finalise half): the owner's best date gets one obvious primary action, and once finalised the page says so once and gets out of the way.

## Plan

- Owner view: full-width primary "Finalise <date>" for the top date (first everyone-free day, else first most-free day), with the existing confirm step. Copy link drops to a secondary (outline) button when that primary is shown. Other rows keep the small Finalise chip, confirm with a short "Confirm?" and stay on one line at 375px.
- Finalised: banner holds the date, votes, notes and exports only. Remove the "This calendar is finalised — no further changes needed." box and the grid instruction strip. "Unfinalise" becomes a quiet "Change the date" link (owner only, no label text saying so) below the grid, with an Undo toast that re-finalises the same date.
- Tests: SummaryView, FinalisedBanner, CalendarGrid strip, e2e finalise flow.

## Log

- 2026-11-23 21:30 — Created plan
- 2026-11-23 21:45 — Implemented primary Finalise, secondary Copy link, finalised-state cleanup, Change the date + Undo; vitest updated (504 pass); e2e finalise test extended (not run by this fork)
- 2026-11-24 19:40 — Audit: shipped in #100 (primary Finalise, finalised banner, "Change the date" with Undo are on main). Marked done.
