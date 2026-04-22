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
- **To restore:** The pre-removal tree is tagged `themes-shipped-2026-04-21`
  (pointing at the most recent commit where themes are fully present).
  Cherry-pick the feature commit (`f2a80a3`) and any follow-ups that touched
  theme code between then and the removal. Migration 014 (`ALTER TABLE
  calendars ADD COLUMN theme VARCHAR(32);`) is already on disk and was
  followed by the drop in 016; a clean restore needs a *new* migration
  (017+) to re-add the column rather than resurrecting 014. Palette values
  live in git history on `frontend/src/app/globals.css` and
  `frontend/src/lib/themes.ts` at the tagged SHA. Before restoring,
  re-evaluate whether the settings sheet layout (which has since evolved)
  still wants a theme row at the top — the original plan assumed theme sat
  directly below rename; that assumption no longer matches the current
  sheet.
