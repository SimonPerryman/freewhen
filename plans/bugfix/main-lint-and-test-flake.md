# Fix lint and a flaky test on main after the Refactoring UI stack

Status: done

## Goal

After squash-merging #95–#102 on 2026-11-23, `make lint` and `make test` failed on `main`. Three
files from the UI stack weren't oxfmt-formatted. Those PRs were checked on CRLF checkouts, where
oxfmt flagged every file, so real issues were hidden in the noise; #102 (.gitattributes) removed
that noise. Also, the DayDetailSheet "capped on entry" test types 145 characters with userEvent and
timed out at 5s under full-suite parallel load once the char counter added per-keystroke work.

## Plan

1. Run oxfmt on the three files.
2. Use `userEvent.setup({ delay: null })` in the 145-keystroke test so it doesn't wait a timer tick
   per key.
3. `make lint` and `make test` on a fresh LF checkout of main.

## Log

- 2026-11-23 22:15 — Created plan; formatted check-email page, ShareButton and SummaryView; switched
  the typing test to `delay: null`
