# Node 26 LTS

Status: done

## Goal

Move the frontend from Node 24 to Node 26, which became LTS in October 2026. The owner's machine already defaults to 26, and the repo's 24 pin meant every local command needed `fnm exec --using=24`.

## Plan

- `frontend/.nvmrc` → `26`. CI (`ci.yml`) and deploy (`deploy.yml`) read it through `node-version-file`, so the workflows need no edits.
- `package.json`: `engines.node` → `>=26 <27`, `@types/node` → `^26`. Vercel picks the runtime from `engines`; its build tooling lists `nodejs26.x`.
- **Web Storage clash.** Node 25+ defines a global `localStorage` that returns `undefined` unless `--localstorage-file` is set. It shadows happy-dom's storage, and 82 component tests fail with "Cannot read properties of undefined". Turn Node's version off in the vitest workers (`--no-experimental-webstorage`).
- **Corepack.** Node 25+ no longer ships Corepack. CI already installs pnpm with `pnpm/action-setup`. Local setup becomes a global `pnpm`, which switches itself to the `packageManager` version. Update README, `frontend/README.md`, `docs/infra-notes.md` and AGENTS.md.
- Run `make lint`, `make test` and the e2e suite on Node 26.

## Log

- 2026-11-24 16:00 — Created plan
- 2026-11-24 16:20 — Bumped `.nvmrc`, `engines` and `@types/node`, and updated the setup docs. Added `--no-experimental-webstorage` to the vitest workers, which fixed the 82 localStorage failures. On Node 26: `make lint` and `make test` pass (528 frontend tests), and the e2e suite passes (19 tests). Plain `pnpm` switches itself to the pinned 10.34.9, and Playwright can now start its own servers, which a Corepack/pnpm 12 clash had blocked on Windows. Vercel's build tooling lists `nodejs26.x`. The first production deploy after merge is the real check.
