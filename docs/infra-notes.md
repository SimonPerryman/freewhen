# Infrastructure Notes

## Hosting

| Layer | Provider | Notes |
|-------|----------|-------|
| Frontend | Vercel | Deployed from CI on push to main (Vercel git auto-deploy on `main` is disabled via `frontend/vercel.json`); PR previews still come from Vercel's git integration |
| Backend | Google Cloud Run | Single Go binary, scale-to-zero, deploy via `gcloud run deploy --source .` |
| Database | Neon | Managed Postgres, free tier, pooled connection string |

### Backend alternatives considered

- **Fly.io** — clean ergonomics, but $5/month minimum charge per app even when idle. Cloud Run scales to zero and stays in the free tier at our expected traffic.
- **Render** — more point-and-click, fine if `gcloud` ergonomics grate
- **Railway** — similar tier
- Revisit if Cloud Run cold-starts bite or GCP billing gets weird

## Runtime & Tooling

- Node 24 LTS + pnpm (Corepack-managed)
- Bun is opt-in speedup only, not a dependency
  - `bunx vitest` / `bun run script.ts` can shave seconds without committing the whole project to Bun
  - Do not add Bun to CI or `package.json` scripts without a real measured reason

## Faster CI Options

If GitHub Actions minutes become a bottleneck:

| Option | Detail |
|--------|--------|
| Blacksmith | 3,000 free min/month, ~2x faster, swap via `runs-on:` change |
| WarpBuild | Free credit then ~$0.003/min Linux |
| GitHub native | Cut hosted-runner prices up to 39% on Jan 1 2026 — gap is narrower |

## Configuration

Backend env vars (all read once at startup in `internal/config/config.go`):

| Var | Default | Notes |
|-----|---------|-------|
| `DATABASE_URL` | — | Required |
| `PORT` | `8080` | |
| `CORS_ORIGIN` | `http://localhost:3000` | |
| `LOG_LEVEL` | `info` | `debug` \| `info` \| `warn` \| `error`. JSON to stdout always — pipe through `jq` locally if you want readability. |
| `SECURE_COOKIES` | `true` | Set `false` for local HTTP dev. Prod / staging leave unset → defaults to secure. |
| `TRUSTED_PROXY_DEPTH` | `0` | Number of trailing `X-Forwarded-For` entries that belong to trusted infrastructure. `0` is correct for Cloud Run direct ingress (GFE appends only the real client IP). Set `1` if running behind a Google external HTTPS load balancer. Used by per-IP rate limiters; getting it wrong lets clients spoof their IP and poison rate-limit buckets. |
| `POSTHOG_KEY` | unset | PostHog project API key. If unset, backend flag evaluation no-ops (returns caller-supplied fallback). |
| `POSTHOG_HOST` | unset | PostHog API host (e.g. `https://eu.i.posthog.com`). Leave unset to use the SDK default. |
| `POSTHOG_PERSONAL_API_KEY` | unset | Optional. Enables local polling of flag definitions for fast in-process evaluation. Without it, the SDK still works but evaluates flags remotely. |

Frontend env vars:

| Var | Default | Notes |
|-----|---------|-------|
| `NEXT_PUBLIC_API_URL` | — | Backend base URL |
| `NEXT_PUBLIC_LOG_LEVEL` | `info` | Same semantics as backend `LOG_LEVEL`. `NODE_ENV` is never read. |
| `NEXT_PUBLIC_POSTHOG_KEY` | unset | If unset, analytics is a no-op. |
| `NEXT_PUBLIC_POSTHOG_HOST` | unset | |

> **Vercel `NEXT_PUBLIC_*` vars must be added with `--no-sensitive`.** Vercel
> defaults Production/Preview env vars to sensitive (write-only); `vercel pull`
> in CI then writes them as `""` and the build inlines empty strings, breaking
> the deployed bundle. See `docs/deploy-checklist.md` for the exact commands and
> verification steps.

The backend handles `SIGTERM`/`SIGINT` with a 25-second graceful shutdown — Cloud Run traffic drains cleanly on new revisions.

## Cost guardrails

Two layers, both managed in `infra/`:

- **Hard cap:** `cloud_run.tf` sets `max_instance_count = 3`. A runaway loop or traffic spike can't scale infinitely — Cloud Run sheds load instead of billing for it.
- **Early warning:** `infra/billing.tf` defines a £5/month project-wide budget on the billing account, with email alerts at 50%, 90%, 100%, and 150% of the cap. Routed via an explicit `google_monitoring_notification_channel` so the recipient list stays in code, not implicit billing IAM. Console: Billing → Budgets & alerts → "FreeWhen monthly spend".

The budget needs two things that aren't in TF state and have to be granted out-of-band on the billing account (TF can't bootstrap its own billing-account access without `billing.admin`, which is too broad to hand to CI):

- `roles/billing.costsManager` on the tf-apply SA
- `roles/billing.viewer` on the tf-plan SA

Inputs (`billing_account_id`, `alert_email`) live in `secrets.auto.tfvars` locally and as `GCP_BILLING_ACCOUNT_ID` / `ALERT_EMAIL` GitHub secrets in CI — both seeded by the first local apply.

## Observability

### Admin metrics endpoint

`GET /admin/metrics` returns SQL-derived product metrics (users, calendars, participants). Requires `is_admin = true` on the user row.

To grant admin to a user:

```sql
UPDATE users SET is_admin = true WHERE email = 'you@example.com';
```

### PostHog (analytics + session replay + error tracking)

- **Project:** PostHog EU cloud (`https://eu.i.posthog.com`)
- **SDK:** `posthog-js`, initialised in `frontend/src/lib/analytics.ts`
- **Config:** `NEXT_PUBLIC_POSTHOG_KEY` and `NEXT_PUBLIC_POSTHOG_HOST` env vars on Vercel. If `NEXT_PUBLIC_POSTHOG_KEY` is unset, all analytics calls are no-ops (dev default).
- **Person profiles:** `identified_only` — anonymous visitors are not profiled.
- **Events tracked:** `calendar_created`, `share_link_clicked`, `calendar_opened`, `participant_added`, `availability_marked`

**Consent banner required before wider rollout.** Session replay + analytics cookies need explicit consent under UK/EU rules. The `analytics.init()` call in `src/lib/analytics.ts` is the single hook point for gating behind a consent banner.

### PostHog feature flags

- **Frontend:** `frontend/src/lib/flags.ts` exposes `isEnabled(key, fallback)`, `getVariant(key, fallback)`, and `onFlagsLoaded(cb)`. Call sites never touch `posthog-js` directly, so swapping vendors later is a library change, not a codebase sweep.
- **Backend:** `internal/flags/` exposes an `Evaluator` interface threaded onto the handler struct via `WithFlags(...)`. The PostHog implementation reads `POSTHOG_KEY`, `POSTHOG_HOST`, and `POSTHOG_PERSONAL_API_KEY` from env. When `POSTHOG_KEY` is unset, a `Noop` evaluator returns the caller-supplied fallback — same 12-factor pattern as the frontend.
- **Naming:** `kebab-case`, prefixed with the area it gates — e.g. `calendar-import-suggestions`, `auth-magic-link-v2`, `share-cta-v2`.
- **Lifecycle:** create in PostHog dashboard → wrap a single code path → roll 0% → 10% → 100% over a few days → once stable for a week, delete the flag *and* the code branch it gated. Long-lived flags accumulate into the next observability problem.
- **Events:** PostHog fires a `$feature_flag_called` event on every evaluation. This counts against the 1M events/mo allowance — keep flag keys out of tight loops (e.g. don't call `isEnabled` inside a render that runs 60x/sec).

## Deploy Flow

- **Backend:** CI runs `gcloud run deploy --source .` on push to main, gated on lint/test/build passing. Migrations run on boot. First request after idle pays a ~1s cold start.
- **Frontend:** CI runs `vercel deploy --prebuilt --prod` on push to main, gated on the same `ci` job. Vercel's own git auto-deploy on `main` is disabled in `frontend/vercel.json`. PR previews still come from Vercel's git integration (not gated on CI — they're previews).

## First-Time Setup

1. **Create the GCP project and enable APIs:**
   ```
   gcloud projects create freewhen --set-as-default
   gcloud services enable run.googleapis.com cloudbuild.googleapis.com \
     secretmanager.googleapis.com artifactregistry.googleapis.com
   ```
2. **Store the database URL in Secret Manager:**
   ```
   echo -n "$DATABASE_URL" | gcloud secrets create freewhen-database-url \
     --data-file=- --replication-policy=automatic
   ```
3. **Set up Workload Identity Federation for GitHub Actions** following
   [Google's guide](https://github.com/google-github-actions/auth#setting-up-workload-identity-federation).
   Capture the provider resource name and the deploy service account email.
   Grant the service account `roles/run.admin`, `roles/iam.serviceAccountUser`,
   `roles/secretmanager.secretAccessor`, and `roles/artifactregistry.writer`.
4. **Add the WIF values to GitHub repo secrets:**
   - `GCP_WIF_PROVIDER` — full provider resource name
   - `GCP_DEPLOY_SA` — deploy service account email
   - `VERCEL_TOKEN` — account-level token from Vercel → Account Settings → Tokens
   - `VERCEL_ORG_ID` / `VERCEL_PROJECT_ID` — from `frontend/.vercel/project.json` after running `vercel link` locally, or from the Vercel project settings
5. **(Optional) Map a custom domain:**
   ```
   gcloud run domain-mappings create --service freewhen-api \
     --domain api.freewhen.simonperryman.co.uk --region europe-west1
   ```
   DNS propagation can take a few minutes; the `*.run.app` URL works immediately.
6. **Point the frontend at the backend:** set `NEXT_PUBLIC_API_URL` in Vercel
   to the Cloud Run URL (or your custom domain). Update `CORS_ORIGIN` on the
   service to match the Vercel domain.

## Secret rotation

All runtime secrets live in Secret Manager and are referenced by `infra/secrets.tf` (data sources, never managed). Rotate by adding a new version — Cloud Run reads `version = "latest"`, so the new revision picks it up on next deploy or after a `gcloud run services update --clear-revision`-style nudge.

### Resend API key (`freewhen-resend-api-key`)

1. Generate a new key in the [Resend dashboard](https://resend.com/api-keys) with **send-only** scope. Note the key — Resend shows it once.
2. Push the new value as a fresh version:
   ```
   echo -n "$NEW_KEY" | gcloud secrets versions add freewhen-resend-api-key --data-file=-
   ```
3. Trigger a deploy (push to main, or `gcloud run services update freewhen-api --region=europe-west1` no-op) so the running revision picks up `latest`.
4. Smoke-test by requesting a magic link against prod and confirming receipt.
5. Revoke the old key in the Resend dashboard.

Same shape for `freewhen-database-url`, `freewhen-calendar-secret`, `freewhen-google-client-id`, `freewhen-google-client-secret` — the secret IDs are the only things that change. Local `.env.local` is gitignored; rotate that copy by editing the file, never by committing.

## Email deliverability (SPF / DKIM / DMARC)

Magic-link mail is sent via Resend from `mail.freewhen.simonperryman.co.uk` (a verified subdomain). SPF, DKIM, and the return-path are managed automatically by Cloudflare's Resend integration. DMARC is a manual TXT record at `_dmarc.freewhen` with `p=quarantine; adkim=r; aspf=r`.

### DMARC aggregate (RUA) reports

Receivers (Gmail, Microsoft, Yahoo, etc.) send a daily zipped XML report to whatever address is in the `rua=` tag, summarising:

- Per sending IP, message counts claiming to be from the domain
- SPF / DKIM pass or fail and which domain they aligned with
- Whether DMARC overall passed
- What the receiver did with failures (none / quarantine / reject)

Use cases: spot legitimate senders that aren't aligned (need DKIM/SPF added), spot spoofing, validate before tightening policy from `p=quarantine` → `p=reject`.

**Reporting interval (`ri=`) is effectively daily.** The RFC lets you request other intervals but Google/Microsoft/Yahoo all ignore non-daily values. Don't bother setting `ri=`.

### Reducing inbox noise from RUA

Three options, in order of preference:

1. **Forward to a free DMARC processor** (Postmark's [dmarc.postmarkapp.com](https://dmarc.postmarkapp.com), dmarcian, etc.) — they ingest the daily zips and email a weekly digest. Postmark's DMARC product is separate from their sending product; no sending account needed.
2. **Gmail filter** — auto-archive the daily zips to a label, check monthly. Zero third party in the loop.
3. **Drop `rua=` entirely** — remove the tag from the Cloudflare TXT record at `_dmarc.freewhen` and receivers stop reporting. Loses visibility into new senders and spoofing attempts; only do this once the sender list is stable and policy is `p=reject`.

### How to remove `rua=`

1. Cloudflare → `simonperryman.co.uk` zone → DNS → find the TXT record at `_dmarc.freewhen`.
2. Edit the value: drop the `rua=mailto:...;` segment, leaving e.g. `v=DMARC1; p=quarantine; adkim=r; aspf=r`.
3. Save. No backend change needed.

## Integration Tests

`make test-integration` runs the Go integration suite against a real Postgres.
On first run, [`fergusstrange/embedded-postgres`](https://github.com/fergusstrange/embedded-postgres)
downloads a Postgres binary (~60MB) to `~/.embedded-postgres-go/` and caches it
there for subsequent runs. No Docker or local Postgres install required.

CI caches this directory via `actions/cache` so runs don't re-download the
binary each time. The cache key is keyed on OS+arch+a manual version — bump it
when upgrading the embedded-postgres library.

To run against an existing database instead (e.g. a Neon branch), set
`TEST_DATABASE_URL` and the embedded server is skipped.
