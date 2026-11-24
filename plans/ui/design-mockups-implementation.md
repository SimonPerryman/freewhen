# Design mockups implementation

Status: done

## Goal

`design-mockups.html` (mock, not kept in the repo) is a hand-built explorations page with several
distinct directions for FreeWhen's visual identity. The palette tokens (cream
`#faf9f7`, terra `#e06449`) are already in `frontend/src/app/globals.css`, but
none of the typography, layout, or surface treatments from the mockup have
shipped. This plan breaks the mockup into trackable workstreams, in the order
they should land. Each gets its own plan file once we decide to start it; this
file is the index, the shared decisions, and the foundation.

## Decisions (locked at plan time)

These were open at first draft. Recording them here so sub-plans don't re-open
them:

- **Multi-preset accent system: dropped.** `calendar-colour-themes.md` stays
  cut; `remove-calendar-themes.md` stays done. The mockup's "lighter version"
  is still a multi-palette system, and emoji + title already differentiate
  calendars. Cover gradient + Best-so-far pill ship with terra hard-wired.
- **OG image: light only, terra accent.** No runtime dark variant — recipient
  clients can't send `prefers-color-scheme` to a server. Dark composition
  may live in a `/og-preview` dev route for marketing screenshots.
- **Hero grid: animation only, no count.** No fake "3 responding" pulse —
  faking a number is a trust bait-and-switch. The 21-cell stagger reads
  "live" on its own.
- **Dark mode: shipped with the typography swap.** Half the mockup's framing
  ("dark as a peer") collapses without it; tokens rot if added later. Costs
  ~1 day in the foundation PR; saves 2× the work component-by-component.
- **Wordmark scope: rendered HTML surfaces only.** Header, hero, login/auth,
  error pages, OG image. Plain "FreeWhen" in `<title>`, manifest, email
  from-name/subject, ARIA labels, `og:site_name`. Italic is illegible below
  ~16px and meaningless in plaintext.
- **Wordmark implementation: CSS class on a `<span>`, not a component.** Never
  takes props — `.serif-accent` utility is enough.
- **CTA buttons stay ink-on-cream with "→" glyph.** Terra is reserved for
  italic accents + heat-map intensity. Don't tint buttons terra.
- **Calendar titles store Markdown source.** Single column,
  `*emphasis*`-only dialect, UI button wraps selection. See phase 3 for
  the full rationale and renderer constraints.

## Plan

### 1. Foundation — typography swap + dark mode (mockup section iv. + the cross-cutting dark tokens)

The cheapest, highest-leverage change and a prerequisite for everything below.
Bundles dark mode because the mockup treats dark as a peer and every component
we touch in later phases will otherwise need a dark pass retroactively.

- Add `Fraunces` (variable, weights 400–700, axes `['opsz']`,
  `font-optical-sizing: auto`) and `Instrument Serif` (regular + italic) via
  `next/font/google` in `frontend/src/app/layout.tsx`. Subset Latin only.
  Expose as CSS variables (`--font-fraunces`, `--font-instrument`) alongside
  the existing DM Sans setup. `display: 'swap'`.
- Update `frontend/src/app/globals.css`:
  - `h1`–`h3` use Fraunces with `letter-spacing: -0.02em`,
    `line-height: 1`, `font-weight: 400`.
  - Add `.serif-accent` utility (Instrument Serif italic, terra). Apply via
    `<em class="serif-accent">word</em>` on hero/title accents.
  - Keep DM Sans for body, buttons, labels, eyebrows.
- Port the mockup's dark tokens into `globals.css` as a `prefers-color-scheme:
  dark` block (or a `[data-theme="dark"]` selector if we want a manual
  toggle later — TBD in sub-plan):
  - `--dk-bg: #141210`, `--dk-bg-2: #1c1915`, `--dk-paper: #2c2720`,
    `--dk-border: #4a443d`, `--dk-text: #edebe8`, `--dk-text-2: #9e9688`,
    `--dk-terra: #e8755e`.
  - Audit existing tokens — current `globals.css` has zero dark tokens
    (`--background`, `--page-bg`, `--accent` are all light-only). This is a
    fork of the token system, not an addition.
- **Preview before delete.** On a branch, render the current homepage +
  calendar page with the new fonts (and dark mode) and screenshot at 375px
  and 1280px. Decide before ripping the DM Sans display usage.
- Audit every existing heading (`Header`, `BestDay`, `SummaryView`,
  `MyCalendarsList`, login/auth pages, `error.tsx`, `not-found.tsx`,
  homepage marketing copy) and apply the new display family. Pull
  `<em class="serif-accent">$word</em>` accents on the hero and per-calendar
  titles.
- Wordmark: introduce `.wordmark` CSS class. Render
  `Free<span class="serif-accent">When</span>` everywhere "FreeWhen" appears
  on a rendered surface (see decision above for scope).

Sub-plan: `plans/ui/typography-and-dark-mode.md`.

### 2. Homepage hero with live-feeling grid (mockup section i.)

- Build a `<HeroLiveGrid />` component (mobile-first: 7 columns × 3 rows on
  ≥375px; pattern from `liveGrid` in the mockup script). Cells animate via
  `animation-delay: calc(60ms * var(--i) + 200ms)` — pure CSS, no JS.
  **Respect `prefers-reduced-motion: reduce`**: render the final state, no
  stagger.
- Restructure `src/app/page.tsx`:
  - Top row: `.wordmark` left, current month name right (e.g. "April") in
    DM Sans grey.
  - Headline: `Find when<br/>everyone's <em class="serif-accent">free.</em>`
    in Fraunces, two lines.
  - Subhead, verbatim from the mockup:
    `"Drop a name, share a link, watch availability fill in."`
  - `<HeroLiveGrid />` between subhead and CTA.
  - CTA: existing input + button flow, restyled — input on `--paper` with
    `--border`, button on `--ink` with `--cream` text and a "→" glyph.
    **Do not tint the button terra.**
- Test on 375px first; verify the grid does not push the CTA off-screen.
- Dark variant: same component, tokens pick up automatically from (1).

Sub-plan: `plans/ui/homepage-hero-live-grid.md`.

#### 2a. Follow-ups (pending — visual review 2026-05-09)

Corrective amendments to Phase 2 output after reviewing the live hero on
desktop + iPhone 14 Pro Max.

1. **Drop the orphaned month label.** With the inline wordmark removed
   (see Log entry 2026-05-09 "Dropped the inline hero wordmark"), the
   month sits alone top-left of the hero with no visual partner and no
   functional purpose on `/`. Remove the `.top` row, the `monthName`
   `useMemo`, and the `.month` style from `frontend/src/app/page.tsx`
   and `page.module.css`. Rejected alternatives: re-adding the hero
   wordmark (already decided against — duplicates the global Header), or
   moving the month into `HeroLiveGrid` as an eyebrow (the card already
   has `APR 28 — MAY 4` + `PREVIEW`).

2. **Fix the subhead line break.** `max-width: 28ch` on `.subheading`
   currently splits "watch / availability fill in" mid-phrase on both
   desktop and mobile. Loosen to `max-width: 34ch` and add
   `text-wrap: balance` so the two lines balance into complete clauses.
   Verify on 375px, 430px, and ≥640px.

3. **Vertical rhythm on tall mobile viewports.** On iPhone 14 Pro Max the
   bottom ~45% of the viewport is empty. Centre the column with
   `min-height: 100svh; justify-content: center;` on `.page` (or
   `padding-block: max(20px, 8svh)`). Keep mobile-first spacing intact
   for short viewports.

4. **Stabilise the CTA button label.** `buttonLabel` flips between
   `Create →` and `Sign in to create →` once `isLoading` resolves,
   causing a layout jump on first paint. Either set a `min-width` on
   `.button` wide enough for the longest label, or render a neutral
   placeholder until auth settles.

Smaller polish to batch later (lower priority):

- Reduce `PREVIEW` eyebrow weight inside `HeroLiveGrid` so the date range
  leads, or drop `PREVIEW` entirely.
- Resolve the hero card vs text-column left-edge mismatch on desktop —
  pick one grid.
- Bump input/button gap from 8px to 10–12px, or unify into a single pill.
- Add tiny muted day-of-week labels (M T W T F S S) above the grid so the
  "availability across 7 days" metaphor lands faster.
- Confirm bottom-left `N` glyph on mobile is the Next.js dev indicator,
  not shipped chrome.

### 3. Calendar page header restructure (carved out from mockup section ii.)

This is the layout half of the original "themed calendar" section, with the
multi-preset accent system removed. Terra hard-wired. Ships independently of
any backend change.

- New `<CalendarCover />` component, ~130px tall, sits at the top of the
  calendar page above the title:
  - Two stacked radial gradients in terra + cream
    (`radial-gradient(140% 100% at 20% 0%, var(--terra) 0%, transparent 55%),
    radial-gradient(120% 100% at 100% 100%,
    color-mix(in oklab, var(--terra) 70%, white) 0%, var(--terra-soft) 70%)`).
  - SVG noise overlay at `opacity: 0.12` (data-URI from the mockup's
    `feTurbulence` filter — already inline in `design-mockups.html`).
  - Existing emoji rendered bottom-right at 44px with a soft drop-shadow.
  - Optional small Fraunces-italic "badge" bottom-left (e.g. "Bachelorette",
    "Camping") — defer until we have a calendar `kind`/`category` field;
    ship the cover without the badge first.
- Title block underneath uses Fraunces with the
  `<em class="serif-accent">word</em>` accent treatment (e.g. the author
  emphasises "hen do" in `Jen's hen do`). Authoring + storage:
  - **Storage: Markdown source in `calendars.title`.** Author writes
    `Jen's *hen do*`. One column, one source of truth, survives every
    rendering context.
  - **Constrain the dialect.** Only `*...*` → `<em class="serif-accent">`
    is allowed. Reject/strip every other Markdown feature (`**bold**`,
    links, headings, code) at the render layer — we are not building a CMS
    in the title field. A ~10-line allowlist renderer, no `marked`/`remark`
    dependency.
  - **UI affordance.** The title input gets a "format selection as italic"
    button (or `Cmd/Ctrl+I` while text is selected) that wraps the
    selection in `*...*`. No raw asterisks typed by users in the common
    case; raw asterisks still work for power users / paste-from-elsewhere.
    `*` literal is escapable as `\*` (rarely needed).
  - **Plaintext surfaces strip the asterisks** with a regex helper
    (`title_plain(t)` → `t.replace(/\*([^*]+)\*/g, '$1')`). Used for
    `<title>`, `og:title`, email subjects, share API, ARIA labels. If
    plaintext stripping turns out to be hot, denormalise to a generated
    `title_plain` column — defer until measured.
  - **Migration.** Existing titles are already plaintext-with-no-asterisks
    by construction, so they render correctly under the new pipeline with
    no backfill. New emphasis is purely additive.
- Promote `BestDay` into a "Best so far" pill (`.ev .best` in the mockup):
  `--terra-soft` background, label in Fraunces italic, the `when` value in
  DM Sans 600. Lives inside the cover/header zone, not as a separate block
  below the grid.
- Meta pills row ("8 people", "3 weekends") under the title — simple count
  derived from existing data.
- **Verify heat-map intensity scale.** Mockup uses
  `color-mix(in oklab, var(--terra) 22% / 50% / 100%, var(--paper))` for
  h1/h2/h3. `CalendarGrid` currently uses some scale — confirm it's three
  steps and aligned, otherwise add a translation.

Sub-plan: `plans/ui/calendar-header-restructure.md`.

#### 3a. Follow-ups (pending — visual review 2026-05-09)

Corrective amendments to Phase 3 output after reviewing the live calendar
detail page on desktop + iPhone 14 Pro Max with a fresh "Test" calendar
(1 person, 1 marked weekend, 🌻 emoji).

1. **The cover is currently ornament with no information density.** ~~In the
   mockup (`design-mockups.html` lines 995–1170) the cover was the *header
   of a calendar tile in a list*, anchored by `badge + emoji + overlapping
   italic title + meta pills`. We promoted it to the detail page header
   but stripped almost everything that justified it…~~

   **Resolved 2026-05-09 — took option (c): dropped on detail page.**
   `<CalendarCover />` removed from `CalendarPageClient.tsx` and the
   component files deleted. Detail page is now title + pills + grid, no
   cover. If/when the `/calendars` list view gets a pass, the cover
   pattern can be re-introduced there as a tile header (its original
   mockup role). Options (a) and (b) deferred indefinitely.

2. **Title row chrome.** Pencil + gear icons stacked next to the h1
   ("Test  ✏  ⚙") add visual noise. Rename already lives inside
   `CalendarSettingsSheet`; the standalone pencil button is a duplicate
   entry point. Remove the inline pencil and let the gear (settings) be
   the single affordance. Verify nothing else depends on the pencil
   handler before deletion.

3. **Heat-map scale never landed; we have per-participant slots
   instead.** Phase 3 flagged "verify heat-map intensity scale" against
   the mockup's three-step `color-mix(in oklab, var(--terra)
   22%/50%/100%, var(--paper))`. The shipped grid renders one *slot per
   participant per cell* (faded for unavailable, filled for available)
   with the participant's initial — every cell shows a faded "S" when
   the lone participant hasn't marked it. That's a different mental
   model than the mockup's aggregate heat. Two options:

   - **Keep the slot grid** (it carries more information once N > 1) and
     formally retire the mockup's heat-map scale from this plan — the
     intensity tiers only need to apply to the OG image (Phase 4) where
     per-participant slots are illegible at thumbnail size.
   - **Switch to aggregate heat** for the live grid too, dropping the
     per-participant initials. Loses information; gains the mockup look.

   Recommend the first. Update Phase 4 sub-plan to make the heat-map
   scale OG-image-only and stop treating the grid as out-of-spec.

4. **Empty-state for N=1.** A fresh calendar (creator only, one
   weekend marked) is the most common first impression and currently
   feels empty: cover dominates, "1 person" / "1 weekend" pills read as
   "you, alone", the per-cell faded "S" plasters the whole grid with
   the creator's not-yet-marked status. Out of scope for this
   follow-up but worth a dedicated empty-state pass — log a future
   plan rather than fold it in here.

5. **Drop-shadow tuning for dark mode.** `drop-shadow(0 4px 8px
   rgba(0,0,0,0.15))` on `.emoji` was tuned for cream backgrounds. The
   cover sits on cream/pink which sits on `--dk-bg` — the shadow muddies
   the emoji's edge against the gradient. Either drop the shadow under
   `prefers-color-scheme: dark` or reduce alpha to `0.08`.

6. **"Invite your friends" card border.** Dashed terra outline doesn't
   appear elsewhere in the mockup or shipped surfaces. Confirm whether
   this was intentional; if not, swap to the standard `--border` solid
   rule used elsewhere.

Smaller polish to batch later (lower priority):

- Confirm `renderTitle()` on a real calendar with `*emphasis*` markdown —
  the `Test` calendar can't exercise the italic accent and it's the
  feature Phase 3 hung the title treatment on.
- ~~Audit cover horizontal stretch on ≥1024px — the radial gradient
  composition assumed a tile-sized container.~~ Moot: cover deleted from
  detail page (see follow-up 1).

### 4. Dynamic OG image (mockup section iii.)

- Add `frontend/src/app/c/[slug]/opengraph-image.tsx` using Next.js
  `ImageResponse` (App Router convention). Light only, 1200×630.
  - `.wordmark` top-left.
  - Title with Fraunces + `serif-accent` em on the last/featured word.
  - Meta row: people count, weekends/days count, "Best so far · {date}".
  - Real heat-map grid (3 rows × 7 cols) built from the calendar's actual
    response intensities, same `h1`/`h2`/`h3` scale as `CalendarGrid`.
  - Soft radial-gradient backdrop (terra + fern) for atmosphere; matches
    the mockup's `.og-bg`.
- Twitter card meta tags reuse the same image.
- **Cache invalidation strategy: query-param cache-buster.** OG URL is
  `…/c/{slug}/opengraph-image?v={updated_at}` where `updated_at` is the
  calendar's last-mutation timestamp from the Go backend. Avoids a cross-
  service `revalidateTag` call from Go → Vercel; cost is one short read on
  page render. Pin in sub-plan.
- Must not block calendar page render; OG generation is its own route with
  its own runtime.

Sub-plan: `plans/ui/dynamic-og-image.md`.

### 5. Voice detail pass (cross-cutting, do last)

The mockup's connective tissue — easy to miss, easy to get wrong:

- "→" glyph on every primary CTA (Create, Submit, Save).
- `::before { content: "→ "; color: var(--terra); }` on section/note
  headings where appropriate (settings sheets, empty states).
- Eyebrow style: DM Sans uppercase, `letter-spacing: 0.18em`, ink-2 colour,
  used above section headings.
- Pill style: `99px` border-radius, `--terra-soft` background,
  `color-mix(in oklab, var(--terra) 70%, black)` text. Used on meta pills,
  badges, the "Best so far" pill.
- Audit existing toasts, settings sheets, dialogs against these.

Sub-plan: `plans/ui/voice-detail-pass.md`.

### Order of operations

1. **Typography + dark mode** — foundation, ships standalone.
2. **Homepage hero** — depends on (1).
3. **Calendar page header restructure** — depends on (1), independent of (2).
4. **Dynamic OG image** — depends on (1), benefits from (3) being done first
   (so the heat-map scale is verified), but technically independent.
5. **Voice detail pass** — last, after the big surfaces are settled.

(2)–(4) can be parallelised across PRs once (1) lands.

### Out of scope

- Multi-preset calendar accent themes (locked above — stays cut).
- Photo cover uploads (mockup explicitly defers).
- Real live-data backing for the homepage hero grid (it's marketing).
- A real "N responding" count on the hero (locked — drop fake, defer real).
- Cover badge category (deferred — needs a `kind` field on calendars).

## Log

- 2026-05-08 — Created plan. Index for the four sections of
  `plans/ui/design-mockups.html`. Section ii flagged as conflicting with the
  cut `calendar-colour-themes` decision and gated on a revisit call.
- 2026-05-08 — Decided calendar-title emphasis storage: Markdown source in
  the existing `title` column, `*...*`-only dialect, UI button wraps
  selection. Rejected raw-HTML input (escaping nightmare across plaintext
  surfaces) and offset-column approach (offsets desync on edits). Folded
  into phase 3 and the locked decisions section.
- 2026-05-08 — Audit pass after re-reading the mockup CSS. Locked six
  decisions (preset themes dropped; OG light-only; hero animation no count;
  dark mode bundled with typography; wordmark scope = rendered HTML only,
  CSS class not component; CTAs stay ink-on-cream). Restructured: phase 1
  now bundles dark mode; phase 3 carved out from old section ii as a
  layout-only restructure with terra hard-wired; phase 5 added for the
  voice/connective-tissue details. Added: hero copy verbatim,
  `prefers-reduced-motion` guard, heat-map scale verification step,
  preview-before-delete typography step, OG cache-buster strategy.
- 2026-05-09 — Phase 1 implemented: swapped `next/font` to Fraunces +
  Instrument Serif (DM Sans kept for body); merged old token system into
  unified `:root` with brand vars (`--terra`, `--cream`, `--ink`, etc) feeding
  the existing semantic tokens; added `.serif-accent`, `.wordmark`, `.eyebrow`,
  `.pill` utilities and global `h1-h3 = Fraunces 400`; global
  `prefers-reduced-motion` shrinks all animations. Header swapped to
  `Free<span class="serif-accent">When</span>`. New helper at
  `frontend/src/lib/title.tsx` (`renderTitle`/`titlePlain`/`wrapItalic`) for
  the markdown-italic title dialect.
- 2026-05-09 — Phase 2 implemented: new
  `frontend/src/components/HeroLiveGrid` (7×3 cells, 21-cell stagger via
  `--i` and `animation-delay`, no fake "responding" pulse). Restructured
  `frontend/src/app/page.tsx` to wordmark + month, Fraunces hero, verbatim
  subhead, hero grid, ink-on-cream "Create →" CTA. Updated tests.
- 2026-05-09 — Phase 3 implemented: new
  `frontend/src/components/CalendarCover` (130px terra/cream radial with SVG
  noise + emoji). `CalendarPageClient` now renders the cover, title via
  `renderTitle()` for markdown italic, meta pills (`{N} people` +
  weekends/days). `BestDay` restyled as a "Best so far" terra-soft pill and
  promoted to the header zone. Updated tests.
- 2026-05-09 — Phase 4 implemented: added
  `frontend/src/app/calendars/[id]/opengraph-image.tsx` (1200×630, light only,
  Fraunces + Instrument Serif fetched at request time, parses `*...*`
  emphasis, real heat-map buckets from `freeByDate` with a fallback pattern,
  best-day pill via `computeSummary`, terra+fern radial backdrop). Wired
  `generateMetadata` in `page.tsx` to emit OG + Twitter card images keyed by
  `?v={updatedAt}` for cache invalidation.
- 2026-05-09 — Dropped the inline hero wordmark on `/` (mockup had it in
  the hero top row alongside the month). The global `Header` already renders
  the wordmark site-wide, so showing it again in the hero stacked two
  wordmarks within ~80px of each other. Hero top row now contains just the
  current month name; headline still leads. Wordmark scope unchanged in spirit
  (rendered HTML surfaces) — it just lives in the header on routes that have
  one.
- 2026-05-09 — Phase 5 implemented: appended " →" glyph to primary CTAs
  across JoinForm, RenameCalendarSheet, DuplicateCalendarSheet, PassphraseGate,
  CalendarSettingsSheet (Require passphrase / Duplicate this calendar), and
  login (Send magic link). Added `→ ` prefix on settings sheet section
  labels via `::before`. Token audit passed — Toast/CalendarSettingsSheet
  already use semantic vars. All 223 tests pass; lint clean.
- 2026-05-09 — Visual review of `/` (desktop + iPhone 14 Pro Max) logged
  Phase 2 follow-ups: drop the orphaned month label, fix `.subheading`
  line break (28ch → 34ch + `text-wrap: balance`), centre the hero on
  tall mobile viewports, stabilise the CTA button label against the
  auth-resolved label flip. Captured under §2a; smaller polish items
  (PREVIEW weight, card vs text grid mismatch, input/button gap, DoW
  labels, mobile `N` glyph check) batched there too.
- 2026-05-09 — Implemented §2a + §3a follow-ups. Hero (`page.tsx` /
  `page.module.css`): dropped the orphaned month label (`.top` row,
  `monthName` `useMemo`, `.month`/`.wordmarkRoot` styles); subhead loosened
  to `max-width: 34ch` + `text-wrap: balance`; `.page` now uses
  `min-height: 100svh` + `align-items: center` to centre the column on
  tall mobile viewports; `.button` got `min-width: 196px` so the label
  flip from `Sign in to create →` → `Create →` no longer reflows the row.
  Calendar detail (`CalendarCover`, `CalendarPageClient`,
  `CalendarSettingsSheet`): cover shrunk from 130px → 44px (option b —
  thin coloured rule); emoji repositioned centre-right at 24px and
  drop-shadow removed (also addresses §3a item 5). Inline pencil rename
  button removed; `Rename calendar →` action added to the top of
  `CalendarSettingsSheet`, opening the existing inline rename input via
  `onRename` (which closes the sheet first). `InvitePrompt` border
  changed from dashed-terra to standard `1px solid var(--border)`. Heat-
  map scale (§3a item 3) formally retired from this plan — per-
  participant slot grid stays; the mockup's three-step intensity is
  OG-only. N=1 empty-state (§3a item 4) deferred to a future plan.
  Updated `page.test.tsx` (dropped FreeWhen wordmark assertion — global
  Header carries it, not the page) and `CalendarPageClient.test.tsx`
  (owner rename now requires opening the settings sheet first). All
  223 tests pass; lint clean.
- 2026-05-09 — Visual review of calendar detail page logged Phase 3
  follow-ups under §3a: cover is ornament without the badge/best-so-far
  anchors (recommend shrink-to-accent now, drop on detail pages
  medium-term); inline pencil button duplicates the settings sheet's
  rename and should be removed; per-participant slot grid diverges from
  the mockup's aggregate heat-map and should be formally retired from
  this plan (heat scale becomes OG-only); emoji drop-shadow needs a
  dark-mode tweak; "Invite your friends" dashed border is unverified.
  N=1 empty state flagged for a future dedicated pass.
- 2026-05-09 — Implemented the §2a + §3a "smaller polish" batches.
  `HeroLiveGrid`: dropped the `PREVIEW` eyebrow (date range now leads on
  its own); added muted day-of-week labels (M T W T F S S) above the
  heat cells so the "availability across 7 days" metaphor lands.
  `page.module.css`: bumped `.cta` gap from 8px → 12px. §3a polish
  verified without code changes — calendar container caps at
  `max-width: 540px` so the cover never stretches at ≥1024px;
  `renderTitle()` confirmed to handle `*emphasis*` correctly via the
  existing `TOKEN_RE` regex and existing tests. Hero card vs
  text-column left-edge mismatch and the bottom-left `N` glyph (Next.js
  dev indicator) left as-is — neither warranted a code change. All 223
  tests pass; lint clean.

## Amendment — wordmark vertical alignment

The `Free` (display sans) and `When` (Instrument Serif italic) glyphs in
the `.wordmark` sit on the same baseline but read as misaligned: the
italic serif's cap-height is a smaller fraction of its em box than the
sans, so the "W" cap appears to float above the "F" cap. Italic optical
centering compounds the effect.

Fix: scope a rule to `.wordmark .serif-accent` that (a) sizes the accent
down to match cap-height, and (b) translates it down to match the
baseline:

```css
.wordmark .serif-accent {
  display: inline-block;
  font-size: 0.92em;
  transform: translateY(0.06em);
}
```

`display: inline-block` is required for `transform` to apply to an
otherwise-inline `<em>`. Scoped to `.wordmark` so other `.serif-accent`
uses (calendar/heading titles via `renderTitle()`) are unaffected.

## Amendment — hero headline italic descender clipping

The hero `.headline` (`src/app/page.module.css`) sets `line-height: 1`.
The italic Instrument Serif "f" in `*free*` has a long curving descender
that pokes below the line box, so against the section's bottom padding
it reads as if the bottom of "free" is being clipped.

Fix: add a small `padding-bottom` to the headline so the descender has
room without changing the inter-line gap on the wrapping headline:

```css
.headline {
  padding-bottom: 0.08em;
}
```

Scoped to the page-level `.headline` only — global `h1` line-height
stays at 1 so other headings aren't affected.

## Amendment — "Best so far" pill: label treatment + vertical rhythm

Two issues with the `BestDay` pill as it sits on the calendar detail
page (`CalendarPageClient.tsx:1273-1283`, between the empty-state hint
and the Import-from-Google entry):

1. **Vertical rhythm is asymmetric.** `.card` has `margin-top: 12px`
   but no bottom margin, and the Import entry below renders as a bare
   `<div>{importView.entry}</div>` with no top margin. The pill sits
   tight against the Import button while having room above — reads as
   unbalanced rather than as a deliberate group.
2. **Italic accent label fights the date next to it.** The label uses
   Instrument Serif italic at 14px in the same baseline-aligned row as
   an upright DM Sans 600 date. Two competing voices for what is
   functionally a microlabel/eyebrow, and italic stroke contrast
   collapses badly at 14px on mobile. The italic accent is doing
   editorial work where a tag is needed.

Fix:

- Give `.card` symmetric vertical rhythm (`margin: 12px 0`) so it owns
  its own spacing and callers don't have to compensate.
- Re-treat `.label` as an eyebrow: DM Sans, uppercase, tracked, smaller
  — drops the italic, harmonises with the upright date, reads
  unambiguously as a tag.

```css
.label {
  font-family: var(--font-stack-body);
  font-style: normal;
  font-size: 11px;
  font-weight: 600;
  letter-spacing: 0.08em;
  text-transform: uppercase;
  /* colour unchanged */
}
```

Scope is `BestDay.module.css` only — other `.serif-accent` / italic
usages (wordmark, calendar titles via `renderTitle()`) are untouched.

## Log

- 2026-05-09 — Amendment: wordmark "Free" vs italic "When" reads
  misaligned due to font-metric (cap-height) mismatch between display
  sans and Instrument Serif italic. Documented the fix (size-up the
  accent inside `.wordmark`, scoped so other serif-accent usages are
  untouched).
- 2026-05-09 — Implemented amendment: added a `.wordmark .serif-accent`
  rule to `globals.css`. First pass tried `font-size: 1.08em` (wrong
  direction — Instrument Serif italic's cap-height per em is *larger*
  than the sans, not smaller). Sized down to `0.92em` to align cap-tops,
  then added `transform: translateY(0.06em)` (with
  `display: inline-block`) to align the baseline — the italic font's
  baseline metric sits higher within its em box than the sans, so size
  alone wasn't enough. Final values verified visually. Scoped to
  `.wordmark` so other `.serif-accent` uses are unaffected.
- 2026-05-09 — Implemented hero headline descender amendment
  (`padding-bottom: 0.08em` on `.headline`), then reverted at user
  request pending further review. Amendment section above remains as
  the documented fix when we return to it.
- 2026-05-09 — Re-applied the headline descender amendment after
  review.
- 2026-05-09 — Reverted headline copy back to "Find when everyone's
  *free.*" — "we're all free" reads weaker because the italic accent
  visually under-weights against the upright lead, and "everyone's"
  carries the meaning more clearly. Accepts the `y`/`f` descender
  asymmetry as a non-issue. Now matches metadata copy in
  `layout.tsx`/`manifest.ts`.
- 2026-05-09 — Deleted `<CalendarCover />` from the calendar detail
  page (resolves Phase 3 follow-up #1, option c). Removed import +
  render in `CalendarPageClient.tsx` and deleted the component
  directory. Cover was ornament-only — badge deferred, "Best so far"
  pill never moved into it, title never overlapped — so it was a
  decorative gradient bar with just an emoji. Detail page is now title
  + pills + grid. Cover-as-tile-header pattern can return on the
  `/calendars` list view if/when that gets a pass.
- 2026-05-09 — Implemented "Best so far" pill amendment in
  `BestDay.module.css`: `.card` margin → `12px 0` (symmetric vertical
  rhythm so the pill no longer sits tight against the Import entry
  below); `.label` re-treated as an eyebrow (DM Sans, 11px, weight
  600, `letter-spacing: 0.08em`, uppercase) — drops the Instrument
  Serif italic which was illegible at 14px and competing with the
  upright bold date next to it. Colour token unchanged. Scope limited
  to `BestDay`; other italic-accent usages untouched.
- 2026-05-09 — Reverted the headline descender amendment after closer
  inspection — the perceived clipping wasn't a clip at all but a
  descender-depth asymmetry: Instrument Serif italic's `y` (in
  "everyone's") descends much deeper than its `f` (in "free"), so the
  `f` reads as truncated next to it. Probed `font-feature-settings:
  ss01, salt` for a swash `f` (no-op — Instrument Serif has no such
  alternate). Built two temporary specimen pages (not kept) — one showing the full
  alphabet across alt accent faces and one testing a single-glyph `f`
  swap via a donor face — but ultimately resolved by reword: headline now reads
  "Find when we're all *free.*" in `src/app/page.tsx`. The `we're all`
  phrase has no `y`, so the `f` descender no longer competes. No font
  changes needed; metadata in `layout.tsx`/`manifest.ts` left as
  "Find when everyone's free." for SEO/PWA copy.
