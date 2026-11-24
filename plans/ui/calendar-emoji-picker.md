# Calendar emoji picker

Status: done

## Goal

There was no way to change a calendar's emoji. It was auto-picked at create time and fixed for good. The owner wants auto-pick to avoid repeats (`plans/bugfix/calendar-emoji-repeats.md`), but "people can still choose the same emoji on purpose". This adds a small picker to calendar settings. Any emoji in the picker can be chosen, including one the owner's other calendars already use.

## Plan

- **Backend:**
  - `PATCH /calendars/{id}` accepts `emoji` alongside `name`, and both fields are validated before anything is written.
  - The emoji must be in `pickableEmojis`, which is the 12 auto-pick emojis plus 12 topical ones (🎉 🎂 🎄 🍻 ☕ 🍕 🎲 🎸 📚 🎾 🚲 🧗). All are single code points with default emoji presentation, so there are no VS16 variants to normalise.
  - Anything else returns 400 `invalid_emoji`. `Service.SetCalendarEmoji` is owner-only and publishes `calendar_updated`. `Store.UpdateCalendarEmoji` bumps `updated_at`.
- **Frontend:**
  - `lib/emoji.ts` mirrors the list.
  - The Settings sheet gets an "Emoji" fieldset at the top: a grid of 24 tiles (six per row at 375px, each at least 44px). The tiles are native radios, so arrow keys and screen readers work, and the checked one gets an accent ring.
  - `useCalendarEmoji` applies the change optimistically, rolls back with a toast if the save fails, and on success shows "Emoji changed to 📚" with Undo. That follows the AGENTS rule for reversible actions.
- **Tests:**
  - Go: handler (valid, invalid rejected before any write, name+emoji, not owner), service (pickable set, `SetCalendarEmoji`, owner-only table), store integration (update + not found), server integration (a repeat emoji is allowed, an unlisted one is rejected).
  - vitest: the sheet's radios and selection, and the hook (optimistic update, Undo, rollback, no-op).
  - e2e: `calendar-emoji.test.ts`.
- **Docs:** `docs/api.md` (PATCH body and errors) and `docs/onboarding.html` (endpoint table, hook list, settings sheet).

## Log

- 2026-11-24 13:40 — Created plan. Confirmed there's no existing UI or API to change the emoji, so this optional PR is in scope.
- 2026-11-24 13:55 — Backend done. Handler, service, store and integration tests pass (`go test -tags=integration -p 1` on store and server).
- 2026-11-24 14:05 — Frontend picker, hook, tests and docs done. Shots at 375px light and dark: `settings-*` and `settings-picked-*` (📚 ringed, Undo toast). The emoji e2e and create-calendar e2e pass.
