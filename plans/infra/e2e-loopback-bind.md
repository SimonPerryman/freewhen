# E2E servers bind to loopback only

Status: done

## Goal

Running the Playwright e2e suite on Windows pops a Windows Defender Firewall
prompt every time. The e2e backend (`go run ./cmd/e2e-server`) listened on all
interfaces (`:8090`), and because `go run` builds a fresh binary in a temp dir
on each run, an "Allow" never persists. Nothing outside the machine needs to
reach the e2e servers, so bind them to loopback.

## Plan

- `backend/cmd/e2e-server/main.go`: listen on `127.0.0.1:$PORT` instead of `:$PORT`.
- `frontend/playwright.config.ts`: start `next dev` with `--hostname localhost`
  (loopback-only, matches the URLs the tests load).
- URLs stay `http://localhost:...` so cookies/CORS origins are unchanged;
  Node and Chromium both fall back from `::1` to `127.0.0.1` when resolving
  `localhost` (verified with Node 24 `fetch` and `http.get`).
- Production `cmd/server` is untouched — Cloud Run requires listening on all
  interfaces.

## Log

- 2026-11-12 15:40 — Created plan; e2e-server and Playwright next dev bound to 127.0.0.1.
- 2026-11-12 16:20 — Switched next dev to `--hostname localhost`: the e2e-failures
  investigation (#88) found a 127.0.0.1 bind with `localhost` test URLs slowed
  startup enough that create-calendar / magic-link lost typed input.
