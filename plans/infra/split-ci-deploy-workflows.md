## Split CI and deploy into separate workflows

Status: done

## Goal

Stop showing skipped `tf-apply` / `deploy-backend` / `deploy-frontend` jobs on PRs. Today these live in `ci.yml` gated by `github.ref == 'refs/heads/main'`, so every PR check-run list includes four "skipped" deploy jobs that don't tell the reviewer anything. Splitting clarifies what runs where: PRs run validation, main runs validation + deploy.

## Approach

Two workflow files instead of one:

1. **`.github/workflows/ci.yml`** — runs on `pull_request` *and* `push: branches: [main]`. Jobs: `changes`, `ci`, `e2e`, `tf-plan`. No deploy jobs. This stays the required-status-check on the branch protection rule.
2. **`.github/workflows/deploy.yml`** — runs on `push: branches: [main]` only. Jobs: `tf-apply`, `deploy-backend`, `deploy-frontend`.

Both files trigger on the same `push` event independently — no `workflow_run` chaining. The deploy workflow doesn't need to wait for `ci.yml` because branch protection already blocks merges until CI passes; once the merge commit lands on main, CI re-runs (belt-and-braces) and deploy runs in parallel. If we want stricter ordering later we can revisit `workflow_run`, but the simpler shape is fine for a small repo.

## Plan

1. Create `.github/workflows/deploy.yml`:
   - `on: push: branches: [main]`
   - Concurrency group `deploy-${{ github.ref }}` with `cancel-in-progress: false` (don't kill an in-flight Cloud Run rollout).
   - Duplicate the `changes` job (paths-filter only — needs to run here so deploy jobs can gate on `backend`/`frontend`/`infra` outputs). Acceptable duplication; the alternative is passing outputs across workflows which isn't supported cleanly.
   - Move `tf-apply`, `deploy-backend`, `deploy-frontend` jobs over verbatim, but:
     - Drop `needs: [ci, e2e]` and the `needs.ci.result == 'success'` / `needs.e2e.result == 'success'` guards. Branch protection enforces CI passing pre-merge.
     - Drop the `github.ref == 'refs/heads/main'` `if` guard — the workflow trigger already restricts to main.
     - Keep the `needs: [changes]` + `needs.changes.outputs.* == 'true'` gating so unchanged stacks don't redeploy.
     - Keep `tf-apply` as a `needs:` of `deploy-backend` so service config lands before the new image.
     - Keep `deploy-backend` as a `needs:` of `deploy-frontend` (with the `success || skipped` tolerance) so API ships before UI.
2. Edit `.github/workflows/ci.yml`:
   - Remove `tf-apply`, `deploy-backend`, `deploy-frontend` jobs.
   - Keep `changes`, `ci`, `e2e`, `tf-plan` as-is.
   - Concurrency group stays the same.
3. Verify branch protection on `main` requires the `ci` job (and `e2e` / `tf-plan` if currently required). No change needed unless the required check names referenced the deploy jobs — they shouldn't, since those skip on PRs.
4. Smoke test:
   - Open a no-op PR — confirm only `changes` / `ci` / `e2e` / `tf-plan` show up; no skipped deploy noise.
   - Merge to main — confirm `ci.yml` and `deploy.yml` both run; `deploy.yml` correctly skips backend/frontend/infra jobs that didn't change.
   - Land an infra-only change to confirm `tf-apply` runs and `deploy-backend`/`deploy-frontend` skip.

## Tradeoffs / non-goals

- **Duplicated `changes` job** — accepted. The job is ~10s and avoids a brittle artifact-passing dance between workflows.
- **No `workflow_run` chaining** — explicitly chosen for simplicity. Deploy doesn't strictly wait for the post-merge CI run; we rely on branch protection blocking bad merges. If we ever see a deploy succeed while post-merge CI fails on a flake, revisit.
- **Concurrency** — deploy uses `cancel-in-progress: false` (different from CI) so two rapid merges don't interrupt a Cloud Run rollout mid-flight.

## Log

- 2026-05-04 — Created plan
- 2026-05-04 — Created `.github/workflows/deploy.yml` with duplicated `changes` job and moved `tf-apply` / `deploy-backend` / `deploy-frontend` over. Dropped `needs: [ci, e2e]`, the `ci.result == 'success'` / `e2e.result == 'success'` guards, and the `github.ref == 'refs/heads/main'` `if` (workflow trigger restricts to main). Used `cancel-in-progress: false` on the deploy concurrency group. Stripped the same three jobs from `ci.yml`; `changes` / `ci` / `e2e` / `tf-plan` remain.
