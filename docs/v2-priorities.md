# V2 Priorities

Feature priority for post-v1 work. Auth is the key dependency — creator must sign in, joiners stay anonymous.

See `v3-priorities.md` for post-v2 candidates.

## Tier 1 — Core v2

| Feature | Requires | Notes |
|---|---|---|
| ~~Optional auth~~ ✅ | — | **Shipped.** Google OAuth + magic link. Creator must sign in; joiners stay anonymous. See `plans/v2/t1/auth.md`. |
| ~~Admin controls~~ ✅ | Auth | **Shipped.** Remove participants, rename calendar, owner role. See `plans/v2/t1/admin-controls.md`. |
| ~~Finalise a date~~ ✅ | Auth (creator) | **Shipped.** Owner locks in a date; calendar becomes read-only until unfinalised. See `plans/v2/t1/finalise-a-date.md`. |
| ~~Basic PWA~~ ✅ | — | **Shipped.** Manifest + icons + "Add to Home Screen". See `plans/v2/t1/pwa-basics.md`. |

## Tier 2 — Retention & UX

| Feature | Requires | Notes |
|---|---|---|
| ~~Live updates~~ ✅ | — | **Shipped** via Server-Sent Events. See `plans/v2/t2/live-updates.md`. |
| ~~"My calendars" list~~ ✅ | Auth | **Shipped.** See `plans/v2/t2/my-calendars.md`. |
| Notifications/reminders ⏸ | Auth | **Paused** — plan drafted in `plans/v2/t2/notifications.md`. Deferred in favour of lighter Tier 3 QoL wins; revisit when a clear week is available. |
| ~~Observability & analytics~~ ✅ | — | **Shipped.** PostHog (events, session replay, error capture) + admin metrics endpoint. See `plans/v2/t2/observability-and-analytics.md`. |
| Feature flags ⏸ | Observability | **Paused** — plan drafted in `plans/v2/t2/feature-flags.md`. PostHog-based flag plumbing for gradual rollouts and kill-switches. Prereq for safer shipping of Calendar integrations. |
| ~~Share with pre-filled message~~ ✅ | — | **Shipped.** See `plans/v2/t2/share-prefilled-message.md`. |
| ~~Comments/notes on dates~~ ✅ | — | **Shipped.** Tap a selected cell to open a day sheet with a 140-char note editor. See `plans/v2/t2/comments-on-dates.md`. |
| ~~Private links / password protection~~ ✅ | — | **Shipped.** Shared password before calendar loads. See `plans/v2/t2/password-protected-calendars.md`. |
| ~~Multi-select / range tap~~ ✅ | — | **Shipped.** Two-tap range + Weekends/Weekdays quick actions. See `plans/v2/t2/multi-select-range-tap.md`. |

## Tier 3 — Polish & expansion

| Feature | Requires | Notes |
|---|---|---|
| ~~Participant initials in cells~~ ✅ | — | **Shipped.** See `plans/v2/t3/participant-initials-in-cells.md`. |
| ~~Edit participant name~~ ✅ | — | **Shipped.** See `plans/v2/t3/edit-participant-name.md`. |
| ~~Default name from signed-in user~~ ✅ | — | **Shipped.** Prefills with signed-in name; anon users fall back to the last name they used on any calendar in this browser. See `plans/v2/t3/default-name-from-signed-in-user.md`. |
| ~~Clear availability~~ ✅ | — | **Shipped.** Third toolbar button with undo toast. See `plans/v2/t3/clear-availability.md`. |
| Date range suggestions | — | Highlight creator's preferred window. Might be a solution looking for a problem. |
| ~~Duplicate a calendar~~ ✅ | Auth | **Shipped.** "Same group, new event." Owner-only duplicate copies name/emoji/visibility and (optionally) participants; drops availability/finalised/passphrase. See `plans/v2/t3/duplicate-a-calendar.md`. |
| ~~Calendar integrations~~ ✅ | Auth | **Shipped.** Export (ICS) + Google Calendar import. See `plans/v2/t3/calendar-integrations.md`. |
| ~~"Who's missing?"~~ ✅ | Auth | **Shipped.** Owner-only invite list + "Not joined" / "Joined but not responded" panel on the calendar page. Auto-matches invites to participants by normalised name with a manual-link escape hatch. See `plans/v2/t3/whos-missing.md`. |
| Expiry / auto-archive | Auth | Keeps "my calendars" clean. Only matters at scale. |
| ~~Voting on preferred dates~~ ✅ | — | **Shipped.** Binary per-date vote attached to a participant's available dates; ★ badge on the cell, checkbox in the day sheet, and a tie-breaker in `BestDay`. See `plans/v2/t3/voting-on-preferred-dates.md`. |
| Week/day views | — | Mainly useful if time-of-day is added. |
| "Sign in to see names" anonymity | Auth | Niche for friend groups. |
| Recurring calendars | Auth? | Ongoing group planning. Different product shape. |

## Further out

| Feature | Requires | Notes |
|---|---|---|
| Timezone support | — | Only for distributed groups. |
| i18n | — | Important for scale, not now. |
| Time-of-day availability | — | Most use cases are "find a day" — people already have a rough time in mind. |
| Group management | Auth | Saved friend groups. Only useful at scale. |
| AI suggestions | Auth, history | Fun but gimmicky without data. |
| Native mobile app | — | PWA covers most needs first. |
