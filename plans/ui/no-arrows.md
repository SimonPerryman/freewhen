# Remove decorative arrows from the UI

Status: done

## Goal

Remove every decorative "→" from button labels and the settings section headings (owner feedback, Refactoring UI review). Keep the ‹ › week-nav chevrons in CalendarGrid. No icon replacements; plain verb labels.

## Plan

- Strip " →" from: login "Send magic link", JoinForm "Join", PassphraseGate "Unlock", RenameCalendarSheet "Save", DuplicateCalendarSheet "Duplicate", CalendarSettingsSheet (3 labels).
- Drop the CSS `content: "→ "` on the settings section label; heading weight 700 and existing spacing set it apart.

## Log

- 2026-11-23 10:00 — Created plan
- 2026-11-23 10:05 — Removed arrows from 6 components and the settings CSS prefix
- 2026-11-24 19:40 — Audit: shipped in #98; no decorative arrows left in UI labels or settings CSS. Marked done.
