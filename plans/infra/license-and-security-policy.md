# License and Security Policy

Status: done

## Goal

Prepare the repo to go public: license it under the MIT License and publish a security policy that
tells reporters how to disclose vulnerabilities privately.

## Plan

1. `LICENSE` at the repo root: the MIT License text from GitHub's licenses API (`gh api licenses/mit`),
   with year and copyright holder filled in.
2. `"license": "MIT"` (SPDX id) in `frontend/package.json`, the only package.json in the repo.
3. `README.md`: short License section.
4. `SECURITY.md`: GitHub private vulnerability reporting, best-effort response, scope and
   out-of-scope lists, no testing against other users' data.
5. Third-party attribution check: dependencies ship their own notices via npm / Go modules, so no
   NOTICE file is needed. Record the reasoning in the PR.
6. `docs/onboarding.html`: one line under "Repo principles".
7. `make lint`, `make test`, open a PR.

## Log

- 2026-11-23 20:51 — Created plan
- 2026-11-23 20:55 — Fetched license text via `gh api` from polyform-licenses tag `1.0.0`; added
  LICENSE (body byte-identical to source, `Required Notice:` line above), SECURITY.md, README
  License section, package.json `license`, onboarding "Repo principles" line
- 2026-11-23 21:05 — Attribution check: no NOTICE needed (npm / Go module deps carry their own
  notices; sharp's LGPL libvips and caniuse-lite's CC-BY-4.0 are satisfied by the installed
  packages). `make lint` gofmt/oxfmt failures are CRLF-checkout noise (LF HEAD blobs pass);
  golangci-lint otherwise, oxlint, typecheck, lint-pii and `make test` pass
- 2026-11-23 22:10 — Owner switched from PolyForm Noncommercial to MIT: the commercial restriction
  wasn't protecting anything worth the friction (unrecognised license, no GitHub badge). LICENSE now
  the MIT text from `gh api licenses/mit`; README, package.json and onboarding updated to match.
