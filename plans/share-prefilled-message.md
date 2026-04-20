# Share With Pre-Filled Message

Status: done

## Goal

Make the share sheet produce a message that reads naturally in iMessage, WhatsApp, and Slack, rather than the current generic `"Mark your availability on FreeWhen"`. The link already works — we're upgrading the prose so recipients understand what they're being sent before tapping.

## Scope

- Craft the `text` payload for `navigator.share()` to include the calendar name (and emoji if set) and a short action phrase.
- Branch on calendar state: pre-finalise vs finalised. A finalised calendar should announce the date, not invite people to mark availability.
- Leave the clipboard fallback as URL-only (rationale below).
- No changes to the backend.

**Out of scope:**
- Participant-specific messages ("Hey it's Alice, join this calendar"). Keep one message shape regardless of who's sharing — adding identity invites copy bloat and we have no signal for anonymous joiners anyway.
- Custom user-editable messages. The share sheet on iOS already lets the sender edit freely before sending; we just need to seed it well.
- SMS-specific or WhatsApp-specific message formatting. `navigator.share` hands the whole `{title, text, url}` payload to the OS and the destination app decides what to do — we don't get per-app control.

## Design Decisions

**URL stays separate from message.** Web Share API takes `title`, `text`, and `url` as separate fields. iOS/Android compose them into a single message for the target app; we shouldn't pre-compose by jamming the URL into `text`. Keeping `url` separate means iMessage shows a rich link preview, and Slack unfurls properly. Concatenating into `text` breaks both.

**Clipboard fallback = URL only.** On desktop (no `navigator.share`), the button copies just the URL to clipboard. Reasons:
- Desktop users who paste into iMessage / Slack / a browser URL bar all expect a clean link. A "message + URL" paste breaks URL-bar paste and looks odd in some message fields.
- The hero "Invite your friends" card already has framing text around the button — the recipient context is set by the *sender's* message, not the link they paste.
- Simpler: one behaviour to reason about.

If we later find desktop users want a richer paste, we can add a secondary "copy message" action — but not before there's evidence.

**Message shape depends on finalised state.** Two templates:

1. **Pre-finalise:** `"{emoji} Help me find a date for {name} — mark when you're free:"`
   - "Help me" is warmer than "Mark your availability" and makes the ask clear.
   - "for {name}" anchors the recipient to *what* they're scheduling.
   - Trailing colon reads naturally before the URL the OS appends.
2. **Finalised:** `"{emoji} {name} is on {date} — add it to your calendar:"`
   - Announces the outcome instead of asking for input.
   - Date formatted as `Friday 1 May` (short weekday + D MMM, matching the finalised banner style on the page).
   - "add it to your calendar" hints at the future calendar-export work without committing to it — for now the URL just opens the finalised page.

**Emoji prefix is optional.** If `calendar.emoji` is set, prepend it with a space. If not, skip the prefix entirely (no fallback emoji — avoids the "📅 📅 📅" soup when the calendar already has one and we add a generic one).

**Title field stays as `{name}`.** Most platforms ignore `title`, but Android uses it in the share-sheet preview. Calendar name alone is fine there; no need to duplicate the message.

**No localisation.** English only for now, matching the rest of the app.

**No A/B testing.** This is a copy change; we'll ship the one variant that reads best and iterate if analytics (`invite_shared` counts already captured) don't move.

## Implementation

All changes in `frontend/src/components/ShareButton/ShareButton.tsx`.

### Props

Extend `ShareButtonProps`:

```ts
interface ShareButtonProps {
  calendarName: string;
  calendarEmoji?: string | null;
  finalisedDate?: string | null; // ISO yyyy-mm-dd
  variant?: "default" | "hero";
}
```

All new props are optional. Callers that don't pass them get the current behaviour (pre-finalise, no emoji).

### Message builder

Add a `buildShareMessage({ name, emoji, finalisedDate })` helper inside the component file (or `frontend/src/lib/share.ts` if we want it unit-testable without rendering — leaning toward `lib/share.ts` because date formatting has edge cases worth testing in isolation).

```ts
export function buildShareMessage(args: {
  name: string;
  emoji?: string | null;
  finalisedDate?: string | null;
}): string {
  const prefix = args.emoji ? `${args.emoji} ` : "";
  const name = args.name || "this calendar";
  if (args.finalisedDate) {
    return `${prefix}${name} is on ${formatFinalisedDate(args.finalisedDate)} — add it to your calendar:`;
  }
  return `${prefix}Help me find a date for ${name} — mark when you're free:`;
}
```

`formatFinalisedDate` parses `yyyy-mm-dd` as a local date and formats as `"Friday 1 May"`. Reuse existing date util if one matches (`src/lib/date.ts` or similar — check before writing new).

### Callers

Update the four `ShareButton` usages in `CalendarPageClient.tsx` and the one in `InvitePrompt.tsx` to pass `calendarEmoji` and `finalisedDate`:

- `CalendarPageClient.tsx:638` — `<InvitePrompt calendarName={...} calendarEmoji={calendarEmoji} />`
- `CalendarPageClient.tsx:648, 659, 669` — `<ShareButton calendarName={...} calendarEmoji={calendarEmoji} finalisedDate={finalisedDate} />`
- `InvitePrompt.tsx` — accept and forward the new props.

## Testing

### Unit (`frontend/src/lib/share.test.ts`)

- `buildShareMessage` pre-finalise with emoji.
- `buildShareMessage` pre-finalise without emoji.
- `buildShareMessage` finalised state formats date as `Friday 1 May`.
- `buildShareMessage` empty name falls back to `"this calendar"`.
- `buildShareMessage` finalised with emoji combines both.

### Component (`ShareButton.test.tsx`)

- Update existing `navigator.share` assertion from `"Mark your availability on FreeWhen"` to the new pre-finalise message.
- Add test: `finalisedDate` prop produces the finalised-state message.
- Keep the existing clipboard fallback test — it asserts URL copying, which is unchanged.

### Manual QA

- Share a pre-finalise calendar from iOS → preview the iMessage compose screen, confirm text + link preview render cleanly.
- Share the same calendar from Android → confirm text field pre-fills.
- Finalise the calendar, share again → confirm new copy with date.
- Share from desktop (Chrome, no `navigator.share`) → confirm clipboard copy is still URL-only.

## Build Order

1. Write `buildShareMessage` + unit tests in `src/lib/share.ts`.
2. Wire into `ShareButton`, update component test.
3. Thread `calendarEmoji` + `finalisedDate` through `InvitePrompt` and the direct `ShareButton` callers in `CalendarPageClient.tsx`.
4. Manual QA on an iPhone (the primary target — most shares happen from mobile).
5. Mark plan `done`.

## Open Questions

- **Exact wording.** "Help me find a date for X" is the current proposal — worth saying out loud before committing. Alternatives considered: "When are you free for X?" (abrupt), "What days work for X?" (good but loses the "help me" warmth), "Finding a date for X — mark yours" (good, slightly terse).
- **Should the finalised-state share include a time?** Currently we only store a date. If time-of-day ships later, revisit.

## Log

- 2026-04-20 15:58 — Created plan.
- 2026-04-20 16:05 — Implemented: `src/lib/share.ts` builds the pre-finalise / finalised messages with optional emoji prefix, `ShareButton` now accepts `calendarEmoji` + `finalisedDate` and forwards to `navigator.share`, `InvitePrompt` and all four `CalendarPageClient` call sites pass the new props. Unit tests for `buildShareMessage` (6 cases) and an additional finalised-state share test. Clipboard fallback unchanged (URL only). Status → done.
