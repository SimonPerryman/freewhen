# FreeWhen frontend

Next.js App Router app for FreeWhen. See the root `README.md` for the full stack overview and `AGENTS.md` for project conventions.

## Run

```bash
pnpm install
pnpm dev          # http://localhost:3000
```

Or from the repo root: `make dev-frontend`.

The app expects the backend on `:8080` — start it with `make dev-backend` in another shell.

## Tooling

- **Node 26 LTS** via `.nvmrc`; pnpm pinned in `package.json` (a global pnpm switches to that version itself)
- **oxlint** for linting, **oxfmt** for formatting
- **vitest** with jsdom for tests; `@/` alias maps to `src/`
- Optional **bun** as a speedup (e.g. `bunx vitest`) — not a runtime dependency

## Commands

```bash
pnpm lint                              # oxlint
pnpm test                              # vitest
pnpm exec vitest run path/to/file      # single file
pnpm build                             # production build
```

## Conventions

- App Router under `src/app/`
- Components: `src/components/<Name>/<Name>.tsx` — folder-per-component, no barrel files
- Shared utilities and API client: `src/lib/` (flat)
- Use `log.info`/`log.warn`/`log.error` from `src/lib/log.ts` instead of `console.*`
- Mobile first — design at 375px, enhance with min-width breakpoints
- For reversible actions, use the `Toast` primitive with an `Undo` action — never `window.confirm`
- Error boundary lives in `src/app/error.tsx`; not-found in `src/app/not-found.tsx`

## Deployment

Auto-deploys to Vercel on push. See `docs/infra-notes.md`.
