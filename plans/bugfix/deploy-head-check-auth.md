# Fix Deploy head-check skipping every run on private repo

Status: done

## Goal

The `head-check` job in `.github/workflows/deploy.yml` runs `git ls-remote` against the repo URL with no auth. The repo is private, so the command fails with `could not read Username for 'https://github.com'`, `HEAD_SHA` is empty, the equality check against `github.sha` fails, and every deploy run short-circuits as "superseded" — silently skipping the actual Cloud Run / Vercel deploy.

Symptom: PR #28 (`ui(calendars): replace Duplicate button with overflow menu`) merged with green CI but never deployed. The Deploy run completed in 7s with notice `Superseded by  — skipping deploy for 37933eca…` (note the empty SHA).

## Plan

1. Replace `git ls-remote https://github.com/${{ github.repository }}.git …` with `gh api repos/${{ github.repository }}/commits/main --jq .sha`, passing `GH_TOKEN: ${{ github.token }}` via env. `gh` is preinstalled on GitHub-hosted runners and uses the workflow's GITHUB_TOKEN, which has the `contents: read` permission already granted to this job's default token.
2. Manually re-trigger the deploy for the merged-but-undeployed `37933eca…` commit by re-running the Deploy workflow run on main (or, if re-run is rejected because the commit is no longer HEAD, push a no-op commit).

## Log

- 2026-05-05 07:30 — Created plan; diagnosed empty-SHA "superseded" skip caused by unauthenticated `git ls-remote` against a private repo.
- 2026-05-05 10:10 — Shipped fix in PR #29 (squash-merged). Switched head-check to `gh api repos/<repo>/commits/main --jq .sha` with `GH_TOKEN: github.token`. Post-merge Deploy run now proceeds past head-check instead of 7s skip.
