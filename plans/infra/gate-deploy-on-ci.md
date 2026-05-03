# Gate Cloud Run deploys on CI

Status: pending

## Goal

Today, Cloud Build auto-deploys the backend on every push to `main`, with no
gating from `make lint` / `make test` / `make test-integration`. CI only runs
on PRs, so a direct push (or a missed status check) can ship broken code to
production.

Move the deploy step into GitHub Actions so it runs **after** CI passes, using
the `deploy-backend` job that already exists in `.github/workflows/ci.yml`
(currently dormant — there's no push trigger and no WIF secrets configured).

## Plan

1. **Set up Workload Identity Federation** (one-time GCP setup) ✅ done
   - Follow [Google's WIF guide](https://github.com/google-github-actions/auth#setting-up-workload-identity-federation)
   - Create a workload identity pool + provider tied to GitHub
     (`github.com/SimonPerryman/freewhen` repo, `main` branch attribute condition)
   - Create a deploy service account (e.g. `freewhen-deploy@freewhen-493819.iam.gserviceaccount.com`)
   - Grant the SA these roles on the project:
     - `roles/run.admin` — deploy Cloud Run revisions
     - `roles/iam.serviceAccountUser` — act as the Cloud Run runtime SA
     - `roles/secretmanager.secretAccessor` — reference secrets in `--set-secrets`
     - `roles/artifactregistry.writer` — push the built image
   - Bind the WIF provider to the SA so GitHub Actions on `main` can impersonate it

2. **Add GitHub repo secrets** ✅ done
   - `GCP_WIF_PROVIDER` — full provider resource name
     (`projects/<project-number>/locations/global/workloadIdentityPools/<pool>/providers/<provider>`)
   - `GCP_DEPLOY_SA` — deploy service account email

3. **Re-enable the push trigger in `.github/workflows/ci.yml`** ✅ done
   - Uncomment / add `push: branches: [main]` alongside the existing
     `pull_request` trigger
   - The existing `deploy-backend` job is already gated by `needs: [ci]` and
     `if: github.ref == 'refs/heads/main'`, so it will only run after CI passes
     and only on `main`

4. **Sync the `gcloud run deploy` invocation with the live service config** ✅ done
   - Move tunables to GitHub Actions variables (repo Settings → Secrets and
     variables → Actions → Variables) so config lives in one place and isn't
     hardcoded across the workflow:
     - `GCP_REGION` = `europe-west1` (currently `europe-west2` in the workflow —
       must change or auto-deploy creates a second service in London)
     - `GCP_SERVICE_NAME` = `freewhen-api`
     - `GCP_MAX_INSTANCES` = whatever you set in the GUI (e.g. `5`)
     - `GCP_MEMORY` = whatever you set in the GUI (e.g. `512Mi`)
   - Read them in the workflow via `${{ vars.GCP_REGION }}` etc.
   - Audit every flag the existing service has set in the GUI (env vars,
     `--set-secrets` references, `--allow-unauthenticated`, CPU boost,
     min-instances, ingress) — anything missing from the `gcloud run deploy`
     command gets **reset to defaults on each deploy**. Either:
     - List every env var + secret in the workflow's `--set-env-vars` /
       `--set-secrets` flags (config-as-code, recommended), or
     - Use `--update-env-vars` / `--update-secrets` (only changes named keys,
       leaves others alone — easier migration but config drifts from repo)
   - Recommended: full `--set-*` flags so the workflow is the single source of
     truth. Capture current GUI state with `gcloud run services describe
     freewhen-api --region europe-west1 --format yaml` and translate every
     non-default field into flags.

5. **Disconnect the Cloud Build → Cloud Run trigger** ✅ done
   - Trigger already deleted in Cloud Build. Push to `main` no longer
     auto-deploys via Google's side; until step 3 lands, `main` ships
     nothing automatically.

6. **Verify**
   - Push a trivial change to a branch → open PR → CI runs, no deploy
   - Merge PR → CI runs again on `main`, then `deploy-backend` runs, new
     revision appears in Cloud Run
   - Force a CI failure (lint error) on a follow-up PR → confirm `deploy-backend`
     doesn't run

7. **Optional follow-up**
   - Add branch protection on `main` requiring the `ci` check to pass before
     merge — defence in depth so CI failures can't reach `main` even via direct
     push by an admin

## Notes

- The workflow's deploy step uses `--source .` from `working-directory: backend`,
  meaning Cloud Build is still invoked under the hood (just from CI rather than
  on push). That's fine — the gating is what matters. If we want to remove
  Cloud Build entirely later, swap to `docker build` + `gcloud run deploy --image`.
- Region history: originally targeted `europe-west3` (Frankfurt) to co-locate
  with Neon `eu-central-1`, but Cloud Run **domain mappings aren't supported
  there**, and neither is `europe-west2` (London). Settled on `europe-west1`
  (Belgium) — supported, and still low-latency to Neon. The old `europe-west3`
  service has been deleted; the live one is in `europe-west1`. Step 4 moves
  region into `vars.GCP_REGION` so this isn't hardcoded — set the variable to
  `europe-west1` before merging the workflow change.

## Log

- 2026-05-02 — Created plan
- 2026-05-03 — Split region/service-name/memory/max-instances into a dedicated
  step using GitHub Actions variables; expanded flag-sync guidance so the
  first auto-deploy doesn't silently reset GUI-configured env vars / secrets
- 2026-05-03 — Moved live Cloud Run service from `europe-west3` to
  `europe-west1` (Frankfurt → Belgium) because domain mappings aren't
  supported in west3 or west2. Old west3 service deleted; redeploy was done
  manually via Cloud Shell with `gcloud run deploy --source .` from a fresh
  SSH clone. Plan now targets `europe-west1` as the canonical region.
- 2026-05-03 — Deleted the Cloud Build push trigger. `main` no longer
  auto-deploys; steps 1–4 now need to land before the next intended deploy.
- 2026-05-03 — Completed step 1 (WIF pool `github`, provider `freewhen`
  with `assertion.repository == 'SimonPerryman/freewhen'` condition,
  deploy SA `freewhen-deploy` with the four required project roles, and
  `roles/iam.workloadIdentityUser` bound to the repo-scoped principalSet)
  and step 2 (`GCP_WIF_PROVIDER` and `GCP_DEPLOY_SA` GitHub repo secrets).
- 2026-05-03 — Completed steps 3 and 4: added `push: branches: [main]`
  trigger to `ci.yml`, rewrote the deploy step to read from
  `vars.GCP_REGION` / `vars.GCP_SERVICE_NAME` / `vars.GCP_MEMORY` /
  `vars.GCP_MAX_INSTANCES`, and added explicit `--set-env-vars` /
  `--set-secrets` flags covering all 8 env vars and 5 secrets from the
  live service. Workflow is now the single source of truth for Cloud Run
  config; GUI edits will be overwritten on next deploy.
