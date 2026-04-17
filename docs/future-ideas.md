# Future Ideas

Roughly ordered by likely impact, but not committed to any sequence.

## V2 Candidates

### Authentication & Accounts
- Optional login (Google, Apple, email)
- Persistent identity across devices and calendars
- See all calendars you're part of (past and present)

### Calendar Integrations
- Import availability from Google Calendar / Apple Calendar / others
- Keep availability private - only free/busy is shared, not event details
- Export the agreed date into your own calendar

### Privacy & Access Controls

**Access (who can get in):**
- Private links (require a code or approval to join)
- Password-protected calendars — must enter a shared password before the calendar loads at all
- "Sign in to see other people's names" — anonymous until authenticated

**Progressive disclosure (what joiners see, and when):**
A per-calendar setting chosen by the host. Ordered from most open to most restrictive:
1. **Open** — current behaviour; anyone with the link sees all names and availability immediately
2. **Join to view** — before joining (adding your name), you see nothing: no participants, no availability
3. **Submit to view** — you've joined, but until you mark at least one date you can't see others' names or availability (prevents anchoring bias)
4. **Submit first, then reveal** — you must enter and submit your availability; once submitted you see everyone else's
5. **Host-only reveal** — participants never see each other's availability, only the host does; useful when the host is making the final call and wants honest, un-influenced responses

These stack with the access controls above (e.g. a password-protected calendar in "Host-only reveal" mode).

### Admin Permissions & Calendar Settings
- Owner can remove participants and rename the calendar
- Owner can promote other participants to admin, with the same remove/rename powers
- Admins cannot remove or demote the owner — owner role is protected
- Calendar settings panel (owner/admin only) for things like:
  - Mode toggle: "when are you free" vs "when are you not free" (could alternatively be a client-side view toggle available to anyone)
  - Other future per-calendar config as it emerges

### Richer Calendars
- Calendars that last beyond a single event (recurring group planning)
- Time-of-day availability, not just full days
- Date range suggestions from the creator ("pick from these 2 weeks")
- Week and day views alongside the default month view — useful for near-term planning or when time-of-day availability is added

### Visual Polish
- Show participant initials inside their availability cells (e.g. `SP` rendered in each day that Simon Perryman marked free), so you can see at a glance who's covering which dates without hovering or cross-referencing the legend
- Needs a sensible rule for dense cells (many participants on one day) — maybe show the first N initials and a `+3` overflow indicator

### Real-Time & Notifications
- Live updates when someone marks availability (WebSockets)
- Reminders: "3 people have responded, waiting on 2 more"
- Push notifications or email nudges

### Internationalization (i18n)
- Translate UI strings into multiple languages
- Locale-aware date and time formatting (e.g. week starts on Sunday vs Monday)
- Right-to-left layout support

## Further Out

- Voting on preferred dates (not just available)
- Comments / notes on specific dates ("I can do this but only evening")
- Timezone support for distributed groups
- Mobile app (or PWA)
- Group management (saved friend groups)
- AI suggestions ("based on everyone's history, Saturdays work best")
