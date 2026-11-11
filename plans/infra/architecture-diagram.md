# Architecture Diagram

Status: done

## Goal

A one-screen, interactive architecture diagram (`docs/architecture.html`) for understanding how
FreeWhen fits together. The field guide (`docs/onboarding.html`) has the reference tables; this page
shows the components and walks seven everyday flows through them as numbered steps: create a
calendar, join and mark days, live updates, sign in, passphrase lock, import from Google, finalise
and email.

## Plan

1. Map the components from the code: `backend/internal/*`, `frontend/src/lib`, `infra/`.
2. Draw them in the hand-drawn rough.js style used for rdm's diagram: frontend, Go API layers and
   seams, outside services, deploy.
3. Write each flow's steps from the code (handlers, service, providers, notifications), not from
   memory.
4. Link it from the README's Documentation list and from the field guide's architecture section.
5. Check it renders on one 1080p screen.

## Log

- 2026-11-11 16:00 — Created plan; built `docs/architecture.html` with seven flows, checked against
  the code (import suggestions come from a single-use cookie set in the OAuth callback; the
  callback reads free/busy once and discards the token). Linked from README and onboarding.html.
- 2026-11-11 17:00 — Checked all seven flows render on one 1920×1080 screen; opened the PR. Docs only,
  so `make lint`/`make test` (scoped to `backend/` and `frontend/src`) are unaffected.
