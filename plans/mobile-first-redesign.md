# Mobile-First Redesign

Status: done

## Goal

Make FreeWhen work beautifully on phones (375px baseline) and improve general UX across all viewports. The current UI is desktop-centered with no media queries — it technically fits on mobile but the calendar grid is cramped, touch targets are small, and the layout doesn't adapt.

## Audit — Current Problems

### Mobile issues

1. **Calendar grid is too cramped on small screens** — 7 fixed columns with 4px gaps inside a 540px max-width card with 32px padding leaves ~470px for the grid. On a 375px phone with 24px page padding, the card content area is ~295px, meaning each day cell is ~40px wide. The day number + month label + slot grid are squeezed.

2. **Card padding is desktop-sized** — 32px card padding on the calendar page and 40px on the home page waste space on mobile. On a 375px screen, 32px padding per side = 64px lost to padding alone.

3. **Page padding is rigid** — 40px vertical / 24px horizontal page padding doesn't adapt. The `min-height: 100vh` centering means the card floats in the middle on desktop but on mobile the top padding pushes content down unnecessarily.

4. **Touch targets are too small** — Nav buttons are 32x32px (below the 44px minimum recommended by Apple/Google). Calendar day cells are ~40px wide on mobile. The "Share link" button is small with 6px vertical padding.

5. **Join form row doesn't stack** — Input + button in a horizontal row can get cramped on narrow screens. The button text ("Join") stays on one line but the input gets squeezed.

6. **No visual hierarchy shift for mobile** — Font sizes (28px heading, 15px body, 14px calendar, 12px day headers, 9px month labels) don't scale. The 9px month label is borderline unreadable on mobile.

7. **Day name headers are short but could be shorter** — "Mon", "Tue", etc. take up space. On very small screens, single letters ("M", "T", "W") would be tighter.

### General UX issues

8. **No visual feedback when toggling a day** — Tapping a day toggles availability but there's no animation or haptic-style feedback. The state change (a tiny colored slot going from 0.2 to 1.0 opacity) is subtle.

9. **"Everyone free" highlight is subtle** — The 2px border + 8% tinted background is easy to miss. This is the most important information and should be more prominent.

10. **Summary view is below the fold** — The "Days everyone is free" section sits below the calendar grid and participant list. On mobile you have to scroll past the grid to see it. This is arguably the most valuable information.

11. **No empty state guidance** — When you first land on a calendar with no participants, there's no explanation of what to do beyond the join form label.

12. **Share button is understated** — Sharing is the primary action after creating a calendar. The "Share link" button looks like a secondary action with its ghost-button styling.

13. **No indication of other participants' last activity** — You can't tell if the calendar is stale or if people have recently updated their availability.

14. **Copy link doesn't show the actual URL** — Users might want to see the URL before copying, or share via other means.

15. **The home page is stark** — A single input with no context, illustration, or example of what the result looks like. New users have no mental model of what they're creating.

## Plan

### Phase 1: Mobile-first CSS foundations

**Goal:** Make existing layout work well at 375px without changing component structure.

- [x] Reduce card padding to 16px on mobile, 32px on desktop (min-width: 640px breakpoint)
- [x] Reduce page padding to 16px on mobile, 40px on desktop
- [x] Remove vertical centering on mobile for calendar page — align to top with 16px top padding. Keep centering on home page.
- [x] Increase nav button size to 44x44px for touch targets
- [x] Increase calendar day cell min-height and add min-width to ensure touch target compliance
- [x] Reduce grid gap from 4px to 3px on mobile to recover space
- [x] Scale font sizes: ensure nothing is below 11px on mobile. Bump month label from 9px to 11px.
- [x] Make JoinForm stack vertically on mobile (column layout) and switch to row on larger screens

### Phase 2: Calendar grid mobile optimisation

**Goal:** Make the calendar grid the best it can be on a phone screen.

- [x] Use single-letter day headers on mobile ("M", "T", "W", "T", "F", "S", "S"), full abbreviations on desktop
- [~] Add horizontal scroll or swipe gesture as a fallback if the grid still feels cramped — skipped, Phase 1 spacing changes make it comfortable
- [x] Improve "everyone free" day styling — stronger background tint, subtle glow/shadow, or accent-colored left border so it pops even in a tight grid
- [x] Add a brief scale/pulse animation on day toggle for tactile feedback
- [~] Consider showing 4 weeks instead of 5 on very small screens — skipped, grid fits well after Phase 1 gap/padding changes

### Phase 3: Layout & information hierarchy

**Goal:** Surface the most useful information first.

- [x] Move SummaryView above the calendar grid (or into a sticky banner) so "everyone free" dates are visible immediately
- [x] On mobile, collapse ParticipantList into a compact inline row (colored dots only) that expands on tap, rather than a vertical list
- [x] Add a brief helper text or illustration on the calendar page when there are 0 participants, explaining the flow: "Share the link — when people join, you'll see their availability here"
- [x] Make the Share/Copy button more prominent — full-width on mobile below the heading, with an icon

### Phase 4: Home page & polish

**Goal:** Make the landing page inviting and give new users confidence.

- [x] Add a brief visual or animation showing what a FreeWhen calendar looks like (could be a static mini-preview or animated illustration)
- [x] Add a one-line value prop or tagline below the heading
- [x] Ensure the create form is comfortable on mobile — full-width button, adequate input height (already 16px font, good for iOS zoom prevention)
- [x] Add subtle entrance animations for the card (fade + slide up)
- [x] Review dark mode contrast — ensure all interactive elements are clearly distinguishable

### Phase 5: Typography & design tokens refresh

**Goal:** Establish a stronger visual identity beyond Geist/Arial defaults.

- [x] Choose a distinctive display font for headings (the current setup references Geist via Next.js but falls back to Arial — this feels generic)
- [x] Review the colour palette for the design system — the current monochrome (black accent on white) is clean but could benefit from a brand accent colour for key actions
- [x] Add a CSS custom property for border-radius consistency (currently mixed: 6px, 8px, 12px)
- [x] Define spacing scale as CSS custom properties for consistency

### Phase 6: Strip the card

**Goal:** Remove the card container pattern entirely. The card (border, shadow, background, padding) wraps the only content region on each page — it's a container containing everything, which is just a page with extra borders. On mobile it costs 66px (17.6% of a 375px viewport) on pure decoration.

**Approach:** Full-bleed mobile, soft-constrained desktop.

- [x] Remove `.card` wrapper — replace with a simple `.container` using `max-width` + `margin: 0 auto` + padding
- [x] Mobile: content goes edge-to-edge with 16px inline padding, no border/shadow/background
- [x] Desktop (640px+): content centers at max-width with natural negative space as the visual boundary
- [x] Unify `--card-bg` and `--page-bg` (or remove `--card-bg` entirely) — no need for two background layers
- [x] Use subtle horizontal rules or spacing between sections (summary → grid → participants) for visual structure instead of a card wrapping everything
- [x] Update entrance animation to work on the container instead of a card

## Out of scope

- Backend changes
- New features (real-time updates, auth, etc.)
- Component restructuring beyond CSS changes (keep the same React component tree)

## Log

- 2026-04-17 — Created plan after auditing all frontend CSS, components, and project docs
- 2026-04-17 22:24 — Completed Phase 1: mobile-first CSS foundations. All changes use min-width: 640px breakpoint. Card padding 16px→32px, page padding 16px→40px/24px, nav buttons 44x44px, grid gap 3px→4px, month label 9px→11px, JoinForm stacks vertically on mobile. 70 tests pass, 0 lint errors.
- 2026-04-17 22:27 — Completed Phase 2: calendar grid mobile optimisation. Single-letter day headers on mobile (M/T/W…) with full abbreviations on desktop via CSS show/hide. "Everyone free" styling strengthened with 14% tint, accent left border, outer glow. Pulse animation (0.2s scale) on day toggle for tactile feedback. Skipped horizontal scroll and 4-week view — not needed after Phase 1 spacing improvements. 70 tests pass, 0 lint errors.
- 2026-04-17 22:36 — Completed Phase 3: layout & information hierarchy. Moved SummaryView above CalendarGrid. ParticipantList collapses to inline dots on mobile with expand-on-tap. Added empty state guidance text with dashed border. CopyLinkButton now full-width with link icon on mobile, ghost button on desktop. Header stacks vertically on mobile.
- 2026-04-17 22:36 — Completed Phase 4: home page & polish. Heading changed to "FreeWhen" with tagline "Find when everyone's free. No sign-ups, no fuss." Added CSS-only mini calendar preview with highlighted cells. Card entrance animation (fade + slide up, 0.4s). Dark mode border bumped from #262626 to #333 for better contrast. Page metadata updated.
- 2026-04-17 22:36 — Completed Phase 5: typography & design tokens. Geist now the body font (was only on pages, body fell back to Arial). Added --radius-sm/md/lg and --space-xs through --space-3xl CSS custom properties. All hardcoded border-radius values replaced with tokens. Removed redundant font-family declarations from page styles. Updated page title/description metadata.
- 2026-04-17 22:36 — All 5 phases complete. 70 tests pass, 0 lint errors. Plan marked as done.
- 2026-04-17 22:42 — Reopened plan. Added Phase 6: strip the card pattern entirely. Card adds no value — it's a container wrapping the only content region. On mobile it wastes 17.6% of viewport width on decoration. Replace with max-width centered container, let negative space do the work on desktop.
- 2026-04-17 22:48 — Completed Phase 6: stripped card from both pages. .card → .container (max-width only, no border/shadow/bg). Unified --page-bg to #fff (was #fafafa). --card-bg kept for component elements (inputs, buttons, day cells) but now #fff light / #111 dark. Stripped card from error.tsx and not-found.tsx too. 70 tests pass, 0 lint errors. All 6 phases complete.

