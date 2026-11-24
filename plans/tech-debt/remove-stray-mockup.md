# Remove stray mockups

Status: done

## Goal

Drop the rough HTML mock-ups, demos and POCs from the now-public repo: the top-level `mockups/calendar-emoji-header.html` and every tracked `.html` mock under `plans/`. The owner keeps local copies in the gitignored `.ai-cache/mockups/` (plan mocks under `.ai-cache/mockups/plans/<same path>`).

## Plan

1. Copy each file to the main checkout's `.ai-cache/mockups/`, keeping plan mocks under a `plans/` subfolder with their original relative path.
2. `git rm` them; the top-level `mockups/` folder and the `plans/ui/mocks/` and `plans/v2/t3/mocks/` folders go with them.
3. Search the repo for references. Turn links and path references in plan bodies into plain text marked "(mock, not kept in the repo)". Leave Log entries as they are, since they record history.

Removed:

- `mockups/calendar-emoji-header.html`
- `plans/ui/design-mockups.html`
- `plans/ui/mocks/create-cta-variants.html`
- `plans/ui/mocks/theme-before-after.html`
- `plans/ui/mocks/themes-compare.html`
- `plans/ui/prefer-indicator-poc.html`
- `plans/ui/share-button-redesign.demo.html`
- `plans/v2/t3/mocks/calendar-integrations-cell-states.html`
- `plans/v2/t3/mocks/calendar-integrations-flow.html`

## Log

- 2026-11-24 09:38 — Created plan
- 2026-11-24 09:38 — Copied the mockup to `.ai-cache/mockups/` in the main checkout and removed it (and the empty `mockups/` folder) from git
- 2026-11-24 09:38 — Searched for references; none found, so no other files needed changing
- 2026-11-24 09:40 — Widened scope to every tracked HTML mock under `plans/` (8 files). Copied them to `.ai-cache/mockups/plans/`, removed them from git, and replaced the links and path references in the bodies of calendar-integrations, theme-refresh, design-mockups-implementation, create-cta-refresh, prefer-indicator-rework and share-button-redesign with plain text
