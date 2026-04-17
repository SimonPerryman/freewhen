# Infrastructure Notes

## Hosting

| Layer | Provider | Notes |
|-------|----------|-------|
| Frontend | Vercel | Auto-build from git, free hobby tier fits V1 |
| Backend | Fly.io | Single Go binary, deploy via `flyctl` |
| Database | Neon | Managed Postgres, free tier, pooled connection string |

### Backend alternatives considered

- **Render** — more point-and-click, fine if `flyctl` ergonomics grate
- **Railway** — similar tier
- Revisit if Fly bills get weird or cold-start latency bites

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

- **Backend:** CI runs `flyctl deploy` on push to main. Migrations run on boot.
- **Frontend:** Vercel's git integration deploys on push. Preview deploys on PRs.

## First-Time Setup

1. Create Neon project, copy `DATABASE_URL`
2. `fly launch --no-deploy` in `backend/`, then set secrets:
   ```
   fly secrets set DATABASE_URL=... CORS_ORIGIN=... ENV=prod
   ```
3. Create `FLY_API_TOKEN` (`fly tokens create deploy`), paste into GH repo secrets
4. Import repo in Vercel, set root to `frontend/`, set `NEXT_PUBLIC_API_URL` to Fly app URL
5. Point a custom domain (optional); update `CORS_ORIGIN` to match
