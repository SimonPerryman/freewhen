# Staging environment (and N-environments via IaC)

Status: pending

## Goal

Stand up a `staging` environment that mirrors prod, and refactor `infra/` so that adding any further environment (`preview`, `demo`, `<dev-name>`) is a 5-minute copy of a tfvars file plus a single console click for the Google OAuth redirect URI.

The driver: migrations run on boot in prod. We currently have no prod-shaped place to test them before they ship. Staging closes that gap and gives us somewhere to dogfood breaking changes.

## Constraints

- Almost everything is IaC-able. Verified providers exist for: `google`, `github`, `vercel`, `neon`, `posthog`, `resend`. The only manual step per new env is adding redirect URIs to the existing Google OAuth client (no public API).
- Resend domain verification is the one "real-world" friction point — staging can sidestep it by setting `AUTH_EMAIL_DISABLED=true` (magic links logged, not sent), or by using `onboarding@resend.dev` for verified internal addresses.
- Neon branches are cheap and instant; one branch per env off `main` is the right shape.
- Per-env Terraform state must be isolated (separate GCS prefix) so a broken staging plan can't roll prod.

## Plan

### Phase 1 — Refactor prod into a reusable module (no behaviour change)

1. Move the contents of `infra/*.tf` into `infra/modules/freewhen/`. Parameterise everything currently hardcoded:
   - Existing vars stay: `project_id`, `project_number`, `region`, `service_name`, `github_owner`, `github_repo`, `gcp_wif_provider`.
   - New vars: `env_name`, `cors_origin`, `cookie_domain`, `frontend_url`, `magic_link_from`, `google_redirect_url`, `google_calendar_redirect_url`, `min_instances`, `max_instances`, `cpu_limit`, `memory_limit`, `auth_email_disabled`.
   - `secret_id` prefix becomes `freewhen-${env_name}-...` so secrets don't collide across envs.
2. Create `infra/envs/prod/main.tf` calling the module with current prod values. Backend block uses prefix `freewhen/prod`.
3. Run `terraform state mv` to move every resource from the old root state to the new prod state. Plan must show **zero changes** before merging — if it doesn't, stop and reconcile.
4. Update `Makefile` `tf-plan` / `tf-apply` targets to take an `ENV=prod|staging` arg and `cd` into the right `envs/` dir.
5. Update `plans/infra/terraform-migration.md` log if relevant.

Risk gate: phase 1 ships on its own with no functional change. Anything past phase 1 is opt-in per env.

### Phase 2 — Add third-party providers to the module

Each behind a boolean var so envs can opt out:

- **Vercel** (`vercel/vercel`): one `vercel_project` resource per env (`freewhen` for prod, `freewhen-staging` for staging), each with its own `vercel_project_environment_variable` set (`NEXT_PUBLIC_API_URL`, `NEXT_PUBLIC_LOG_LEVEL`, `NEXT_PUBLIC_POSTHOG_KEY`, `NEXT_PUBLIC_POSTHOG_HOST`) and `vercel_project_domain` for the env's hostname. Both projects point at the same git repo; staging can be wired to deploy from `main` (production target) so it tracks trunk continuously.
- **Neon** (`kislerdm/neon`): `neon_branch` off `main`, `neon_role` for the app. Output `database_url` and write it to `google_secret_manager_secret_version` so Cloud Run picks it up on next deploy.
- **PostHog** (`posthog/posthog`): one project per env. Outputs project key → Vercel env var + Cloud Run env.
- **Resend** (`resend/resend`, community): manage domain + API key. Domain DNS records exposed as TF outputs for now; phase 4 candidate to wire into a DNS provider directly.

For prod, import existing resources into state so nothing recreates. Diff carefully.

### Phase 3 — Stand up staging

1. `infra/envs/staging/main.tf` calls the module with staging values:
   - `env_name = "staging"`
   - `service_name = "freewhen-staging"`
   - `cors_origin = "https://staging.freewhen.simonperryman.co.uk"`
   - `cookie_domain = ".staging.freewhen.simonperryman.co.uk"`
   - `max_instances = 1`, `cpu_limit = "1"`, `memory_limit = "256Mi"`
   - `auth_email_disabled = false` — staging sends real emails via a verified `mail.staging.freewhen.simonperryman.co.uk` subdomain in Resend with a separate API key (so a staging bug can't burn prod's sender reputation). ~30–50 min of DNS + Resend setup.
   - `neon_parent_branch = "main"`
2. `terraform apply` in `infra/envs/staging`.
3. Manual: in the existing Google OAuth client, add staging redirect URIs for `/auth/google/callback` and `/auth/google/calendar/callback`. Document this in `infra/envs/staging/README.md`.
4. Add `staging.freewhen.simonperryman.co.uk` and `api.staging.freewhen.simonperryman.co.uk` DNS records — leave outside TF for now if registrar provider isn't wired yet.
5. Smoke test: hit staging, run a no-op migration, verify it boots.

### Phase 4 — CI and deploy model

**Branching/deploy model: trunk-based with gated prod promotion.**

- `main` is the only long-lived branch. Anything merged to `main` is asserted prod-ready by the author (use feature flags from `flags.ts` to merge dark code).
- `deploy-staging.yml` runs on every push to `main`. Builds the image once, tags it with the commit SHA, pushes to Artifact Registry, deploys to `freewhen-staging` Cloud Run.
- `deploy-prod.yml` runs on `workflow_dispatch`, gated by a GitHub `production` environment with required reviewer (= owner). It takes the commit SHA as input and deploys **the same image** that's already running on staging — no rebuild. This is critical: a rebuild on `main` HEAD risks pulling in commits that landed after the staging deploy, defeating the soak.
- `tf-plan` PR-comment job runs against both `infra/envs/prod` and `infra/envs/staging` so drift in either is visible.

**Why this model (vs. deploying main to both at once):**

Feature flags cover application-logic risk, but they cannot gate:

- DB migrations (run on boot)
- Env var / secret changes
- Config changes (CORS, cookie domain, trusted proxy depth)
- Dependency upgrades (Go runtime, Node, libraries)
- Cloud Run runtime config (scaling, memory, timeout)
- Anything in `infra/` that affects backend runtime behaviour

That class of change is what staging exists to catch. **Flags gate features, staging gates infrastructure** — different threats, neither replaces the other.

**Operational shape:**

- Typical feature work: merge → flag stays off → promote prod whenever convenient → flip flag later. The promote click is routine because nothing is user-visible yet.
- Risky changes (migrations, config, infra, deps): merge → watch staging for as long as you want → promote when satisfied. The promote click is the deliberate gate.
- Hotfix: same path, just promote immediately after staging deploy goes green. No special branch.
- Collisions (someone else merges between your staging deploy and your promote): the SHA-pinned promote means you ship exactly what you reviewed; their commit will need its own promote afterwards.

### Phase 5 (optional, future) — DNS as code

Wire Cloudflare or Cloud DNS provider so the Resend domain DNS records and `*.freewhen.simonperryman.co.uk` records are created automatically when a new env is added. After this, a new env is purely `tfvars + apply`.

## What "N environments" looks like once done

```
mkdir infra/envs/demo
cp infra/envs/staging/main.tf infra/envs/demo/main.tf   # edit strings
terraform -chdir=infra/envs/demo init
terraform -chdir=infra/envs/demo apply
```

Plus one Google OAuth console click. Everything else — Cloud Run, secrets, Neon branch, PostHog project, Resend domain, Vercel envs — is provisioned by `apply`.

## Effort

- Phase 1: 2–3 hrs (`state mv` is the fiddly part)
- Phase 2: 3–4 hrs (provider-by-provider, one PR each)
- Phase 3: ~30 min after phases 1–2 land
- Phase 4: 1–2 hrs
- Phase 5: 1–2 hrs, deferrable

Roughly a focused day spread across several PRs. Phases 1 and 3 alone get a working staging environment if we punt on the third-party providers and configure Vercel/PostHog/Neon by hand once.

## Decisions

- **Branching model:** trunk-based. `main` deploys to staging automatically; prod is a manual `workflow_dispatch` promotion of the same image SHA. Feature flags (`flags.ts`) cover application-logic risk; staging covers everything flags can't (migrations, config, infra, deps).
- **Staging email:** real, via verified `mail.staging.freewhen.simonperryman.co.uk` subdomain in Resend with a separate API key.
- **Vercel topology:** two separate projects (`freewhen` for prod, `freewhen-staging` for staging), both pointing at the same repo. Clean separation of analytics, Speed Insights budgets, deploy history, and rollback scope. Both projects managed via the `vercel` Terraform provider so the "two dashboards" overhead is cosmetic — env vars, domains, and project settings are declared in HCL.

## Log

- 2026-05-08 — Created plan
- 2026-05-09 — Decided trunk-based deploy model with SHA-pinned prod promotion, real emails on staging via separate Resend domain + key. Vercel project topology still open.
- 2026-05-09 — Decided two Vercel projects (separate prod + staging), managed via `vercel/vercel` Terraform provider. All open questions now closed.
