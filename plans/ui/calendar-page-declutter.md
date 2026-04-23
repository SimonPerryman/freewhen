# Calendar Page Declutter

Status: done

## Goal

The calendar page (`/calendars/[id]`) has accumulated v2 features that each
landed with their own row of header UI. For an owner on a fresh calendar this
now renders, stacked vertically above the grid, every time:

1. Title + rename pencil
2. "Duplicate this calendar" (plain text button)
3. "Hide others' availability until they respond" (plain text button)
4. Theme picker
5. Passphrase block (code + Copy link with passphrase / Regenerate / Remove / hint paragraph)
6. Who's missing panel
7. Live status dot

It's noisy, visually flat (everything is the same weight of grey text button),
and it pulls attention away from the one thing that matters on this page: the
calendar grid and the share action. Most of these controls are set once and
never touched again — they shouldn't live in the permanent header.

Goal: the calendar page opens with a clean header (title + share + grid) and
admin-y controls move behind a single entry point.

## Issues to fix

Captured from the screenshots + review on 2026-04-21:

1. **"Duplicate this calendar" and "Hide others' availability until they respond"** —
   unstyled plain-text buttons stacked with no separator or spacing rhythm.
   Low-frequency owner actions; should not be in the permanent header.
2. **Theme picker** — always visible for owners. A once-per-calendar setting
   that doesn't need a permanent slot. (Also worth revisiting whether it
   earned its place at all — leave that question for a separate plan, this
   one just hides it.)
3. **Who's missing panel** — oddly designed (inline panel inside the header
   region, no strong container). Feature is good; placement isn't.
4. **Passphrase block** — `Copy link with passphrase` / `Regenerate` / `Remove`
   are all visible as soon as a passphrase exists. Regenerate and Remove are
   destructive and rare; they shouldn't share real estate with the happy-path
   action. Also: the share button should already embed the passphrase in the
   URL when one is set — there's no reason to have a *second* "copy link"
   button specific to the passphrase flow.
5. **Share button size regression at 0 participants** — the small default
   `ShareButton` renders when the calendar has no participants and no
   current participant (CalendarPageClient.tsx:1467). The big hero-style
   "Invite your friends…" block (`InvitePrompt`) only renders at exactly 1
   participant (CalendarPageClient.tsx:1418). So the empty state is actually
   *less* prominent than the one-participant state. Should be the reverse.
6. **Overall information architecture** — the first-time visitor should see
   calendar name → join / share → grid. Nothing else. Owner-only controls
   should live behind a single "Settings" or "⋯" entry point (sheet or
   popover), not as a vertical wall of grey buttons.

## Plan

### 1. Consolidate owner controls into a single Settings sheet

- New component: `components/CalendarSettingsSheet/` — a mobile-first bottom
  sheet (desktop: side panel or modal) that groups the owner-only controls:
  - Rename (move the pencil-on-title rename into the sheet too, or keep the
    inline pencil — decide when building; the inline pencil is cheap and
    useful so probably keep it)
  - Theme
  - Visibility (hide others until they respond)
  - Passphrase (enable / show value / regenerate / remove, with destructive
    actions visually demoted — e.g. small text links under a divider)
  - Duplicate this calendar
  - Who's missing (lifted out of the header — probably lives here as a
    section, or stays on the page but repositioned; see step 3)
- Entry point: a single icon button in the header (cog or `⋯`) visible only
  to the owner. Placed adjacent to the calendar title.
- The sheet is closed by default. None of its contents render in the page
  tree until opened.

### 2. Fix the share button empty-state regression

- When the calendar has 0 participants (no one has joined, including the
  viewer), render the prominent `InvitePrompt`-style hero (big headline +
  share CTA). Today it only fires at exactly 1 participant.
- Keep the compact share button for the 2+ participants case where the
  summary card carries the weight.
- Concretely: fold the `revealed` branch at CalendarPageClient.tsx:1417 so
  the 0-participant and 1-participant branches share the hero treatment.
- Bonus: audit whether the separate 1-participant `InvitePrompt` vs 2+
  `SummaryView + hero ShareButton` split still makes sense, or whether
  there's a single "pre-consensus CTA" component that handles both.

### 3. Passphrase: one share action, not two

- When a passphrase is set, `ShareButton`'s built URL should already include
  `?unlock=<passphrase>` so recipients can open the calendar without typing.
- Delete the separate "Copy link with passphrase" button — it becomes
  redundant once the main share action includes it.
- The passphrase *value* (the code block) stays in the settings sheet so the
  owner can still read or dictate it. Regenerate + Remove live under that,
  demoted visually (small, not primary-weighted).

### 4. Who's missing — reposition

Options to decide during build:
- (a) Move into the settings sheet under a "People" section.
- (b) Keep on the page but move below the grid, next to the participant
     list, so it's contextual rather than header clutter.
- (c) Show inline as a single pill ("3 not yet replied — invite again")
     that opens the fuller panel as a sheet.

Leaning (b) or (c). Decide when we see it next to the grid.

### 5. LiveStatusDot — keep but make it ambient

The status dot is fine in concept but it sits as its own block in the
header flow. Move it to a corner of the header (top-right, aligned with
title) as a small unobtrusive indicator.

### 6. Visual polish pass

Once the header is down to title + settings icon + share affordance:
- Re-check spacing rhythm (the vertical stack today has inconsistent gaps).
- Confirm mobile-first layout at 375px — the header should be noticeably
  calmer on a phone, not the same density with smaller text.
- Check dark theme — the passphrase `code` block has historically been one
  of the worse-looking elements on dark.

## Non-goals

- No changes to the calendar grid itself.
- No changes to the join flow (name + join stays where it is).
- Not questioning whether themes / passphrase / who's-missing should exist
  at all — just where they live. Removals are a separate conversation.

## Log

- 2026-04-21 — Created plan after owner-view screenshots showed the header
  had become a wall of grey text buttons. Main decision: move v2 owner
  controls behind a single Settings sheet rather than redesigning each row.
- 2026-04-21 — Implemented. New `CalendarSettingsSheet` component groups
  theme, visibility toggle, passphrase (with regenerate/remove demoted to
  underlined text links), and duplicate behind a cog button in the title
  row. Header is now just title + rename pencil + cog + LiveStatusDot
  tucked to the right. `WhosMissingPanel` moved below the grid (option b).
  `ShareButton` gained a `passphrase` prop — when set it appends
  `?unlock=<passphrase>` so recipients don't need the separate
  "copy link with passphrase" button (removed). Share/invite branch
  collapsed: 0-and-1-participant cases now both render the `InvitePrompt`
  hero; 2+ keeps the summary-card layout. Removed unused
  `.passphraseBlock`/`.visibilityToggle` CSS. 226/227 existing tests
  pass; the 1 failure pre-dates this work.
