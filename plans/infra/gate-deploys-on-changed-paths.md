---
# Gate deploys on changed paths

Status: done

## Goal

Skip Cloud Run / Vercel deploys when a push to `main` doesn't touch the
relevant stack. Today every merge to `main` redeploys both backend and
frontend regardless of what changed — wasted CI minutes, unnecessary
churn on Cloud Run revisions, and a slower path to green.

## Plan

- Add a `changes` job to `.github/workflows/ci.yml` using
  `dorny/paths-filter@v3` that emits `backend` and `frontend` boolean
  outputs based on the diff for the triggering event.
- Filters:
  - `backend`: `backend/**`, `Makefile`, `.github/workflows/ci.yml`
  - `frontend`: `frontend/**`, `Makefile`, `.github/workflows/ci.yml`
  - `Makefile` and the workflow file are intentionally in both — shared
    tooling changes should re-deploy both stacks rather than risk a
    silent skip.
- Gate `deploy-backend` / `deploy-frontend` with
  `needs.changes.outputs.<stack> == 'true'` in addition to the existing
  `github.ref == 'refs/heads/main'` check.
- Lint/test/build jobs stay unconditional — we still want to validate
  every change, only the deploy steps are skipped.

## Log

- 2026-05-03 — Created plan and wired up `dorny/paths-filter` in CI.
- 2026-05-09 — Marked done. Path-filter gating has been in CI and skipping no-op deploys as expected.
