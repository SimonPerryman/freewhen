# Share button redesign

Status: done

## Goal

The default-variant `ShareButton` is wrong in two ways on the finalised view:

1. **Layout bug.** The `@media (min-width: 640px)` rule in
   `ShareButton.module.css:37-45` switches the button to a narrow
   `min-width: 180px; max-width: 260px` pill. But the calendar page
   container is capped at ~540px (see `92032a9` — the FinalisedBanner had
   the same bug), so on any desktop viewport ≥640px the button collapses
   into a small left-aligned pill inside a much wider column, leaving the
   row visually unfinished.
2. **Aesthetic.** The iOS-style box-with-up-arrow share icon is the
   loudest detail in the component and doesn't match the warm "Paper"
   palette / Fraunces+Instrument Serif typography established in
   `1482c88`.

Replace the default variant with an **inset paper card** that pairs
geometrically with the FinalisedBanner and reads as part of the same
material story. Drop the icon entirely.

Also in scope: **drop the iOS share-icon SVG from the hero variant.**
The hero variant (used inside `InvitePrompt` for the only-you state, and
inside the `SummaryView` summary card when ≥2 participants pre-finalisation)
keeps its salmon full-width button shape — that does a different job (a
single big CTA in an empty state) — but the box-with-up-arrow icon goes,
since that's the visual the user dislikes and leaving it on the hero would
look half-finished. Hero gets icon-removed + a kicker line above the
button to match the editorial tone of the default variant.

`InvitePrompt` (`frontend/src/components/InvitePrompt/InvitePrompt.tsx`)
is a thin wrapper around `<ShareButton variant="hero" />` — no
standalone change needed; it inherits the hero update.

## Design spec

Demo: [`share-button-redesign.demo.html`](./share-button-redesign.demo.html)
(open in a browser; theme toggle in the top-right).

**Structure** — card-as-button, no separate inner target.

| Element | Spec |
|---|---|
| Card bg | `--tile-bg`, 1px `--border-soft`, `--radius-lg` |
| Card padding | `14px 16px` |
| Gap to FinalisedBanner | `20px` (was `14px` — too tight, cards merged visually) |
| Card hover bg | `--accent-subtle` (already in `globals.css`) |
| Card hover border | `color-mix(in oklab, var(--accent) 35%, var(--border))` |
| Active | `transform: translateY(1px)` |
| Focus | 2px `--accent` outline, 2px offset |

**Content** — two-line stack on the left, chip on the right.

| Element | Spec |
|---|---|
| Kicker | Instrument Serif italic, 14px, `--muted`. **State-aware:** `"Tell everyone the date"` when finalised; `"Invite the rest of the group"` when not finalised. |
| Headline | Fraunces 600, 17px, `--ink`, `letter-spacing: -0.005em`. Always `"Send the link to the group"`. |
| Chip | DM Sans 500, 12.5px, `8px 14px`, `border-radius: 999px`. Rest: outlined `--accent` border, transparent bg, `--accent` text. Hover: filled `--accent` / `--accent-text`. Label `"Share"`. |

**Feedback** — on clipboard fallback, chip flips to filled `--accent`,
label becomes `"Copied!"` for ~1.8s, with a 280ms scale pop
(`0.96 → 1.03 → 1`, cubic-bezier `0.2 0.8 0.2 1`). Gated behind
`prefers-reduced-motion`.

**Mobile (≤375px)** — `min-width: 0` on the text column so the kicker
can ellipsis-truncate if it ever needs to. Headline stays full-width.

## Analytics

Rename `invite_shared` → `share_link_clicked` and add a `context` field
derived from the new `state` prop:

```ts
analytics.capture("share_link_clicked", {
  method: "native" | "clipboard",
  context: "pre-finalisation" | "finalised",
});
```

`invite_shared` is misleading post-finalisation — nobody's being invited,
the date's being announced. Check whether any dashboards/funnels pin to
the old event name before merging; if so, fire both events for one
release as a deprecation window.

## Hero variant change (minimal)

- Delete the SVG block in `ShareButton.tsx:79-103` for the hero branch.
- Keep the salmon `width: 100%` button shape on hero (it's the right
  shape for an empty-state CTA).
- Hero label: keep `"Share"` / `"Copy link"` fallback exactly as today.
- Optional polish: add an italic Instrument Serif kicker line *above*
  the button inside `InvitePrompt` so the hero card matches the
  editorial tone of the default card. Defer if it grows the diff.

## Plan

1. **Default variant rebuild.** Restructure `ShareButton.tsx` markup so
   the default variant renders the card layout (kicker / headline /
   chip). Remove the iOS share-icon SVG. Rewrite the default-variant
   block in `ShareButton.module.css` — delete the `@media (min-width:
   640px)` block entirely; the card is full-width at every viewport.
2. **State prop.** Add `state?: "pre-finalisation" | "finalised"` to
   `ShareButton`, defaulting to `"pre-finalisation"`. Drives the kicker:
   `"Invite the rest of the group"` vs `"Tell everyone the date"`.
3. **Hero variant: icon removal.** Drop the SVG from the hero branch.
   Keep the salmon full-width button shape and existing label fallback.
4. **Call sites** in `frontend/src/app/calendars/[id]/CalendarPageClient.tsx`:
   - Line ~1370 (default, finalised path): pass `state="finalised"`.
   - Line ~1399 (hero, pre-finalisation path): no prop change, still
     `variant="hero"`.
5. **Analytics.** Rename event to `share_link_clicked`; add `context`
   derived from the `state` prop. Spot-check that no dashboard depends
   on `invite_shared`; if it does, fire both events for one release.
6. **`buildShareMessage` glance.** Confirm pre-finalisation share-sheet
   text doesn't reference a non-existent date. Tiny fix if it does.
7. **Tests.** Update `ShareButton.test.tsx`: assert on kicker text per
   state, chip rest/hover/copied transitions, no SVG present, analytics
   payload shape. Keep clipboard-fallback test.
8. **Verify.** `make lint` + `make test-frontend`. Visual check at
   375px and 1280px in both themes.

## Log

- 2026-05-10 23:35 — Created plan. Pre-work in conversation:
  identified the layout bug, traced history (`01d903b` → `71d4fa7`
  Apr 20 → `47ba693` Apr 20), confirmed it's the same root cause as
  the FinalisedBanner bug fixed in `92032a9`. Iterated four design
  directions (editorial pill, inset paper card, inline editorial link,
  postcard stamp), settled on inset paper card. Refined CTA, theming
  and copy in two passes — see `share-button-redesign.demo.html`
  for the resolved spec.
- 2026-05-10 23:48 — Resolved open questions. Brought the hero
  variant into scope (icon removal only, keep button shape) so the
  arrow goes everywhere it lives, including `InvitePrompt`. Decided
  to rename analytics event `invite_shared` → `share_link_clicked`
  with a `context` field, since the old name is wrong post-finalisation.
  Decided to keep `"Invite the rest of the group"` as the
  pre-finalisation kicker even when only 2 are in. Mobile wrap
  strategy: `min-width: 0` on the text column to allow kicker
  truncation at <375px.
- 2026-05-10 — Implemented. Default variant rebuilt as inset paper card
  with kicker / headline / chip; removed `@media (min-width: 640px)`
  block so the card is full-width at every viewport. Hero variant keeps
  the salmon button shape but the iOS share-icon SVG is gone. Added
  `state` prop driving the kicker copy. Renamed analytics event
  `invite_shared` → `share_link_clicked` with `context` field. Updated
  call site in `CalendarPageClient.tsx` (finalised path passes
  `state="finalised"`). Tests rewritten to cover both variants,
  state-aware kicker, no-SVG assertion, and analytics payload shape.
