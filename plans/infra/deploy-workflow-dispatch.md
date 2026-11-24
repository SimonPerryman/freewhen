# Manual "deploy everything" trigger

Status: done

## Goal

The deploy workflow only ships what a push changed (paths-filter against the previous commit).
While the repo was private and out of Actions minutes, #70–#106 merged without deploying, and
re-running the latest run would only see #106's `ci.yml` change. Add a manual trigger that deploys
backend, frontend and infra regardless of the diff.

## Plan

1. Add `workflow_dispatch` to `deploy.yml`.
2. Make the `changes` outputs `true` on a dispatch run, so every deploy job runs.
3. Merging this PR deploys everything anyway (`deploy.yml` is in all three path filters); the
   dispatch is for next time.

## Log

- 2026-11-24 09:40 — Created plan; added the trigger and forced outputs. Workflow-only change, so
  `make lint`/`make test` don't cover it.
- 2026-11-24 19:40 — Audit: shipped in #107; a manual run on 2026-11-24 09:33 deployed everything successfully. Marked done.
