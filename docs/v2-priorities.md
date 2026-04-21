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
| Notifications/reminders | Auth | "3 people responded, waiting on 2 more." Only works for signed-in users. |
| ~~Share with pre-filled message~~ ✅ | — | **Shipped.** See `plans/share-prefilled-message.md`. |
| ~~Comments/notes on dates~~ ✅ | — | **Shipped.** Tap a selected cell to open a day sheet with a 140-char note editor. See `plans/comments-on-dates.md`. |
| ~~Private links / password protection~~ ✅ | — | **Shipped.** Shared password before calendar loads. See `plans/password-protected-calendars.md`. |
| ~~Multi-select / range tap~~ ✅ | — | **Shipped.** Two-tap range + Weekends/Weekdays quick actions. See `plans/multi-select-range-tap.md`. |

## Tier 3 — Polish & expansion

| Feature | Requires | Notes |
|---|---|---|
| Participant initials in cells | — | See who's free at a glance. Nice polish. |
| Edit participant name | — | Self-rename (typo fix) + owner-rename (cleanup). Same unique-name check as join. No rename limit — rate-limit server-side only if abuse shows up. |
| Default name from signed-in user | Auth | Prefill join input with user's display name (still editable). Falls back to empty for magic-link users without a name. |
| Clear availability | — | Small QoL. |
| Date range suggestions | — | Highlight creator's preferred window. Might be a solution looking for a problem. |
| Duplicate a calendar | Auth | "Same group, new event." Can't auto-invite anonymous users. |
| Calendar integrations | Auth | Import/export Google/Apple. High value, high complexity. |
| "Who's missing?" | Auth | See who hasn't responded. Needs expected participant list or named invites. |
| Expiry / auto-archive | Auth | Keeps "my calendars" clean. Only matters at scale. |
| Voting on preferred dates | — | "I'd prefer this date." Adds signal beyond availability. |
| Calendar colour themes | — | Let creator pick a vibe. Minor but personal. |
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
