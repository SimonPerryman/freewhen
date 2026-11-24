# UI fixes: summary overflow, Quick fill, banner dots

Status: done

## Goal

Three CSS-first fixes from the 2026-11-24 UI review (first PR of the UI-fixes stack):

1. **Summary rows overflow at 375px.** A row with Finalise plus a "Sam prefers" pill is wider than the card (`.dateItem` is `nowrap`, `.rowEnd` doesn't wrap, nothing can shrink). The pill crosses the card border and gets clipped at the screen edge.
2. **Quick fill has no affordance.** The `<summary>` is `display:flex`, which drops the native marker, so it reads as grey text. When it's open, "Weekends Weekdays" reads like a comma-less list of words, not controls. The owner asked for real design here.
3. **Confirmed banner dots and vote line.** On the accent background, 8px participant swatches fall below 3:1 in both themes, and "N preferred this date" uses `--text-secondary` on accent (about 1:1, so it's invisible).

## Plan

- **SummaryView:** each row becomes a two-column grid (`minmax(0,1fr) auto`, areas `"date action" / "meta action"`).
  - The date (and "3/4 free") sits on line 1 and ellipsises as a last resort.
  - Pills (prefer, notes) go on line 2, wrapping, and only render when there's at least one. Rows with no pills use a one-line grid.
  - Finalise sits on the right, centred across both lines, `min-height: 32px`.
  - The same layout applies to both lists.
- **Quick fill:**
  - The summary becomes an outlined `inline-flex` control: 44px tall, `--border`, `--radius-md`, `--text-primary`, with a 16px chevron that rotates when open. Native markers are hidden.
  - Open: the `<details>` becomes one `--tile-bg` tray, and the options become pill chips on `--card-bg`.
  - "Weekends" and "Weekdays" each get a tiny 7-day week glyph (Mon–Sun, matching days filled) so they read as options, not words. Import from Google uses the same chip, and Clear stays a quiet text button on the right.
  - No closed-state hint: the chevron and outline carry the affordance.
- **FinalisedBanner:**
  - `.swatch` goes to 10px with a 2px `--paper-ink` ring (ring vs banner: 6.0:1 dark, 3.2:1 light).
  - `.voteLine` becomes `color: inherit` (about 5:1 in both themes).
- **Tests:**
  - SummaryView: pills render inside the row's meta element, not beside Finalise. No meta node renders when there are no pills.
  - CalendarGrid: the chevron is decorative, and button names stay "Weekends"/"Weekdays".
  - e2e: at 375px the Summary card has `scrollWidth <= clientWidth`.
- Screenshots at 375px, light and dark, before and after.

## Log

- 2026-11-24 12:20 — Created plan. Before shots captured: summary content is 337px wide in a 301px card, the "1 preferred this date" line is invisible, and Quick fill is plain grey text.
- 2026-11-24 12:25 — SummaryView rows are now a grid via a small `SummaryRow` helper. The pill-less rows get a `oneLine` modifier, so the empty second grid row doesn't add a row gap. After shots: the summary is 301px wide in its 301px card.
- 2026-11-24 12:26 — Quick fill: outlined summary with a chevron, a tray when open, and chip options with a Mon–Sun week glyph. Import from Google uses the same chip styling (`ImportFromGoogle.module.css`). No closed-state hint.
- 2026-11-24 12:27 — Banner: 10px swatches with a 2px ink ring. The vote line inherits the banner colour.
- 2026-11-24 12:30 — Tests: a SummaryView structure test, a CalendarGrid chevron/glyph a11y test, and the e2e `summary-layout.test.ts` (row and pill inside the list at 375px). `make lint` and `make test` pass.
- 2026-11-24 13:30 — CI only triggered for PRs into `main`, so the stacked PRs above this one got no checks. Dropped the `branches` filter from `pull_request` in `ci.yml` (separate commit). Push-to-main deploys are unchanged.
