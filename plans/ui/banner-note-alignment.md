# Confirmed banner: note rows misalign when they wrap

Status: done

## Goal

On the Confirmed banner, each note row is a dot, the name, a dash and the note. Two bugs showed up in the refreshed README screenshots at 375px:

1. **Dot drifts off the name.** The dot was `align-self: center`, so when a note wrapped ("Sam — Halfway through, no / spoilers") it centred on the whole two-line row and sat between the lines.
2. **Long names crush the note.** Name, dash and note were separate flex items, so a long name, such as an email-derived one, left the note a column a few characters wide that wrapped one word per line.

## Plan

- **Layout.** Wrap name, dash and note in one `.noteBody` span that flows as a paragraph beside the dot, so wrapped lines start under the name and use the full width. `overflow-wrap: anywhere` breaks unbroken strings.
- **Dot.** The row is `align-items: flex-start`, and the dot gets `margin-top: calc((1lh - 10px) / 2)` (with an em fallback), which centres it on the first line.
- **Test.** An e2e test in `finalise.test.ts` uses a long note at 375px. It checks that the row wraps, the dot is centred on the first line (within 2px), and the note is wider than 60% of the row.
- Screenshots at 375px, light and dark.

## Log

- 2026-11-24 15:05 — Created plan from the owner's README screenshot (Sam's dot between lines).
- 2026-11-24 15:15 — Added the e2e test. Before the fix it failed, with the dot 127px off the first line, because the long e2e owner name crushed the note into a 33px column. That's how bug 2 turned up.
- 2026-11-24 15:25 — `.noteBody` paragraph flow and first-line dot. The e2e test passes. Shots in `.ai-cache/ui-fixes/screens/ui-banner-note-dot/`: "Sam" and the long owner name both align, and the note wraps under the name.
