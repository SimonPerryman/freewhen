# Skip superseded deploys

Status: done

## Goal

When two PRs merge to `main` in quick succession, both trigger the Deploy
workflow. The `concurrency: { group: deploy-..., cancel-in-progress: false }`
setting already prevents the second run from cancelling the first mid-flight
(good — half-deployed state is the worst outcome). But once the first run
finishes, the second one will then ship an already-superseded SHA on top of
the newer one that just landed, briefly rolling production back to an older
revision before the next run catches up.

This plan adds a "is this still `main` HEAD?" guard at the start of the
Deploy workflow so a queued run for a stale SHA short-circuits to success
instead of redeploying old code. CI keeps `cancel-in-progress: true` (cheap
to interrupt); Deploy keeps `cancel-in-progress: false` (never interrupt a
rollout) and now also skips if it's been overtaken.

## Plan

1. Add a `head-check` job to the start of `.github/workflows/deploy.yml`.
   It compares `github.sha` against the current `refs/heads/main` SHA via
   `git ls-remote` (no auth needed for a public-read fetch on this repo)
   and exposes `outputs.is_head=true|false`.
2. Make `changes` depend on `head-check` and gate on `is_head == 'true'`.
   Because `tf-apply`, `deploy-backend`, and `deploy-frontend` all
   transitively depend on `changes` outputs, a skip cascades cleanly:
   a stale run shows green with every job either skipped or no-op'd.
3. On a stale SHA, log `::notice::Superseded by <newer-sha>` so the run
   history makes the reason obvious without digging through step output.
4. Don't touch `ci.yml` — its concurrency group already cancels older runs
   in-progress, which is the right behaviour pre-deploy.

## Notes / tradeoffs

- The check has a small race window: between `git ls-remote` and the actual
  deploy step, a newer commit could land. That's fine — the next queued run
  will then be the one that's `HEAD`, and it'll deploy. The guard is
  belt-and-braces on top of `concurrency`, not a strict serialisation.
- Skipping with exit 0 (rather than `cancel`) keeps the run history readable:
  green check = "we considered this SHA and decided not to ship it".
- No change to the queue semantics — runs still serialise per the existing
  concurrency group; we're just no-op'ing the body of stale ones.

## Log

- 2026-05-04 22:30 — Created plan
- 2026-05-04 22:35 — Added `head-check` job to `deploy.yml` and gated
  `changes` on it; downstream jobs cascade-skip via existing
  `needs.changes.outputs.*` conditions.
