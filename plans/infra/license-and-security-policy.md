# License and Security Policy

Status: done

## Goal

Prepare the repo to go public: license it under the PolyForm Noncommercial License 1.0.0
(source-available, no commercial use without permission) and publish a security policy that tells
reporters how to disclose vulnerabilities privately.

## Plan

1. `LICENSE` at the repo root: the unmodified PolyForm Noncommercial 1.0.0 text, fetched from
   `polyformproject/polyform-licenses` (tag `1.0.0`), preceded by the licensor's
   `Required Notice:` line. The license's Notices section asks for plain-text lines beginning with
   `Required Notice:`, so the line goes in as plain text, not markdown-formatted.
2. `"license": "PolyForm-Noncommercial-1.0.0"` (SPDX id) in `frontend/package.json`, the only
   package.json in the repo.
3. `README.md`: short License section (source-available, noncommercial use free, commercial use by
   permission via GitHub, not an OSI open-source license).
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
