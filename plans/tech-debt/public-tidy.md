# Public-repo tidy-up

Status: done

## Goal

Tidy the plans and docs before the repo goes public. Nothing here blocks the release, but some things would confuse or mislead outside readers: example domains we don't own, plans that describe weaknesses which have since been fixed, stale statuses, dead links and small cosmetic errors.

## Plan

1. Replace `freewhen.app` / `freewhen.com` with the real production domains (`freewhen.simonperryman.co.uk`, `api.freewhen.simonperryman.co.uk`) where the text means production, otherwise `example.com`-style placeholders.
2. Add a "Superseded" Log line to plans whose described weakness was later fixed: edit-participant-name, submit-to-view-privacy, participant-user-linking, magic-link, live-updates, calendar-integrations. Check each claim against the code first.
3. Mark `plans/bugfix/backend-security-hardening.md` done.
4. Add a pre-launch note above the calendar-integrations compliance checklist and fill in the region from `infra/`.
5. Cosmetic: wrong-year log dates, literal `HH:MM` placeholders, a duplicated paragraph, stray YAML front matter.
6. Change dead links to temporary mock files into plain text.

Out of scope: `plans/infra/architecture-diagram.md` ("rdm"), `plans/infra/public-readiness.md`, LICENSE/README/SECURITY, `infra/*.tf`, `.github/`.

## Log

- 2026-11-23 20:50 — Created plan
- 2026-11-23 20:51 — Replaced freewhen.app / freewhen.com: production domains in docs/infra-notes.md (also fixed its region to europe-west1), docs/user-journeys.md and calendar-export-richer-event.md; example.com placeholders in handler.go, design-mockups.html and auth.md
- 2026-11-23 20:52 — Checked each fix against the code, then added Superseded log lines to six plans; marked backend-security-hardening done (shipped in #73); added the pre-launch note and europe-west1 region to calendar-integrations; fixed the wrong-year dates, HH:MM placeholders (times dropped, since each plan landed in a single commit), the duplicated paragraph, the YAML front matter and the dead mock links
- 2026-11-23 20:55 — make lint: golangci gofmt and oxfmt flag CRLF-only noise on untouched files (the committed versions are clean); oxlint, typecheck and lint-pii pass. make test: all Go packages ok, vitest 474/474. Status → done
