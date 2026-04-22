# V2 Priorities

Feature priority for post-v1 work. Auth is the key dependency — creator must sign in, joiners stay anonymous.

See `future-ideas.md` for detailed descriptions of each feature.

## Tier 1 — Core v2

| Feature | Requires | Notes |
|---|---|---|
| ~~Optional auth~~ ✅ | — | **Shipped.** Google OAuth + magic link. Creator must sign in; joiners stay anonymous. See `plans/auth.md`. |
| ~~Admin controls~~ ✅ | Auth | **Shipped.** Remove participants, rename calendar, owner role. See `plans/admin-controls.md`. |
| ~~Finalise a date~~ ✅ | Auth (creator) | **Shipped.** Owner locks in a date; calendar becomes read-only until unfinalised. See `plans/finalise-a-date.md`. |
| ~~Basic PWA~~ ✅ | — | **Shipped.** Manifest + icons + "Add to Home Screen". See `plans/pwa-basics.md`. |

## Tier 2 — Retention & UX

| Feature | Requires | Notes |
|---|---|---|
| ~~"Submit to view" privacy~~ ✅ | — | **Shipped.** See `plans/submit-to-view-privacy.md`. |
| ~~Live updates~~ ✅ | — | **Shipped** via Server-Sent Events. See `plans/live-updates.md`. |
| ~~"My calendars" list~~ ✅ | Auth | **Shipped.** See `plans/my-calendars.md`. |
| Notifications/reminders ⏸ | Auth | **Paused** — plan drafted in `plans/notifications.md`. Deferred in favour of lighter Tier 3 QoL wins; revisit when a clear week is available. |
| ~~Share with pre-filled message~~ ✅ | — | **Shipped.** See `plans/share-prefilled-message.md`. |
| ~~Comments/notes on dates~~ ✅ | — | **Shipped.** Tap a selected cell to open a day sheet with a 140-char note editor. See `plans/comments-on-dates.md`. |
| ~~Private links / password protection~~ ✅ | — | **Shipped.** Shared password before calendar loads. See `plans/password-protected-calendars.md`. |
| ~~Multi-select / range tap~~ ✅ | — | **Shipped.** Two-tap range + Weekends/Weekdays quick actions. See `plans/multi-select-range-tap.md`. |

## Tier 3 — Polish & expansion

| Feature | Requires | Notes |
|---|---|---|
| ~~Participant initials in cells~~ ✅ | — | **Shipped.** See `plans/participant-initials-in-cells.md`. |
| ~~Edit participant name~~ ✅ | — | **Shipped.** See `plans/edit-participant-name.md`. |
| ~~Default name from signed-in user~~ ✅ | — | **Shipped.** Prefills with signed-in name; anon users fall back to the last name they used on any calendar in this browser. See `plans/default-name-from-signed-in-user.md`. |
| ~~Clear availability~~ ✅ | — | **Shipped.** Third toolbar button with undo toast. See `plans/clear-availability.md`. |
| Date range suggestions | — | Highlight creator's preferred window. Might be a solution looking for a problem. |
| ~~Duplicate a calendar~~ ✅ | Auth | **Shipped.** "Same group, new event." Owner-only duplicate copies name/emoji/visibility and (optionally) participants; drops availability/finalised/passphrase. See `plans/duplicate-a-calendar.md`. |
| Calendar integrations | Auth | Import/export Google/Apple. High value, high complexity. |
| ~~"Who's missing?"~~ ✅ | Auth | **Shipped.** Owner-only invite list + "Not joined" / "Joined but not responded" panel on the calendar page. Auto-matches invites to participants by normalised name with a manual-link escape hatch. See `plans/whos-missing.md`. |
| Expiry / auto-archive | Auth | Keeps "my calendars" clean. Only matters at scale. |
| ~~Voting on preferred dates~~ ✅ | — | **Shipped.** Binary per-date vote attached to a participant's available dates; ★ badge on the cell, checkbox in the day sheet, and a tie-breaker in `BestDay`. See `plans/voting-on-preferred-dates.md`. |
| Calendar colour themes | — | Let creator pick a vibe. Minor but personal. (previously shipped and cut — see `docs/cut-features.md`) |
| Week/day views | — | Mainly useful if time-of-day is added. |
| Advanced options disclosure on create | — | Roll "Hide others until you respond" (and other power-user toggles) into a collapsed **Advanced options** section on the create-calendar form. Today it sits inline next to the primary fields and adds cognitive weight to the 90% case who don't need it. Default-collapsed, chevron to expand, remembers expanded state per-session. Future toggles (date-range, host-only reveal, etc.) land inside the same drawer so the primary form stays calm. |
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
