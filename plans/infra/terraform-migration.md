# Migrate GCP + GitHub config to Terraform

Status: pending

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

7. **Optional: run `terraform plan` in CI**
   - GitHub Actions job that runs `terraform plan` on every PR touching
     `infra/`, posts the plan output as a comment
   - Same WIF auth as the deploy job, but with a *read-only* SA so a
     malicious PR can't `apply` from CI
   - Skip for now — solo project, not worth the second SA

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
