# V3 Priorities

Candidate themes for post-v2 work. Not committed to a sequence. For v2
status see `v2-priorities.md`; for things built and removed see
`cut-features.md`.

## Richer Calendars

- Time-of-day availability, not just full days
- Week and day views alongside the default month view — most useful if
  time-of-day lands
- Recurring calendars — same group, ongoing cadence (distinct from the
  one-shot "duplicate a calendar" that already ships)

## Privacy & Access Tiers

Beyond the shipped Open behaviour and the cut Submit-to-view tier, the
original progressive-disclosure ladder also included:

- **Join to view** — before adding your name you see nothing (no
  participants, no availability)
- **Submit first, then reveal** — must submit availability before seeing
  anyone else's
- **Host-only reveal** — participants never see each other's availability,
  only the host does; useful when the host makes the final call and wants
  un-influenced responses
- **"Sign in to see other people's names"** — anonymous until authenticated

Revisit only if a concrete use case surfaces; none did pre-launch.

## Notifications

- Push notifications or email nudges ("3 responded, waiting on 2")
- Prereq: the paused notifications plan in `plans/v2/t2/notifications.md`

## Internationalization

- Translate UI strings
- Locale-aware date formatting (week-start Sun vs Mon)
- RTL layout support
- Timezone support for distributed groups

## Platform

- Native mobile apps
- PWA upgrades beyond basics: offline via service worker, push, install
  prompts

## Group-Scale Features

- Group management (saved friend groups)
- AI suggestions ("based on history, Saturdays work best") — needs
  meaningful data first
