# V2 Priorities

Feature priority for post-v1 work. Auth is the key dependency — creator must sign in, joiners stay anonymous.

See `future-ideas.md` for detailed descriptions of each feature.

## Tier 1 — Core v2

| Feature | Requires | Notes |
|---|---|---|
| Optional auth | — | Creator must sign in. Joiners stay anonymous with optional sign-in. Foundation for admin, notifications, "my calendars", privacy modes. |
| Admin controls | Auth | Remove participants, rename calendar, owner/admin roles. Essential moderation. |
| Finalise a date | Auth (creator) | Creator locks in the chosen date. Completes the core journey inside the app. |
| Basic PWA | — | Manifest + icons + "Add to Home Screen". Trivial effort, mobile-first app needs this. |

## Tier 2 — Retention & UX

| Feature | Requires | Notes |
|---|---|---|
| "Submit to view" privacy | — | Prevents anchoring bias. Works without auth. |
| Live updates (WebSockets) | — | Makes "everyone fill this in now" seamless. |
| "My calendars" list | Auth | See all calendars you're part of. Only works for signed-in users. |
| Notifications/reminders | Auth | "3 people responded, waiting on 2 more." Only works for signed-in users. |
| Share with pre-filled message | — | Web Share API on mobile. Low effort, reduces friction. |
| Comments/notes on dates | — | "Free but only after 6pm." Low complexity, avoids side convos. |
| Private links / password protection | — | Shared password before calendar loads. Useful for sensitive plans. |
| Multi-select / range tap | — | Drag-to-select or "select all weekends." QoL for mobile. |

## Tier 3 — Polish & expansion

| Feature | Requires | Notes |
|---|---|---|
| Participant initials in cells | — | See who's free at a glance. Nice polish. |
| Edit name / clear availability | — | Small QoL. |
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
