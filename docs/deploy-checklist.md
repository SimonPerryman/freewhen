# Deploy Checklist

Where every secret and config value lives in production. Assumes the prod
domain is `freewhen.simonperryman.co.uk` — substitute your own throughout.

## GCP Secret Manager (sensitive backend values)

Stored as secrets, mounted into Cloud Run via `--set-secrets`.

| Secret name | Value |
|---|---|
| `freewhen-database-url` | `DATABASE_URL` from Neon (pooled connection string) |
| `freewhen-calendar-access-secret` | `CALENDAR_ACCESS_SECRET` (`openssl rand -base64 48`) |
| `freewhen-resend-api-key` | `RESEND_API_KEY` from Resend dashboard |
| `freewhen-google-client-secret` | `GOOGLE_CLIENT_SECRET` from Google Cloud Console |
| `freewhen-posthog-key` | `POSTHOG_KEY` (`phc_…`) from PostHog project settings |
| `freewhen-posthog-personal-api-key` | `POSTHOG_PERSONAL_API_KEY` (`phx_…`), scope: Feature Flag read |

```
echo -n "$VALUE" | gcloud secrets create <name> --data-file=- --replication-policy=automatic
```

## Cloud Run env vars (non-sensitive backend config)

Set on the service via `--set-env-vars` (or in the GitHub Actions deploy step).

| Var | Value |
|---|---|
| `PORT` | `8080` |
| `LOG_LEVEL` | `info` |
| `CORS_ORIGIN` | `https://freewhen.simonperryman.co.uk` |
| `SECURE_COOKIES` | `true` |
| `COOKIE_DOMAIN` | `.freewhen.simonperryman.co.uk` |
| `FRONTEND_URL` | `https://freewhen.simonperryman.co.uk` |
| `AUTH_EMAIL_DISABLED` | `false` |
| `MAGIC_LINK_FROM_EMAIL` | `auth@freewhen.simonperryman.co.uk` (verified Resend sender) |
| `GOOGLE_CLIENT_ID` | from Google Cloud Console |
| `GOOGLE_REDIRECT_URL` | `https://api.freewhen.simonperryman.co.uk/auth/google/callback` |
| `GOOGLE_CALENDAR_REDIRECT_URL` | `https://api.freewhen.simonperryman.co.uk/auth/google/calendar/callback` |
| `POSTHOG_HOST` | `https://eu.i.posthog.com` |

## Vercel env vars (frontend, all environments)

Project → Settings → Environment Variables. Set for Production (Preview can
reuse Production values, or point at a staging Cloud Run if you have one).

| Var | Value |
|---|---|
| `NEXT_PUBLIC_API_URL` | `https://api.freewhen.simonperryman.co.uk` |
| `NEXT_PUBLIC_LOG_LEVEL` | `info` |
| `NEXT_PUBLIC_POSTHOG_KEY` | same `phc_…` value as backend `POSTHOG_KEY` |
| `NEXT_PUBLIC_POSTHOG_HOST` | `https://eu.i.posthog.com` |

`NEXT_PUBLIC_*` is baked into the client bundle at build time — these are not
secret, but Vercel is the single source of truth for them.

## GitHub Actions secrets (CI/CD plumbing)

Repo → Settings → Secrets and variables → Actions. Used by the deploy
workflow only — never read by the app.

| Secret | Value |
|---|---|
| `GCP_WIF_PROVIDER` | Workload Identity Federation provider resource name |
| `GCP_DEPLOY_SA` | Deploy service account email |

## External setup actions (no env vars, but required)

- **Neon**: create project, grab pooled `DATABASE_URL`
- **Resend**: verify the sending domain (`freewhen.simonperryman.co.uk`) so
  `MAGIC_LINK_FROM_EMAIL=auth@…` works
- **Google Cloud Console** (OAuth client): add the two prod redirect URIs
  alongside the localhost ones
- **GCP project**: create, enable APIs, set up Workload Identity Federation
  for GitHub Actions (see `infra-notes.md` step 3)
- **DNS**:
  - `freewhen.simonperryman.co.uk` → Vercel (CNAME `cname.vercel-dns.com`)
  - `api.freewhen.simonperryman.co.uk` → Cloud Run domain mapping target
- **PostHog**: per-product billing limits set to `$0` if you want hard caps

## Local development

Copy `backend/.env.example` → `backend/.env` and `frontend/.env.example` →
`frontend/.env.local`. Fill in localhost defaults plus any keys you want to
exercise locally (Resend, Google OAuth, PostHog).
