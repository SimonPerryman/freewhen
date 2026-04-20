# User Journeys

## Overview

The core flow is: **Create → Share link → Others join → Everyone marks dates → See overlap**.

No auth, no accounts — just a shareable link.

## 1. Create a calendar

1. Land on `/`
2. Type an event name (e.g. "Weekend trip")
3. Click "Create calendar"
4. Redirected to `/calendars/{id}` — empty calendar, no participants yet

```
┌─────────────────────────────────────────────────────┐
│  ◀ ▶  freewhen.com/calendars/abc123                 │
├─────────────────────────────────────────────────────┤
│                                                     │
│   Weekend trip                        [ Copy link ] │
│                                                     │
│  ┌───────────────────────────────────────────────┐  │
│  │        April 2026                             │  │
│  │  Mo   Tu   We   Th   Fr   Sa   Su            │  │
│  │              1    2    3    4    5             │  │
│  │   6    7    8    9   10   11   12             │  │
│  │  13   14   15   16   17   18   19             │  │
│  │  20   21   22   23   24   25   26             │  │
│  │  27   28   29   30                            │  │
│  └───────────────────────────────────────────────┘  │
│                                                     │
│  ┌───────────────────────────────────────────────┐  │
│  │  Your name: [____________________]            │  │
│  │                          [ Join ]             │  │
│  └───────────────────────────────────────────────┘  │
│                                                     │
│  Participants                                       │
│  ─────────────                                      │
│  No one has joined yet.                             │
│                                                     │
│  Summary                                            │
│  ───────                                            │
│  No availability data yet.                          │
│                                                     │
└─────────────────────────────────────────────────────┘
```

## 2. Share the calendar

1. On the calendar page, click "Copy link"
2. "Copied!" confirmation appears briefly
3. Paste the link to friends via chat/email/etc.

```
┌─────────────────────────────────────────────────────┐
│  ◀ ▶  freewhen.com/calendars/abc123                 │
├─────────────────────────────────────────────────────┤
│                                                     │
│   Weekend trip                      [ Copied! ✓ ]   │
│                                                     │
│  ┌───────────────────────────────────────────────┐  │
│  │        April 2026                             │  │
│  │  Mo   Tu   We   Th   Fr   Sa   Su            │  │
│  │              1    2    3    4    5             │  │
│  │   6    7    8    9   10   11   12             │  │
│  │  13   14   15   16   17   18   19             │  │
│  │  20   21   22   23   24   25   26             │  │
│  │  27   28   29   30                            │  │
│  └───────────────────────────────────────────────┘  │
│                                                     │
│  Participants                                       │
│  ─────────────                                      │
│  ● Alice (you)                                      │
│                                                     │
│  Summary                                            │
│  ───────                                            │
│  No availability data yet.                          │
│                                                     │
└─────────────────────────────────────────────────────┘
```

## 3. Join a calendar

1. Open a shared link `/calendars/{id}`
2. See the calendar name and a "Your name" input
3. Type your name, click "Join"
4. If the name is already taken, see an error — pick a different name
5. You're now a participant with an assigned colour

```
┌─────────────────────────────────────────────────────┐
│  ◀ ▶  freewhen.com/calendars/abc123                 │
├─────────────────────────────────────────────────────┤
│                                                     │
│   Weekend trip                        [ Copy link ] │
│                                                     │
│  ┌───────────────────────────────────────────────┐  │
│  │        April 2026                             │  │
│  │  Mo   Tu   We   Th   Fr   Sa   Su            │  │
│  │              1    2    3    4    5             │  │
│  │   6    7    8    9   10   11   12             │  │
│  │  13   14   15   16   17   18   19             │  │
│  │  20   21   22   23   24   25   26             │  │
│  │  27   28   29   30                            │  │
│  └───────────────────────────────────────────────┘  │
│                                                     │
│  ┌───────────────────────────────────────────────┐  │
│  │  Your name: [ Bob_________________]           │  │
│  │  ⚠ Name "Bob" is already taken                │  │
│  │                          [ Join ]             │  │
│  └───────────────────────────────────────────────┘  │
│                                                     │
│  Participants                                       │
│  ─────────────                                      │
│  ● Alice                                            │
│  ● Bob                                              │
│                                                     │
└─────────────────────────────────────────────────────┘

           ↓  (after picking a unique name)

┌─────────────────────────────────────────────────────┐
│                                                     │
│   Weekend trip                        [ Copy link ] │
│                                                     │
│  ┌───────────────────────────────────────────────┐  │
│  │        April 2026                             │  │
│  │  Mo   Tu   We   Th   Fr   Sa   Su            │  │
│  │              1    2    3    4    5             │  │
│  │   6    7    8    9   10   11   12             │  │
│  │  13   14   15   16   17   18   19             │  │
│  │  20   21   22   23   24   25   26             │  │
│  │  27   28   29   30                            │  │
│  └───────────────────────────────────────────────┘  │
│                                                     │
│  Participants                                       │
│  ─────────────                                      │
│  ● Alice                                            │
│  ● Bob                                              │
│  ● Charlie (you)                                    │
│                                                     │
└─────────────────────────────────────────────────────┘
```

## 4. Return to a calendar you already joined

1. Open the same `/calendars/{id}` link again
2. localStorage remembers your participant ID — you're auto-recognised
3. No need to rejoin; you see the calendar ready to interact with

```
┌─────────────────────────────────────────────────────┐
│  ◀ ▶  freewhen.com/calendars/abc123                 │
├─────────────────────────────────────────────────────┤
│                                                     │
│   Weekend trip                        [ Copy link ] │
│                                                     │
│  ┌───────────────────────────────────────────────┐  │
│  │        April 2026                             │  │
│  │  Mo   Tu   We   Th   Fr   Sa   Su            │  │
│  │              1    2    3    4    5             │  │
│  │   6    7    8    9   10   11   12             │  │
│  │  13   14   15   16   17   18   19             │  │
│  │  20   21   22   23   24   25   26             │  │
│  │  27   28   29   30                            │  │
│  └───────────────────────────────────────────────┘  │
│                                                     │
│  Welcome back, Charlie!                             │
│                                                     │
│  Participants                                       │
│  ─────────────                                      │
│  ● Alice                                            │
│  ● Bob                                              │
│  ● Charlie (you)                                    │
│                                                     │
└─────────────────────────────────────────────────────┘
```

## 5. Mark your availability

1. After joining, the calendar grid becomes interactive
2. Click a date to mark yourself as free (click again to unmark)
3. Past dates are greyed out and unclickable
4. Changes save automatically in the background (batched/serialized)
5. If a save fails, a "Save failed" banner appears with a Retry button

**Bulk selection.** A toolbar above the week nav offers two controls:

- **Weekends** — toggles all Sat/Sun cells in the visible 5-week window. If any are unselected, all become selected; otherwise all are deselected.
- **Weekdays** — same rule, applied to Mon–Fri.

```
┌─────────────────────────────────────────────────────┐
│  ◀ ▶  freewhen.com/calendars/abc123                 │
├─────────────────────────────────────────────────────┤
│                                                     │
│   Weekend trip                        [ Copy link ] │
│                                                     │
│  ┌───────────────────────────────────────────────┐  │
│  │        April 2026                             │  │
│  │  Mo   Tu   We   Th   Fr   Sa   Su            │  │
│  │              1    2    3    4    5             │  │
│  │   6    7    8    9   10   11   12             │  │
│  │  13   14   15   16  ░17░  18  [19]            │  │
│  │  20   21   22   23   24  [25] [26]            │  │
│  │  27   28   29   30                            │  │
│  └───────────────────────────────────────────────┘  │
│                                                     │
│  ░░░ = past (greyed out)    [nn] = you're free      │
│                                                     │
│  ┌───────────────────────────────────────────────┐  │
│  │  ⚠ Save failed                    [ Retry ]   │  │
│  └───────────────────────────────────────────────┘  │
│                                                     │
│  Participants                                       │
│  ─────────────                                      │
│  ● Alice                                            │
│  ● Bob                                              │
│  ● Charlie (you)                                    │
│                                                     │
└─────────────────────────────────────────────────────┘
```

## 6. View group availability

1. As 2+ people join and mark dates, the calendar shows colour-coded slots per person per day
2. A **Summary** section appears showing:
   - "Days everyone is free" — dates where all participants are available
   - "Days most people are free" — top 5 dates ranked by count (e.g. "3/4 free")
3. A **Participant list** shows everyone with their colour dot, marking you as "(you)"

```
┌─────────────────────────────────────────────────────┐
│  ◀ ▶  freewhen.com/calendars/abc123                 │
├─────────────────────────────────────────────────────┤
│                                                     │
│   Weekend trip                        [ Copy link ] │
│                                                     │
│  ┌───────────────────────────────────────────────┐  │
│  │        April 2026                             │  │
│  │  Mo   Tu   We   Th   Fr   Sa   Su            │  │
│  │              1    2    3    4    5             │  │
│  │   6    7    8    9   10   11   12             │  │
│  │  13   14   15   16   17   18  [19]            │  │
│  │                               ABC             │  │
│  │  20   21   22   23   24  [25] [26]            │  │
│  │                           BC  ABC             │  │
│  │ [27]  28   29   30                            │  │
│  │  AC                                           │  │
│  └───────────────────────────────────────────────┘  │
│                                                     │
│  A = Alice   B = Bob   C = Charlie                  │
│  [nn] = at least one person free                    │
│                                                     │
│  Summary                                            │
│  ───────                                            │
│  Days everyone is free:                             │
│    Apr 19, Apr 26                                   │
│                                                     │
│  Days most people are free:                         │
│    Apr 19  — 3/3 free                               │
│    Apr 26  — 3/3 free                               │
│    Apr 25  — 2/3 free                               │
│    Apr 27  — 2/3 free                               │
│                                                     │
│  Participants                                       │
│  ─────────────                                      │
│  ● Alice                                            │
│  ● Bob                                              │
│  ● Charlie (you)                                    │
│                                                     │
└─────────────────────────────────────────────────────┘
```

## 7. Finalise the date (owner)

1. Only the calendar owner sees "Finalise" buttons next to each Summary row
2. Click "Finalise" next to the chosen date — the button becomes a confirm step ("Confirm Sun 26 Apr?")
3. Click to confirm, and the calendar locks:
   - A hero banner replaces the summary and best-day hints: "📅 Sunday 26 April — Confirmed"
   - The grid stays visible but all cells are disabled; the chosen date gets a solid accent treatment
   - New participants cannot join and existing participants cannot edit availability (both return 409)
4. The owner can click "Unfinalise" on the banner to reopen the calendar if plans change

```
┌─────────────────────────────────────────────────────┐
│  ◀ ▶  freewhen.com/calendars/abc123                 │
├─────────────────────────────────────────────────────┤
│                                                     │
│   Weekend trip                                      │
│                                                     │
│  ┌───────────────────────────────────────────────┐  │
│  │  📅  Confirmed                                │  │
│  │      Sunday 26 April          [ Unfinalise ]  │  │
│  └───────────────────────────────────────────────┘  │
│                                                     │
│  ┌───────────────────────────────────────────────┐  │
│  │        April 2026                             │  │
│  │  Mo   Tu   We   Th   Fr   Sa   Su            │  │
│  │              1    2    3    4    5             │  │
│  │   6    7    8    9   10   11   12             │  │
│  │  13   14   15   16   17   18   19             │  │
│  │  20   21   22   23   24   25 ▓▓26▓▓           │  │
│  │  27   28   29   30                            │  │
│  └───────────────────────────────────────────────┘  │
│                                                     │
│  ▓▓ = finalised date (locked)                       │
│                                                     │
│  Participants                                       │
│  ─────────────                                      │
│  ● Alice (you)                                      │
│  ● Bob                                              │
│  ● Charlie                                          │
│                                                     │
└─────────────────────────────────────────────────────┘
```
