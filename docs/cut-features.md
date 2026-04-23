# Cut features

Record of features that shipped (or were built and nearly shipped) and then
removed. Entries are newest-at-top. The goal: when someone asks "didn't we try
X?" there's a single place to look, and restoring a cut feature is a
known-cost exercise rather than archaeology.

Each entry captures what was built, why it was removed, and what it would
take to bring it back. The "To restore" paragraph is the load-bearing bit —
it names the commits, migration numbers, and assumptions a future restorer
needs to re-check.

---

## Submit-to-view privacy

- **Shipped:** 2026-04-20 — commit `b3aa5fc` — plan `plans/submit-to-view-privacy.md`
- **Cut:** 2026-04-23 — plan `plans/remove-submit-to-view.md`
- **Why cut:** Pre-launch feature that nobody asked for. The create-form
  toggle already felt disproportionate enough that the v2-priorities doc was
  proposing to hide it behind an Advanced options disclosure — the same
  "hide the weight" instinct that preceded cutting calendar colour themes.
  No user signal for or against; cheapest time to remove is now.
- **What it did:** Per-calendar `visibility` enum (`open` vs
  `submit_to_view`) gating the `GET /calendars/{id}` response —
  unsubmitted non-owner viewers got a stripped shape
  (`participants: []`, `availability: []`, `participantCount`) until they
  marked a day. Owner always bypassed; anonymous viewers identified
  themselves via `?as={participantId}`. Frontend rendered a hidden-state
  banner and flipped to full UI optimistically on first toggle.
- **Downstream decisions changed:** The PATCH handler was generalised from
  `RenameCalendar` to `UpdateCalendar` as part of this feature. That
  generalisation stays — later features (finalise, passphrase) already ride
  on it and it's no longer tied to visibility.
- **To restore:** Cherry-pick `b3aa5fc` and any follow-ups that touched
  visibility code between then and the removal commit. Migration 008 is on
  disk; migration 017 drops the column, so a clean restore needs a new
  migration (018+) to re-add `visibility` with the CHECK constraint rather
  than resurrecting 008. Re-evaluate whether the owner toggle still belongs
  in `CalendarSettingsSheet` — the sheet has evolved (passphrase,
  duplicate) since the original plan assumed the toggle sat near rename.

---

## Calendar colour themes

- **Shipped:** 2026-04-21 — commit `f2a80a3` — plan `plans/calendar-colour-themes.md`
- **Cut:** 2026-04-22 — plan `plans/remove-calendar-themes.md`
- **Why cut:** Cosmetic knob that solved a problem nobody asked about. Added
  ~900 lines across the stack, and the day after it shipped we had to hide
  it behind a settings sheet because its UI weight was already disproportionate
  to its value. No user feedback pulled for it; the team pushed for it. Not
  worth the carrying cost.
- **What it did:** Owner picked a slug from a curated six-slug set (`peach`,
  `ocean`, `forest`, `plum`, `lemon`, `graphite`) that overrode `--accent`,
  `--accent-hover`, `--accent-subtle`, and `--accent-text` inside the
  calendar page wrapper via `[data-theme="<slug>"]` CSS blocks. Service
  picked a random slug on create; duplicate preserved source theme;
  `PATCH /calendars/{id}` accepted a `theme` field with allow-list
  validation. Owner UI was a swatch row inside the calendar settings sheet.
- **Downstream decisions changed:** None load-bearing. The settings sheet
  (`c5664fa`) was introduced partly to contain the theme picker's UI weight;
  it survives the cut and now holds visibility + passphrase + duplicate, so
  the sheet itself remains justified.
- **To restore:** Cherry-pick the feature commit (`f2a80a3`) and any
  follow-ups that touched theme code between then and the removal commit.
  Migration 014 (`ALTER TABLE
  calendars ADD COLUMN theme VARCHAR(32);`) is already on disk and was
  followed by the drop in 016; a clean restore needs a *new* migration
  (017+) to re-add the column rather than resurrecting 014. Palette values
  live in git history on `frontend/src/app/globals.css` and
  `frontend/src/lib/themes.ts` at the tagged SHA. Before restoring,
  re-evaluate whether the settings sheet layout (which has since evolved)
  still wants a theme row at the top — the original plan assumed theme sat
  directly below rename; that assumption no longer matches the current
  sheet.
