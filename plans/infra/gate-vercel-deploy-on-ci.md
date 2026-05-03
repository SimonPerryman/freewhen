# Gate Vercel deploy on CI

Status: in-progress

## Goal

Currently the backend deploy to Cloud Run is gated on CI passing (see `plans/infra/gate-deploy-on-ci.md`), but the frontend still auto-deploys to Vercel via Vercel's git integration on every push to `main` — independent of CI. A red lint/test/build can ship to production. Make the frontend follow the same model: prod deploys only after CI passes.

## Plan

1. Add `frontend/vercel.json` with `git.deploymentEnabled.main = false` to disable Vercel's auto-deploy on `main`. Leave PR previews enabled (Vercel's git integration handles previews; we don't need CI-gated previews).
2. Add a `deploy-frontend` job in `.github/workflows/ci.yml` mirroring `deploy-backend`:
   - `needs: [ci]`, `if: github.ref == 'refs/heads/main'`
   - Use `vercel` CLI: `pnpm dlx vercel pull`, `vercel build --prod`, `vercel deploy --prebuilt --prod`
   - Secrets: `VERCEL_TOKEN`, `VERCEL_ORG_ID`, `VERCEL_PROJECT_ID`
3. Document new secrets in `docs/infra-notes.md` / `docs/deploy-checklist.md`.

## Log

- 2026-05-03 — Created plan
- 2026-05-03 — Added `frontend/vercel.json` disabling Vercel auto-deploy on `main`; updated `docs/infra-notes.md` and `docs/deploy-checklist.md` with new secrets and deploy flow.
- 2026-05-03 — Added `deploy-frontend` job to `.github/workflows/ci.yml`. Gated on `changes.outputs.frontend` (path filter) and sequenced after `deploy-backend` so the API ships before the UI; tolerates `deploy-backend` being skipped on frontend-only changes.
