# GitHub Deployment Status for Frontend Deploys

Status: in-progress

## Goal

The repo homepage's Deployments sidebar showed a red X for `Production` even though CI deploys were passing. The only records came from the Vercel Git integration (last one: a failure on 2026-05-04); CI deploys via `vercel deploy --prebuilt`, which never reports to GitHub. Make CI report its own Production deployments so the sidebar reflects reality.

## Plan

1. Add `environment: Production` (with the site URL) to the `deploy-frontend` job in `.github/workflows/deploy.yml`, so Actions records each run as a deployment.
2. Dismiss the stale failed Vercel-bot record (id 4574225178) by marking it inactive — pending owner OK.
3. Optional: disable GitHub deployment events in the Vercel project's Git settings so only CI writes records.

## Log

- 2026-11-24 — Created plan; diagnosed stale failure record
- 2026-11-24 — Added `environment` to `deploy-frontend` job
