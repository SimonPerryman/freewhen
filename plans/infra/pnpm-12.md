# pnpm 12

Status: in-progress

## Goal

Move the frontend's pinned pnpm from 10.34.9 to the latest pnpm 12 (12.23.0). Stacks on the Node 26 move (#130): Node 26 has no Corepack, so a global pnpm switches itself to the `packageManager` pin, and that pin is the one place the version lives.

## Plan

- `frontend/package.json`: `packageManager` → `pnpm@12.23.0`.
- Regenerate `pnpm-lock.yaml` with pnpm 12 without re-resolving anything. Dependency resolutions must stay byte-identical; only the format may change.
- Handle the pnpm 11/12 breaking changes that reach this repo:
  - **`strictDepBuilds` is on by default (pnpm 11).** An install fails on any dependency with a build script that isn't listed in `allowBuilds`. `onlyBuiltDependencies` and friends are gone.
  - **Settings live only in `pnpm-workspace.yaml` (pnpm 11).** pnpm no longer reads settings from `.npmrc` (auth and registry only) or the `pnpm` field of `package.json`. The repo has neither, so `allowBuilds` goes in a new `frontend/pnpm-workspace.yaml`. pnpm 12 fails on unknown keys in that file.
  - **`minimumReleaseAge` defaults to 1 day (pnpm 11).** pnpm 12 also checks the lockfile against it on frozen installs. Keep the default. It matches Dependabot's 3-day cooldown.
  - Removed `managePackageManagerVersions` etc., `npm_config_*` env vars, `--frozen-lockfile false`, git-dependency changes: nothing in the repo uses them.
- CI: check `pnpm/action-setup` handles pnpm 12 and `pnpm dlx vercel@…` still works. Check Vercel supports pnpm 12.
- Docs: AGENTS.md, README.md, frontend/README.md, docs/infra-notes.md, docs/stack.md.
- Run `make lint`, `make test` and the e2e suite.

## Log

- 2026-11-24 19:30 — Created plan. Latest pnpm 12 is 12.23.0. Read the 11.0.0 and 12.0.0 release notes.
- 2026-11-24 19:35 — Bumped `packageManager`. `pnpm install` rewrote the lockfile as two YAML documents: a new first document that pins pnpm itself (`packageManagerDependencies`, the `@pnpm/exe.*` binaries for each platform), then the existing lockfile. The second document is byte-identical to the old lockfile, so no dependency changed. `lockfileVersion` stays `9.0`.
- 2026-11-24 19:35 — The install then failed twice. First on `minimumReleaseAge`: `posthog-js@1.454.2` and `@posthog/browser-common@0.13.1` were published on 2026-11-23 around 22:00 UTC, less than a day ago. They pass from around 22:00 UTC today. Kept the policy; local installs used `--config.minimumReleaseAge=0` until then. Second on `ERR_PNPM_IGNORED_BUILDS` for `core-js` (its postinstall only prints a funding banner, and pnpm 10 skipped it too). Added `frontend/pnpm-workspace.yaml` with `allowBuilds: { core-js: false }`.
- 2026-11-24 19:45 — `pnpm/action-setup` v6.1.1 (the current pin and its latest release) says it supports pnpm 12 and reads `packageManager`, so CI needs no edits. `pnpm dlx vercel@65.8.0` works under pnpm 12. It warns that it ignored build scripts for `esbuild` and `koffi`, which pnpm 10 also skipped. Vercel's package-manager docs list pnpm 6–12 as supported.
- 2026-11-24 19:55 — `make lint` and `make test` pass (528 frontend tests), and the e2e suite passes (19 tests). Until the posthog releases age out, they ran with `pnpm_config_minimum_release_age=0`. Without it, pnpm 12's pre-run install (`verifyDepsBeforeRun`) prints the policy error and a warning before each `pnpm run`, but the script still runs. Updated AGENTS.md, README.md, frontend/README.md, docs/infra-notes.md and docs/stack.md (which still said Node 24 and Corepack). docs/onboarding.html doesn't mention the pnpm version, so it's unchanged.
