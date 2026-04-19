# Infrastructure Notes

## Hosting

| Layer | Provider | Notes |
|-------|----------|-------|
| Frontend | Vercel | Auto-build from git, free hobby tier fits V1 |
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

Frontend env vars:

| Var | Default | Notes |
|-----|---------|-------|
| `NEXT_PUBLIC_API_URL` | — | Backend base URL |
| `NEXT_PUBLIC_LOG_LEVEL` | `info` | Same semantics as backend `LOG_LEVEL`. `NODE_ENV` is never read. |
| `NEXT_PUBLIC_POSTHOG_KEY` | unset | If unset, analytics is a no-op. |
| `NEXT_PUBLIC_POSTHOG_HOST` | unset | |

The backend handles `SIGTERM`/`SIGINT` with a 25-second graceful shutdown — Cloud Run traffic drains cleanly on new revisions.

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
- **Events tracked:** `calendar_created`, `invite_shared`, `calendar_opened`, `participant_added`, `availability_marked`

**Consent banner required before wider rollout.** Session replay + analytics cookies need explicit consent under UK/EU rules. The `analytics.init()` call in `src/lib/analytics.ts` is the single hook point for gating behind a consent banner.

## Deploy Flow

- **Backend:** CI runs `gcloud run deploy --source .` on push to main. Migrations run on boot. First request after idle pays a ~1s cold start.
- **Frontend:** Vercel's git integration deploys on push. Preview deploys on PRs.

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
5. **(Optional) Map a custom domain:**
   ```
   gcloud run domain-mappings create --service freewhen-api \
     --domain api.freewhen.app --region europe-west2
   ```
   DNS propagation can take a few minutes; the `*.run.app` URL works immediately.
6. **Point the frontend at the backend:** set `NEXT_PUBLIC_API_URL` in Vercel
   to the Cloud Run URL (or your custom domain). Update `CORS_ORIGIN` on the
   service to match the Vercel domain.
