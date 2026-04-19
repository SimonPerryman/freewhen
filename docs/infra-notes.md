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
