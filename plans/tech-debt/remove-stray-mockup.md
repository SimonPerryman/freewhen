# Remove stray mockup

Status: done

## Goal

Drop the rough, unreferenced mock-up `mockups/calendar-emoji-header.html` (and the top-level `mockups/` folder) from the now-public repo. The owner keeps a local copy in the gitignored `.ai-cache/mockups/`.

## Plan

1. Copy the file to the main checkout's `.ai-cache/mockups/` for safekeeping.
2. `git rm` it; the `mockups/` folder goes with it (it held nothing else).
3. Search the repo for references to `calendar-emoji-header` or the top-level `mockups/` path and fix any links. Mock files under `plans/**/mocks/` and `plans/ui/design-mockups.html` are out of scope.

## Log

- 2026-11-24 09:38 — Created plan
- 2026-11-24 09:38 — Copied the mockup to `.ai-cache/mockups/` in the main checkout and removed it (and the empty `mockups/` folder) from git
- 2026-11-24 09:38 — Searched for references; none found, so no other files needed changing
