# Fix unhandled rejection on duplicate participant name

Status: done

## Goal

Stop the browser from logging an `unhandledRejection` when a join attempt fails (e.g. `participant name already taken`). The UX already handles the error — `handleJoin` sets `joinError` which renders in `JoinForm` — but it then re-throws, and `JoinForm.handleSubmit` has only `try/finally`, so the rejection escapes.

Consequences today:
- Noise in the browser console.
- Any global `unhandledrejection` listener (Sentry, PostHog, etc.) treats these as real errors, making actual bugs harder to spot.

## Plan

- Remove the `throw err;` at `frontend/src/app/calendars/[id]/CalendarPageClient.tsx:1017`. The comment claims "let JoinForm know submission failed", but `JoinForm` doesn't consume that signal — `setSubmitting(false)` already runs in a `finally` block.
- Manually verify: join a calendar with a name that's already taken; confirm the inline error still renders and no unhandled rejection appears in the console.
- Check for the same rethrow-without-consumer pattern elsewhere in `CalendarPageClient.tsx` while we're here (e.g. other `throw err` after `setXError`).

## Log

- 2026-04-21 — Created plan
- 2026-04-21 — Removed `throw err` in `handleJoin` (CalendarPageClient.tsx:1017). Checked the other `throw err` at line 953 in `handleRenameParticipant` — that one is consumed by `ParticipantList.commit`'s try/catch, so left as-is.
