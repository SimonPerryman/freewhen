# Migrate GCP + GitHub config to Terraform

Status: in-progress

## Goal

The Cloud Run deploy gating work (`plans/infra/gate-deploy-on-ci.md`) made
visible how much of FreeWhen's infra lives only in GCP/GitHub UIs and tribal
knowledge: WIF pool + provider + attribute condition, two service accounts
and seven IAM bindings between them, a Cloud Run service with eight env vars
and five secret references, four GitHub Actions variables, two GitHub Actions
secrets. Three deploy failures in a row were each a missing IAM binding only
visible by reading error logs.

Terraform replaces all of that with diffable code, deterministic recreation,
and `terraform plan` as a pre-flight check for IAM/config changes. Goal is
**document reality first, change nothing**: every existing resource gets
imported, `terraform plan` shows zero diff, then future infra changes go
through TF instead of the console.

Out of scope: migrating Vercel (frontend deploy is fine where it is, and
their TF provider is awkward) and Neon (managed via their own dashboard,
not worth the import overhead for a single DB).

## Plan

1. **Bootstrap state backend**
   - Create `gs://freewhen-tfstate` (uniform bucket-level access, versioning
     on, lifecycle rule to delete non-current versions after 90 days)
   - Bucket lives in `freewhen-493819` / `europe-west1` to keep latency low
     and avoid a second region's egress costs
   - Grant `roles/storage.objectAdmin` on the bucket to the human(s)
     running TF locally; nothing else needs access
   - Bootstrap is a chicken-and-egg: create the bucket via `gcloud` once,
     then the rest is in code

2. **Scaffold `infra/` folder**
   - `infra/main.tf` — provider blocks (`google`, `github`), backend config
     pointing at the GCS bucket, `terraform` block pinning provider versions
   - `infra/variables.tf` — `project_id`, `project_number`, `region`,
     `github_owner`, `github_repo` (no defaults; values come from tfvars)
   - `infra/terraform.tfvars` — actual values; **gitignored** if any are
     sensitive, but for this project everything's already public-knowable
     so commit it
   - `infra/iam.tf` — WIF pool, provider, deploy SA, all seven role bindings
     (four project-level + actAs on runtime SA + Cloud Build editor +
     Storage Object Admin once the deploy gate lands)
   - `infra/cloud_run.tf` — `google_cloud_run_v2_service` with full env vars
     and secret refs, ingress, scaling, CPU boost, concurrency, timeout
   - `infra/secrets.tf` — `data "google_secret_manager_secret"` blocks for
     the five existing secrets (data, not resource — TF reads them but
     doesn't manage their values; secrets stay edited via `gcloud secrets
     versions add` to avoid round-tripping plaintext through TF state)
   - `infra/artifact_registry.tf` — the `cloud-run-source-deploy` repo
   - `infra/github.tf` — four `github_actions_variable` resources +
     two `github_actions_secret` resources

3. **Import existing resources**
   - Write `infra/imports.sh` listing every `terraform import` command,
     ~15 lines. Idempotent — re-running is safe because import is a no-op
     once state has the resource
   - Order matters for resources that reference others (WIF pool before
     provider, SA before bindings)
   - Run `terraform plan` after each import block; expect zero diff at the
     end. Any diff = TF code doesn't match reality, fix the HCL not the
     console
   - The five Secret Manager *secrets* import as data sources only;
     versions/values stay out of state entirely

4. **Verify zero-diff state**
   - `terraform plan` must show "No changes. Your infrastructure matches
     the configuration." before this is mergeable
   - Run on the actual deploy SA's local machine (not via CI yet — see
     step 7 for that)

5. **Move config drift checks into PR review**
   - Add a `plans/` requirement note: any infra change touches `infra/`,
     not the console. Console edits will be reverted by the next `apply`
   - Optional: `make tf-plan` target in the root `Makefile` that runs
     `terraform -chdir=infra plan -detailed-exitcode` so drift surfaces
     during local development

6. **Remove the now-duplicated config from `.github/workflows/ci.yml`**
   - With Cloud Run service managed by TF, the workflow's `--set-env-vars`
     / `--set-secrets` flags are redundant — they re-assert config TF
     already owns
   - Two options:
     - **Build-and-deploy split**: workflow builds + pushes to Artifact
       Registry, then runs `terraform apply -target=...` to roll the new
       image. Cleanest, but couples deploys to TF state availability.
     - **Image-only deploy**: workflow builds + pushes, then runs
       `gcloud run services update freewhen-api --image=...`. Cloud Run
       service stays managed by TF for everything *except* the image tag,
       which TF treats as `lifecycle.ignore_changes = [template[0].containers[0].image]`.
       Simpler, decouples deploys from TF
   - Recommend image-only — fewer moving parts on the hot path. Deploys
     stay fast (no `terraform plan` on every push). Config drift caught
     by step 5's `make tf-plan`

7. **Run `terraform plan` in CI on infra changes**
   - Extend the existing `changes` job (`dorny/paths-filter`) with an
     `infra` filter: `infra/**` + `.github/workflows/ci.yml`. Same
     pattern as the existing `backend` / `frontend` filters added in
     `plans/infra/gate-deploys-on-changed-paths.md`
   - Add a `tf-plan` job: `needs: [changes]`,
     `if: needs.changes.outputs.infra == 'true'`. Runs on PRs *and* main
     (drift detection is most useful pre-merge — no `github.ref` gate)
   - Auth via WIF, but with a **read-only** SA (`roles/viewer` on the
     project + `roles/storage.objectAdmin` on the state bucket) so a
     malicious PR can't elevate to `apply` from CI. Bucket-level admin
     is required because the GCS backend writes a lock object even on
     `init`/`plan`; the read-only safety property is preserved by the
     project-level `roles/viewer`, which prevents the SA from mutating
     any actual GCP resources regardless of state-file access. Add this
     SA + WIF binding to `infra/iam.tf` alongside the deploy SA
   - New GitHub repo secret: `GCP_TF_PLAN_SA` for the read-only SA email
   - Job runs `terraform -chdir=infra plan -no-color -detailed-exitcode`
     and posts plan output as a PR comment via `actions/github-script`
     (or `hashicorp/setup-terraform`'s built-in `github_token` integration)

## Risks / things to watch

- **Importing the Cloud Run service is the trickiest part.** The TF resource
  must match the live service field-for-field, including fields with default
  values that don't show up in `describe` output. First `plan` will likely
  show diffs on `liveness_probe`, `startup_probe`, or `revision_template`
  metadata; iterate the HCL until plan is clean.
- **Don't put secret *values* in TF state.** The plan uses `data` sources
  for Secret Manager secrets, not `resource` blocks with `secret_data`.
  Values stay in Secret Manager, edited out-of-band; TF only references
  them by name in the Cloud Run service definition.
- **GitHub provider needs a PAT or App.** Repo-scoped fine-grained PAT with
  Actions/Secrets/Variables read+write. Store outside of state (env var
  `GITHUB_TOKEN` at apply time).
- **Don't import the WIF provider's attribute condition wrong.** It's a
  string field; if the imported value differs by even a quote or whitespace
  from the HCL, plan will want to recreate the provider — which would
  invalidate every CI run mid-flight. Diff carefully before applying.
- **State file is the source of truth.** No more clicking around the GCP
  console for changes. If you must hotfix in the console (e.g. emergency
  rollback), `terraform refresh` + commit the matching HCL change before
  the next normal change, otherwise the next apply silently reverts.

## Sequencing

Land **after** `plans/infra/gate-deploy-on-ci.md` is fully done (deploy
working end-to-end). Doing this in parallel with debugging IAM means TF
imports a half-broken state. The whole point is "document reality" — make
reality stable first.

## Log

- 2026-05-03 — Created plan after debugging the third missing IAM binding
  in the gate-deploy-on-ci work made the case for IaC obvious. Sequenced
  to land *after* deploy gating is verified end-to-end.
- 2026-05-03 — Scaffolded `infra/` (HCL only, no execution): main, variables,
  tfvars, iam, secrets, cloud_run, artifact_registry, github, imports.sh.
  Added `tf-plan` CI job behind a new `infra` paths-filter, plus `make tf-plan`.
  Steps 1 (bucket creation) and 3–4 (imports + zero-diff verification) still
  need to happen on a real machine before this is operational.
- 2026-05-04 — Bucket created. Created `freewhen-tf-plan` SA with
  `roles/viewer` on project, WIF binding, repo secret `GCP_TF_PLAN_SA`.
  Filled in `project_number` + `github_owner` in tfvars.
- 2026-05-04 — First CI run failed at `terraform init` because GCS backend
  writes a lock file even on init/plan; `roles/storage.objectViewer` on the
  bucket is insufficient. Upgraded to `roles/storage.objectAdmin` (in console
  + in `iam.tf`/`imports.sh`). Read-only safety property is preserved by
  project-level `roles/viewer`, not by bucket IAM.
- 2026-05-04 — Second CI run failed because `var.gcp_wif_provider` had no
  default and TF prompted interactively. Added `TF_VAR_gcp_wif_provider` +
  `GITHUB_TOKEN` env vars to the tf-plan job (sourced from the existing
  `GCP_WIF_PROVIDER` repo secret + the default workflow token).
- 2026-05-04 — `tf-plan` job green on PR #4. Plan reads the five live
  Secret Manager secrets via `data` sources (proves WIF auth works) and
  proposes `+ create` for ~20 resources because nothing's imported yet.
  PR is mergeable; steps 3 (imports), 4 (zero-diff verification), and 6
  (strip duplicated env config from deploy workflow) remain.
- 2026-05-04 — PR #4 merged. Started step 3 on a fresh branch
  (`infra/tf-imports`). Reality vs HCL findings:
  - WIF pool ID is `github` (not `github-pool`); provider is `freewhen`
    (not `github-provider`).
  - There is no `freewhen-runtime` SA. Cloud Run runs as the default
    Compute Engine SA (`<project_number>-compute@developer.gserviceaccount.com`).
    Switched HCL from a managed `google_service_account.runtime` to a
    `local.runtime_sa_email` (avoids needing the Compute Engine API
    enabled just for a data-source lookup).
  - Deploy SA roles live: `run.admin`, `iam.serviceAccountUser`,
    `secretmanager.secretAccessor`, `artifactregistry.writer`,
    `cloudbuild.editor` (not `.builds.editor`), `storage.admin`,
    `storage.objectAdmin`. Both storage roles bound separately —
    `storage.admin` includes `objectAdmin` but live IAM has both.
  - Cloud Run `maxScale` is 3, memory is 256Mi, `cpu_idle` is true
    (default; the deploy command doesn't pass `--no-cpu-throttling`).
  - WIF provider `attribute_condition` uses single-quotes; mapping has
    `attribute.repository_owner`, not `attribute.ref`.
  - Two artifact registries exist (`europe-west1` active +
    `europe-west3` orphan). HCL only manages the west1 one.
  - github_actions_secret values can't be read back via API, so plan
    always shows "would write value." Worked around via
    `lifecycle.ignore_changes = [value]`.
  - Cloud Run service has a `build_config` block populated by
    `gcloud run deploy --source .`, plus auto-generated container
    `name` and a control-plane-managed top-level `scaling` block.
    All added to `ignore_changes` in `cloud_run.tf`.
  - Switched github provider auth from `GITHUB_TOKEN` env var to
    `var.github_token` so it doesn't collide with `gh` CLI's env var.
    Local PAT lives in gitignored `infra/secrets.auto.tfvars`; CI
    keeps using the workflow token via `TF_VAR_github_token`.
  - Switched local Terraform from the homebrew/core MPL build (1.5.7)
    to `hashicorp/tap/terraform` (1.15.1) to satisfy `required_version`.
- 2026-05-04 — `terraform plan` shows zero diff. All 31 resources in
  state, HCL matches reality field-for-field. Step 3 + step 4 done.

## Status of plan steps

- ✅ Step 1: Bootstrap state backend — `gs://freewhen-tfstate` created
- ✅ Step 2: Scaffold `infra/` folder — all `.tf` files written
- ✅ Step 3: Import existing resources — all 31 imported, HCL adjusted
- ✅ Step 4: Verify zero-diff state — `terraform plan` reports
  "No changes. Your infrastructure matches the configuration."
- 🔄 Step 5: Move config drift checks into PR review — `make tf-plan`
  added; `plans/` requirement note still TBD
- ⬜ Step 6: Remove duplicated env/secret config from `ci.yml` deploy job
- ✅ Step 7: Run `terraform plan` in CI — done in PR #4
- ⬜ Step 8: Run `terraform apply` in CI on main — now unblocked
