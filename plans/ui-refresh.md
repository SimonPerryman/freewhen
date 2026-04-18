# UI Refresh — Partiful-Inspired Improvements

Status: done

## Goal

Elevate FreeWhen's frontend from functional-minimal to polished and delightful. Inspired by Partiful's playful, social, mobile-native feel — but staying true to FreeWhen's identity as a lightweight, no-sign-up scheduling tool.

## Priority Tiers

### High: Native Share Flow
### High: Mobile-First Polish
### Mid: Motion & Delight
### Mid: Visual Richness
### Mid: Results & Summary UX
### Low: Emoji avatars, color themes, comments (deferred)

---

## Phase 1 — Native Share + Mobile Polish

These two are tightly coupled: the share button is a mobile-first concern and feeds into the overall touch experience.

### 1a. Native Share Button

Replace `CopyLinkButton` with a `ShareButton` that:
- Uses `navigator.share()` on supported devices (mobile Safari, Chrome Android)
- Falls back to clipboard copy on desktop / unsupported browsers
- Shares: `{ title: calendarName, text: "Mark your availability on FreeWhen", url: window.location.href }`
- Shows a brief confirmation toast ("Copied!" / "Shared!") instead of inline text swap

**Files:** `src/components/CopyLinkButton/` → rename to `src/components/ShareButton/`

### 1b. OG Meta Tags for Link Previews

Add dynamic `<meta>` tags so shared links look good in iMessage/WhatsApp/Slack:
- `og:title` = calendar name
- `og:description` = "Mark your availability — no sign-up needed"
- `og:image` = static branded preview image (create a simple one)
- `og:url` = canonical URL

This requires the calendar page to have server-side metadata. Currently the `[id]/page.tsx` is `"use client"` — we'll need to split: a server component wrapper that fetches the calendar name for metadata, rendering the existing client component as a child.

**Files:**
- `src/app/calendars/[id]/page.tsx` — split into server wrapper + client component
- `src/app/layout.tsx` — base OG tags
- `public/og-default.png` — static fallback image

### 1c. Mobile Touch Improvements

- **Larger touch targets on calendar cells:** increase min-height from 52px, ensure 44px minimum tap area
- **Swipe navigation for weeks:** add touch event handlers (touchstart/touchend) on the grid for left/right swipe to change weeks. CSS-only with `scroll-snap` won't work here since we're paginating server data — use pointer events
- **Bottom-anchored share button on mobile:** position the share button fixed at bottom on small screens for easy thumb reach
- **Haptic-feel feedback:** enhance the existing pulse animation — make it snappier, add a subtle background color flash on toggle
- **Safe area insets:** respect `env(safe-area-inset-bottom)` for notched phones

**Files:**
- `src/components/CalendarGrid/CalendarGrid.tsx` — swipe handler
- `src/components/CalendarGrid/CalendarGrid.module.css` — touch target sizing
- `src/components/ShareButton/` — bottom-anchored mobile layout
- `src/app/globals.css` — safe area padding

---

## Phase 2 — Motion & Delight + Visual Richness

### 2a. Staggered Grid Entrance

When the calendar grid first renders (or when navigating weeks), cells animate in with a staggered wave:
- Each cell gets `animation-delay` based on its index (row * 7 + col) * 20ms
- Animation: fade in + slight scale up, 0.25s ease-out
- Only on initial mount or week change, not on every re-render

**Files:** `CalendarGrid.module.css`, `CalendarGrid.tsx`

### 2b. Date Toggle Feedback

Replace the current pulse (scale 1→1.08→1) with a richer interaction:
- Quick background color fill that radiates outward (CSS radial gradient transition)
- On select: slot color fills with a satisfying snap
- On deselect: color drains away

**Files:** `CalendarGrid.module.css`

### 2c. Visual Identity Upgrade

**Typography:**
- Replace Geist Sans with a distinctive display font for headings (e.g. a geometric sans or slab) — keep a clean body font
- Add a display font variable in `layout.tsx` via `next/font/google`
- Increase heading sizes and letter-spacing for more presence

**Color palette:**
- Move away from monochrome black/white accent
- Introduce a primary brand color (warm tone — coral, amber, or teal) with tints/shades
- Keep the dark mode but make it richer (not just inverted grays)
- Update CSS custom properties in `globals.css`

**Texture & depth:**
- Subtle noise/grain texture on the page background
- Soft shadows on cards/cells instead of flat borders
- Gradient accent on the primary CTA button

**Files:**
- `src/app/layout.tsx` — font imports
- `src/app/globals.css` — color tokens, background texture, shadows
- `src/app/page.module.css` — homepage visual refresh
- `src/app/calendars/[id]/page.module.css` — calendar page refresh
- All component CSS files — border → shadow migration

---

## Phase 3 — Results & Summary UX

### 3a. Calendar Heat Map

Instead of binary colored-dot slots per cell, show availability intensity:
- Background color intensity scales with `freeCount / totalCount`
- Light tint (1 person) → medium (half) → strong (everyone)
- Keep the individual participant dots but layer them over the heat tint
- The "everyone free" treatment stays but becomes the peak of the gradient

**Files:**
- `CalendarGrid.tsx` — compute heat intensity per cell
- `CalendarGrid.module.css` — heat map color variables
- `src/lib/overlap.ts` — may need a heat level classifier

### 3b. "Best Day" Callout

When there's a clear winner (everyone free, or one day beats all others), show a prominent card:
- "Best day: Sat 24 May" with a check/star icon
- Positioned above the calendar grid, attention-grabbing
- Animate in when it first appears

**Files:**
- New component `src/components/BestDay/BestDay.tsx`
- `src/app/calendars/[id]/page.tsx` — render above grid

### 3c. Response Progress

Show how many people have responded vs total who've visited:
- "3 of 5 people have responded" progress indicator
- This requires knowing the total expected — could use participant count as proxy, or just show "3 people have responded"
- Simple bar or ring indicator

Note: we only know about people who've joined — we don't track "invited but haven't responded." Keep it simple: show participant count prominently, maybe with a subtle animation when it increments.

**Files:**
- Enhance `ParticipantList` or create `ResponseCount` component
- `src/app/calendars/[id]/page.tsx`

---

## Implementation Notes

- No new dependencies needed for Phase 1-2 (all CSS + vanilla JS)
- Swipe detection: ~20 lines of pointer event handling, no library needed
- OG metadata requires server-side data fetch — needs API call from server component
- All animations should respect `prefers-reduced-motion: reduce`
- Test on 375px (iPhone SE) as the baseline for all mobile work

## Log

- 2025-04-18 — Created plan
- 2025-04-18 20:36 — Implemented all three phases. Lint clean (0 errors), 71 tests passing, build succeeds with no warnings.
