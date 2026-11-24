# UI fixes: day sheet doesn't open the keyboard

Status: done

## Goal

Opening a date's sheet focused the note textarea (`initialFocus={textareaRef}`), which raises the on-screen keyboard on phones every time a day is tapped. That covers half the sheet before the user has decided to write anything. Owner decision (2026-11-24): focus the note, and raise the keyboard, only when the user taps the field. This applies everywhere, with no desktop exception.

## Plan

- `DayDetailSheet.tsx`: drop the `initialFocus` prop. `Sheet` then falls back to its documented default, the first focusable element, which is the ✕ close button. The dialog stays named by its date (`aria-labelledby`), and the focus trap, Escape and return-focus behaviour are unchanged.
- Remove the now-unused `textareaRef` and the "move caret to end on open" effect. The effect only made sense after autofocus, and calling `setSelectionRange` on an unfocused textarea can focus it in some older iOS Safari versions. A tap places the caret natively.
- Tests:
  - vitest: the close button has focus on open, the textarea doesn't, and clicking the textarea focuses it.
  - e2e (`submit-availability`): after opening a day, the close button is focused and the note isn't.
- Screenshot the sheet on open at 375px, light and dark.

## Log

- 2026-11-24 12:40 — Created plan. Before shot: `document.activeElement` on open is the textarea.
- 2026-11-24 12:45 — Removed `initialFocus`, `textareaRef` and the caret-to-end effect. Added vitest (close button focused on open, tap focuses the note) and the e2e assertion. After shot: the close button is active on open and shows no focus ring.
- 2026-11-24 12:48 — `make lint` and `make test` pass. `submit-availability` e2e passes. On-device iOS check still to do (the fix is right either way).
