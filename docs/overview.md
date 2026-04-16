# FreeWhen

## Problem

Coordinating group availability is painful. The typical flow:

1. Someone messages a group chat: "I want to do X, when is everyone free?"
2. People reply with scattered dates: "I'm free Mon, Wed, Fri"
3. Someone else: "I can't do any of those, but I can do Tue, Thu, Sat"
4. Repeat until someone gives up and just picks a date

This back-and-forth is tedious, hard to track, and gets worse with more people. Information gets buried in chat history. Nobody has a clear picture of overlap.

## Solution

A dead-simple shared calendar for finding when a group is free.

- **One calendar per meetup.** Want to plan drinks? Create a calendar, share the link. Planning a hike with different people? New calendar. After the event, you're done with it.
- **Visual overlap at a glance.** Each person gets a colour. Calendar days are split into segments showing who's free. Days where everyone is free are immediately obvious.
- **Zero friction.** No sign-up, no login. Open the link, pick your name, mark your days. Done.
- **Shareable via link.** Just send the URL to your group chat.

## How It Works (V1)

1. Someone creates a new calendar (optionally names the event)
2. They get a shareable link
3. Each person opens the link, enters their name
4. They mark which days they're free on the calendar
5. Everyone can see the combined view - colour-coded segments per person
6. A sidebar/summary highlights days when everyone (or most people) are free
7. Identity is remembered via browser storage so returning users on the same device are recognised
