# Sheet primitive

Status: done

## Goal

Five modal surfaces (RenameCalendarSheet, DuplicateCalendarSheet,
CalendarSettingsSheet, DayDetailSheet, ConfirmDialog) each copy-pasted the
same backdrop / sheet / keyframe CSS and wiring (focus trap, Escape, backdrop
click, `aria-labelledby`), and have drifted: z-index 50 vs 60, max-width 480
vs 540, max-height 85 vs 88dvh, bottom padding 24 vs 28px, three different
close buttons. Replace them with one `Sheet` component so behaviour and
look are consistent and new overlays are cheap.

Stacked on `plans/ui/design-system-tokens-primitives.md` (uses its tokens and
`Button`).

## Plan

1. Tokens in `globals.css`: `--sheet-max-width` (540px),
   `--dialog-max-width` (360px), `--scrim` / `--scrim-strong`.
2. `src/components/Sheet/Sheet.tsx` (+ module CSS):
   - Backdrop + panel. `layout="sheet"` (default): bottom sheet on mobile,
     centred card from 640px. `layout="dialog"`: compact centred card at
     every width, stacked above sheets (`--z-dialog` vs `--z-sheet`) because
     confirm dialogs open on top of the settings sheet.
   - `role="dialog"` (or `alertdialog`), `aria-modal`, `aria-labelledby`
     pointing at the rendered `title` (h2, `useId`), optional
     `aria-describedby`.
   - `useFocusTrap` with optional `initialFocus`; Escape and backdrop click
     call `onClose` unless `dismissible={false}` (e.g. while submitting).
   - Optional close button (`closeLabel`).
   - Enter animation (fade + rise), disabled under `prefers-reduced-motion`.
   - Safe-area bottom padding on the mobile sheet.
3. Migrate the five modal sheets/dialogs; delete their duplicated CSS.
4. Tests: labelled dialog, focus trap (Tab wraps), initial focus, Escape,
   backdrop click (and panel click doesn't close), non-dismissible state,
   alertdialog + description.

### UserMenu — deliberately not migrated

UserMenu is a **menu**, not a modal dialog: `role="menu"` anchored under its
trigger on desktop (a popover dropdown), bottom-sheet-styled only below
560px. It has menu semantics (Escape returns focus to the trigger, outside
click closes, no focus trap, page stays interactive on desktop). Wrapping it
in a modal `Sheet` would change its semantics and desktop placement. It
already uses the shared z-index/spacing tokens from the previous PR; a
future `Popover`/`Menu` primitive is the right home for it (and RowMenu).

### Left for later

- ImportFromGoogle privacy interstitial — a modal without a focus trap;
  should move to `Sheet layout="dialog"` but it's built inside a hook that
  returns JSX, so it wants its own small refactor.
- RowMenu mobile action sheet + UserMenu → a `Menu`/`Popover` primitive.

## Log

- 2026-11-12 11:15 — Created plan
- 2026-11-12 11:40 — Added overlay tokens and `Sheet` (+10 tests). Migrated
  Rename, Duplicate, Settings, DayDetail and ConfirmDialog; deleted ~460
  lines of duplicated backdrop/sheet/keyframe/close-button CSS. DayDetail's
  Fraunces 20px title is passed as a styled span. Trap now stays enabled
  while a form submits (Escape/backdrop just stop closing), so Rename and
  Duplicate refocus the field when an error comes back.
- 2026-11-12 11:50 — UserMenu kept as a popover menu (see above). vitest
  (435), oxlint, oxfmt and `next build` pass locally; CI not run (billing).
