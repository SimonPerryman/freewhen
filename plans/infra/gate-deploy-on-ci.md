# Gate Cloud Run deploys on CI

Status: done

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
   - Create a workload identity pool (`github`) + OIDC provider (`freewhen`)
     - Issuer: `https://token.actions.githubusercontent.com`
     - Attribute mapping: `google.subject=assertion.sub`,
       `attribute.actor=assertion.actor`,
       `attribute.repository=assertion.repository`,
       `attribute.repository_owner=assertion.repository_owner`
     - **Attribute condition: `assertion.repository == 'SimonPerryman/freewhen'`**
       — pins to this exact repo. Watch out: `assertion.repository_owner`
       is just the owner string (`SimonPerryman`), *not* `owner/repo`. Initial
       attempt had `assertion.repository_owner == 'SimonPerryman/freewhen'`
       which can never match — every auth was rejected with "credential
       rejected by attribute condition".
   - Create deploy SA `freewhen-deploy@freewhen-493819.iam.gserviceaccount.com`
   - **Project-level role bindings on the deploy SA** — the four below from
     the original plan turned out to be **insufficient**; the actual set
     needed for `gcloud run deploy --source .` is six:
     - `roles/run.admin` — deploy Cloud Run revisions
     - `roles/iam.serviceAccountUser` — listed but doesn't cover actAs on a
       specific runtime SA (see binding below)
     - `roles/secretmanager.secretAccessor` — reference secrets
     - `roles/artifactregistry.writer` — push the built image
     - `roles/cloudbuild.builds.editor` — `--source` invokes Cloud Build
       under the hood; without this gcloud fails with generic
       `PERMISSION_DENIED` ~5s in
     - `roles/storage.admin` — gcloud uploads source zip to
       `gs://run-sources-<PROJECT>-<REGION>` before Cloud Build picks it up.
       Bucket-scoped `roles/storage.objectAdmin` is **not enough** — gcloud
       calls `storage.buckets.list` (project-level) to discover the bucket,
       so it has to be granted at project level. Initial attempt with
       bucket-scoped `storage.admin` failed on `storage.buckets.list`.
   - **SA-scoped binding on the runtime SA**: project-level
     `iam.serviceAccountUser` does *not* cover impersonating a specific
     SA. Bind `roles/iam.serviceAccountUser` on
     `350910532607-compute@developer.gserviceaccount.com` (the default
     compute SA Cloud Run uses to run the container) with
     `freewhen-deploy@...` as the principal. Without this the deploy fails
     with generic `PERMISSION_DENIED` immediately after auth succeeds.
   - **WIF principal binding on the deploy SA**: bind
     `roles/iam.workloadIdentityUser` on `freewhen-deploy@...` with
     `principalSet://iam.googleapis.com/projects/<PROJECT_NUMBER>/locations/global/workloadIdentityPools/github/attribute.repository/SimonPerryman/freewhen`
     as the principal. The `attribute.repository/...` suffix must match
     whichever attribute the provider's condition keys off.

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

6. **Verify** ✅ done
   - First end-to-end auto-deploy succeeded on 2026-05-03 after iterating
     through five permission gaps (see Log). New revision appeared in
     Cloud Run; deploy job is gated on `ci` passing as designed.

7. **Optional follow-up** ✅ done
   - Classic branch protection rule on `main` enabled with: require PR
     before merging (0 approvals, solo project), require `ci` status check
     to pass, require branches up-to-date, require conversation resolution.
     Force pushes and deletions disabled. Self-bypass left enabled for
     emergencies.

## Notes

- The workflow's deploy step uses `--source .` from `working-directory: backend`,
  meaning Cloud Build is still invoked under the hood (just from CI rather than
  on push). That's fine — the gating is what matters. If we want to remove
  Cloud Build entirely later, swap to `docker build` + `gcloud run deploy --image`
  — that also removes the need for the `cloudbuild.builds.editor` and
  `storage.admin` roles from the deploy SA, leaving a much smaller IAM surface.

### IAM surface — full picture

For posterity (and because we discovered the original four-role list was
incomplete), the deploy SA `freewhen-deploy@freewhen-493819.iam.gserviceaccount.com`
ended up with:

**Project-level** (`IAM & Admin → IAM`):
- `roles/run.admin`
- `roles/iam.serviceAccountUser` *(see SA-scoped binding below — this
  project-level role alone doesn't cover impersonation of a specific SA;
  kept for any future SA without an explicit binding)*
- `roles/secretmanager.secretAccessor`
- `roles/artifactregistry.writer`
- `roles/cloudbuild.builds.editor` *(needed for `--source` builds)*
- `roles/storage.admin` *(needed for source upload + `buckets.list`)*

**SA-scoped** (`Service Accounts → 350910532607-compute@... → Manage access`):
- `roles/iam.serviceAccountUser` granted to `freewhen-deploy@...` so it can
  actAs the runtime SA Cloud Run uses

**WIF binding** (`Service Accounts → freewhen-deploy@... → Manage access`):
- `roles/iam.workloadIdentityUser` granted to
  `principalSet://iam.googleapis.com/projects/350910532607/locations/global/workloadIdentityPools/github/attribute.repository/SimonPerryman/freewhen`

A migration to Terraform (`plans/infra/terraform-migration.md`) is planned
to capture all of this as code, since the manual reconstruction trail is
exactly the failure mode that motivated IaC in the first place.
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
- 2026-05-03 — First post-merge deploy failed five times in a row, each
  time on a different missing IAM permission. Sequence (each fix was a
  separate UI grant + rerun):
  1. **Attribute condition mismatch** — provider had
     `assertion.repository_owner == 'SimonPerryman/freewhen'` (wrong:
     `repository_owner` claim is just `SimonPerryman`, never has `/repo`).
     Fixed by `gcloud iam workload-identity-pools providers update-oidc`
     with `assertion.repository == 'SimonPerryman/freewhen'`.
  2. **Generic `PERMISSION_DENIED` ~5s in** — project-level
     `iam.serviceAccountUser` doesn't cover actAs on a specific SA. Fixed
     by binding `roles/iam.serviceAccountUser` on the runtime SA
     (`350910532607-compute@developer.gserviceaccount.com`) with
     `freewhen-deploy@...` as principal.
  3. **`PERMISSION_DENIED` again** — `--source` builds invoke Cloud Build,
     which the SA wasn't authorised for. Fixed with project-level
     `roles/cloudbuild.builds.editor`.
  4. **`storage.buckets.get` denied** — granted bucket-scoped
     `roles/storage.objectAdmin`, but gcloud also needs `buckets.get`
     before object operations. Granted bucket-scoped `roles/storage.admin`.
  5. **`storage.buckets.list` denied** — `buckets.list` is a project-level
     permission, can't be granted on a single bucket. Replaced bucket-scoped
     `storage.admin` with project-level `roles/storage.admin`.
  After (5), deploy completed end-to-end. Plan's step 1 + Notes section
  updated to reflect the actual IAM surface (six project-level roles +
  one SA-scoped binding, not the original four). Whole episode logged
  as the trigger for `plans/infra/terraform-migration.md`.
- 2026-05-03 — Enabled classic branch protection on `main` (require PR,
  require `ci` status check, require up-to-date branches, require
  conversation resolution; force pushes + deletions disabled). Step 7
  done. Plan status flipped to `done`.
- 2026-05-04 18:23 — Discovered a follow-up: `gcloud builds submit` (used
  by `deploy-backend` to push the image) tries to stream Cloud Build
  logs and exits non-zero if it can't read them — even when the build
  itself succeeds. The check requires `resourcemanager.projects.get`
  (project Viewer/Owner), which the scoped deploy SA doesn't have.
  Three runs in a row failed identically (#13, #14, #14 rerun) with the
  build succeeding but the job exiting on the streaming error, which
  also skipped `deploy-frontend` due to its `needs: deploy-backend`
  gate. Tried `roles/logging.viewer` first (#15) — not enough; gcloud
  checks for project-level Viewer specifically. Settled on passing
  `--suppress-logs` to `gcloud builds submit` (#16) and reverted the
  `logging.viewer` binding. Added a short comment in the workflow next
  to the flag explaining why.
