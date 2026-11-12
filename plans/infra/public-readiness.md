# Public readiness

Status: done

## Goal

Prepare the repo for being flipped from private to public on GitHub: scrub
personal data and account identifiers from tracked files, tighten GitHub
Actions token permissions so fork PRs can't do anything harmful, and harden
`.gitignore` against accidentally committing secrets or Terraform artefacts.
No LICENSE yet — the owner has deferred that decision. Git history is not
rewritten; the scrubbed values are low-sensitivity identifiers, not secrets.

## Plan

1. Replace the real GCP billing account ID in `plans/infra/gcp-billing-budget.md`
   with `XXXXXX-XXXXXX-XXXXXX`.
2. Workflows (`ci.yml`, `deploy.yml`):
   - Top-level `permissions: contents: read`; jobs only add what they need
     (`changes`: `pull-requests: read`; `tf-plan`: `id-token: write` +
     `pull-requests: write`; `tf-apply` / `deploy-backend`: `id-token: write`
     for WIF).
   - `tf-plan` skips fork PRs (secrets + WIF unavailable there anyway), still
     runs on push.
   - Pin `vercel@latest` to a specific version.
3. Root `.gitignore`: `.env*` with `!.env.example` exceptions, `plan.txt`,
   `tfplan`, `*.tfvars` with `!infra/terraform.tfvars` kept tracked.
4. Replace personal names / email addresses in plans and mocks with
   placeholders (`Alex Morgan`, `@example.com`). The prod domain
   `freewhen.simonperryman.co.uk` and the `SimonPerryman/freewhen` repo slug
   stay — both are already public.
5. Manual GitHub settings for the owner (cannot be done from the repo):
   - Settings → Actions → General → Workflow permissions: **Read repository
     contents and packages permissions** (read-only default `GITHUB_TOKEN`).
   - Settings → Actions → General → Fork pull request workflows from outside
     collaborators: **Require approval for all outside collaborators**.
   - Set local git `user.email` to the GitHub noreply address
     (`<id>+SimonPerryman@users.noreply.github.com`) and enable "Keep my email
     addresses private" + "Block command line pushes that expose my email" in
     GitHub → Settings → Emails. Existing commits keep the old address.
   - Optionally enable secret scanning + push protection and Dependabot
     alerts once public (free for public repos).

## Log

- 2026-11-12 10:00 — Created plan
- 2026-11-12 10:40 — Scrubbed billing account ID, real name (demo HTML -> Alex Morgan) and personal emails (-> example.com placeholders). Grep found no other personal emails; first-name "Simon" in sample data and the `SimonPerryman/freewhen` slug / prod domain left as-is. GCP project number in `gate-deploy-on-ci.md` left (not a secret; WIF is gated on repo attribute).
- 2026-11-12 10:45 — Workflows: top-level `permissions: contents: read` in ci.yml + deploy.yml (existing per-job grants already covered changes/tf-plan/tf-apply/deploy-backend; head-check, ci, e2e, deploy-frontend need only contents: read). tf-plan now skips fork PRs. Pinned Vercel CLI to `vercel@65.8.0` (~2 weeks old; latest is 65.14.0).
- 2026-11-12 10:46 — `.gitignore`: `.env*` (+ `!.env.example`), `plan.txt`, `tfplan`, `.terraform/`, `*.tfvars` with `!infra/terraform.tfvars`. Verified via `git ls-files -ci --exclude-standard` that no tracked file is newly ignored.
- 2026-11-12 10:50 — Local gate: oxlint + oxfmt clean. Frontend vitest has 13 localStorage failures caused by local Node v26 (repo pins 24) — pre-existing env issue, no code touched. Go toolchain not installed locally; relying on CI for backend lint/test.
