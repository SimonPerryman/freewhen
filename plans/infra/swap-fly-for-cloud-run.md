# Swap Fly.io for Google Cloud Run

Status: done

## Goal

Switch the planned backend hosting target from Fly.io to Google Cloud Run
**before the first production deploy**. Cloud Run is a better fit for this
workload because it scales to zero, costs $0/month at our expected traffic
under the free tier, and removes Fly's $5/month minimum charge. The Go binary,
distroless Dockerfile, and `$PORT`/`/health` plumbing are already Cloud
Run-compatible — most of the work is documentation and CI.

## Context

- Nothing is deployed yet — there is no live Fly app, no DNS pointing at it,
  no users. Cutover risk is essentially zero.
- The Dockerfile is already a clean multi-stage build ending in
  `gcr.io/distroless/static`. Cloud Run reads it as-is.
- `internal/config/config.go` already reads `$PORT` (default `8080`), which is
  exactly what Cloud Run injects.
- Migrations already run on boot, which works fine with Cloud Run's
  scale-to-zero model — the first request after idle pays a one-time
  ~1s cold start that includes the migration check (no-op once applied).
- The existing `plans/tech-debt/cleanup-pass.md` documents the original Fly decision and
  is a historical record. **Do not edit it** — it should reflect what was
  decided at the time, not what we did later.

## Files to change

| File | Change |
|------|--------|
| `AGENTS.md` | Update the **Deployment** section: replace the Fly line with Cloud Run. |
| `docs/infra-notes.md` | Rewrite the Hosting table row, the "alternatives considered" section, the Deploy Flow line, and the First-Time Setup checklist. |
| `docs/stack.md` | Replace the Fly mention in the (already TBD-flagged) Hosting paragraph. |
| `.github/workflows/ci.yml` | Replace the `deploy-backend` job: swap `flyctl-actions` for `google-github-actions/auth` + `setup-gcloud`, swap `flyctl deploy` for `gcloud run deploy --source .`. |
| `backend/fly.toml` | Delete. |
| `backend/Dockerfile` | No change. (Sanity-check that the server binds `0.0.0.0:$PORT`, not `localhost:$PORT`. If it binds localhost, fix.) |
| `README.md` | No change — it doesn't reference Fly. |

## Plan

1. **Verify server binding.** Read `cmd/server/main.go` (or wherever
   `http.ListenAndServe` lives). Cloud Run requires the server to listen on
   `0.0.0.0:$PORT`. If it currently binds `localhost`, fix that as part of
   this change.
2. **Update `AGENTS.md`** — change the Backend line under Deployment to
   `Google Cloud Run (deploy via gcloud, CI deploys on push to main)`.
3. **Rewrite `docs/infra-notes.md`** with:
   - Hosting table updated.
   - "Alternatives considered" → keep Render/Railway, add Fly.io as the one we
     chose against (with a one-line "why not"). This documents the decision
     so future-us doesn't re-litigate.
   - Deploy Flow line updated to `gcloud run deploy --source .`.
   - First-Time Setup rewritten as a numbered checklist for GCP:
     1. Create GCP project, enable APIs (`run`, `cloudbuild`,
        `secretmanager`, `artifactregistry`).
     2. Store `DATABASE_URL` in Secret Manager
        (`gcloud secrets create freewhen-database-url`).
     3. Set up Workload Identity Federation for GitHub Actions (link to
        Google's guide; capture provider + service account values).
     4. Add `GCP_WIF_PROVIDER` and `GCP_DEPLOY_SA` to GitHub repo secrets.
     5. (Optional) Map a custom domain via
        `gcloud run domain-mappings create`.
     6. Set `NEXT_PUBLIC_API_URL` in Vercel to the Cloud Run URL (or custom
        domain).
4. **Update `docs/stack.md`** — replace `Fly.io/Railway` with `Cloud Run`.
   This file is already TBD-flagged so the edit is small.
5. **Replace the `deploy-backend` job in `.github/workflows/ci.yml`** with the
   Cloud Run version (sketch from the conversation; include
   `permissions: id-token: write` for WIF, `--min-instances 0`,
   `--max-instances 3`, `--memory 256Mi`, `--region europe-west2`,
   `--set-secrets DATABASE_URL=freewhen-database-url:latest`).
6. **Delete `backend/fly.toml`.**
7. **Lint + test** to make sure nothing references the removed paths.
8. **Commit and push.** CI will fail on the deploy step until the GCP setup
   in step 3 is done — that's expected. The plan documents the setup as a
   manual prerequisite to a real deploy, not as part of this PR.

## What this plan deliberately does **not** do

- The actual GCP project setup (manual, requires a Google account login —
  documented in `infra-notes.md` so anyone can do it).
- The first deploy itself.
- Any Fly account cleanup — there's nothing to clean up.
- Touching `plans/tech-debt/cleanup-pass.md`. It's history.

## Risks and mitigations

- **CI deploy step fails on push** until GCP setup is done. Acceptable — the
  test/lint/build jobs still run, so PRs are still gated. If we want CI green
  before setup, gate the deploy job on a manual workflow trigger temporarily.
- **Server binds localhost** — caught in step 1, easy fix.
- **Custom domain delay** — Cloud Run domain mappings take a few minutes for
  DNS. Until then, the `*.run.app` URL works fine; just point Vercel at it.

## Log

- 2026-04-19 08:38 — Created plan.
- 2026-04-19 08:46 — Verified server already binds `:$PORT` (all interfaces) — no fix needed.
- 2026-04-19 08:46 — Updated AGENTS.md, docs/infra-notes.md, docs/stack.md.
- 2026-04-19 08:46 — Replaced `deploy-backend` CI job with WIF auth + `gcloud run deploy --source .` (region europe-west2, min 0 / max 3, 256Mi, DATABASE_URL via Secret Manager).
- 2026-04-19 08:46 — Deleted `backend/fly.toml`.
- 2026-04-19 08:46 — `make test` and frontend oxlint pass; backend `go build ./...` clean. (`golangci-lint` not installed in this env — CI will run it.)
