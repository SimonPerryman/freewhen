# Design system: tokens + Button/TextField primitives

Status: done

## Goal

A design review found colour tokens used well (~650 refs) but almost nothing
else: spacing tokens had 22 uses against ~300 raw px paddings/margins/gaps,
there was no type scale (~20 ad-hoc font sizes incl. 11.5/12.5/17/19px), no
z-index scale (50 vs 60 drift between sheets), and button/input styles were
re-written per component (`.button` ×6, `.cancelButton` ×3, `.input` ×5).

Build a small, real design system with minimal visual change:

- Tokens for spacing, type size, line height and stacking order.
- Shared `Button` and `TextField` primitives, adopted where it is a
  straightforward swap.

The sheet/modal primitive is a follow-up stacked PR
(`plans/ui/sheet-primitive.md`).

## Plan

1. **Tokens** in `src/app/globals.css`:
   - Spacing: 4px-based numeric scale `--space-0-5` (2px) … `--space-16`
     (64px). Half steps (`-0-5`, `-1-5`, `-2-5`, `-3-5`, `-4-5`) exist because
     the UI genuinely uses a 2/6/10/14/18px rhythm. Replaces the old t-shirt
     names (`--space-xs` … `--space-3xl`), which are migrated and removed.
   - Type: `--text-3xs` (10px) … `--text-6xl` (32px), base `--text-base` 14px.
   - Line height: `--leading-none|tight|snug|normal` (1 / 1.2 / 1.4 / 1.5).
   - Z-index: `--z-raised|popover|coachmark|overlay|sheet|dialog|toast|grain|skip-link`.
2. **Bulk migration** of every CSS module: `font-size`, `line-height`,
   `padding*`, `margin*`, `gap`, `z-index` → tokens (scripted, then reviewed).
   Left raw on purpose: 1/3/5px optical nudges, negative margins, the 9px
   grid-cell glyph, the hero `clamp()` and `line-height: 1.1` on display
   numerals.
   Snaps (≤1px, deliberate): 11.5→12, 12.5→13, 17→18, 19→20px font sizes;
   line-height 1.25→1.2, 1.3/1.35→1.4, 1.45→1.5.
3. **`Button`** (`src/components/Button/Button.tsx`): variants
   `primary | secondary | ghost | danger`, sizes `sm | md | lg`, `loading`
   (disabled + `aria-busy` + spinner, spinner hidden under reduced motion),
   `fullWidth`, defaults `type="button"`, forwards refs/props. Also exports
   `buttonClassName()` for links that need button styling.
4. **`TextField`** (`src/components/TextField/TextField.tsx`): label (optionally
   visually hidden), hint, error; wires `aria-describedby` (hint + error ids),
   `aria-invalid`, `role="alert"` on the error; forwards ref + input props.
5. **Adopt** where it's a straight swap:
   - Button: RenameCalendarSheet, DuplicateCalendarSheet, ConfirmDialog,
     CalendarSettingsSheet actions, DayDetailSheet Reset/Save, UserMenu
     Cancel/Save, JoinForm submit, PassphraseGate submit.
   - TextField: RenameCalendarSheet, DuplicateCalendarSheet, PassphraseGate.
6. Tests for Button and TextField; update `docs/onboarding.html` with a
   design-system note.

### Left for later (not straightforward swaps)

- Home hero CTA + input (`app/page.module.css`) — 196px min-width, 12px radius
  hero styling; would need a hero size.
- Login Google/email buttons and email input — brand-specific Google button.
- JoinForm name input (inline input + button row) and email field — should
  move together so both inputs match.
- UserMenu name input (compact in-menu field), WhosMissingPanel add form.
- SummaryView finalise/confirm/cancel (12px pill-ish buttons), FinalisedBanner
  (inverse buttons on accent), CalendarGrid toolbar/nav/today buttons,
  ExportButtons, ShareButton, ImportFromGoogle entry + interstitial, retry
  buttons on the calendar and calendars pages, ParticipantList rename/remove.
- DayDetailSheet "Remove my availability" (danger outline — no outline-danger
  variant yet) and icon-only close buttons (want an `IconButton`).

## Log

- 2026-11-12 10:00 — Created plan
- 2026-11-12 10:20 — Added spacing/type/leading/z-index tokens; scripted
  migration of all CSS modules (font-size raw px 188→2, spacing raw px values
  407→13)
- 2026-11-12 10:50 — Added Button + TextField with tests; adopted them in the
  sheets, ConfirmDialog, DayDetailSheet editor, UserMenu name edit, JoinForm
  and PassphraseGate; deleted the replaced CSS rules. DuplicateCalendarSheet
  keeps its form-level error (limit-reached isn't a field error).
- 2026-11-12 11:00 — Onboarding doc design-system note; vitest (425), oxlint,
  oxfmt (changed files) and `next build` pass locally. CI not run (billing).
