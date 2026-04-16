# V1 Goals

The MVP. Minimal friction, maximum clarity.

## Core Features

### Calendar Creation
- Create a new calendar with an optional event name
- Generates a unique shareable URL

### Adding Availability
- Open a calendar link, enter your name (first time only per browser)
- Mark days you're free by clicking/tapping on the calendar
- Toggle days on/off easily

### Visual Overlap
- Each participant is assigned a distinct colour
- Each calendar day is split into segments (one per participant)
- A segment is filled with that person's colour if they're free
- Days where everyone is free should stand out clearly (highlight, border, label)

### Summary View
- Sidebar or section showing "Days everyone is free" as a simple list
- Possibly also "Days most people are free" with counts

### Sharing
- Shareable URL (the only way to join a calendar)
- Copy-link button

### Identity
- No login required
- On first visit to a calendar, user enters their name
- Identity stored in browser (localStorage or similar)
- Returning to the same calendar on the same browser remembers who you are
- Different device = treated as a new person (acceptable for V1)

## Non-Goals for V1
- User accounts / authentication
- Calendar integrations (Google, Apple, etc.)
- Real-time live updates (polling or manual refresh is fine)
- Privacy controls
- Persistent cross-calendar identity
- Reminders or notifications
