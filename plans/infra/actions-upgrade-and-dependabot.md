# Upgrade GitHub Actions + add Dependabot

Status: done

## Goal

The workflows were on 2024-era action majors (node20 runtimes, which GitHub
is deprecating) and referenced actions by mutable tags. Now that the repo is
public, pin every action to a full commit SHA (with a `# vX.Y.Z` comment) so a
retagged or compromised upstream release can't change what runs with our WIF
credentials and Vercel token. Add Dependabot so these pins, plus npm, Go
modules and Terraform providers, get bumped automatically instead of drifting.

## Plan

1. Resolve each action's latest release (`gh api repos/<o>/<r>/releases/latest`)
   to a commit SHA (dereferencing annotated tags) and pin it in `ci.yml` and
   `deploy.yml`.
2. Read the release notes for every major crossed and adapt inputs if needed.
3. Bump golangci-lint (`version:` in ci.yml) if the current backend config is
   clean under the newest v2.x; bump `terraform_version` if nothing in `infra/`
   depends on removed behaviour.
4. Add `.github/dependabot.yml`: npm (/frontend), gomod (/backend),
   github-actions (/), terraform (/infra); weekly; minor+patch grouped per
   ecosystem; majors ungrouped; `chore(deps)` prefix.
5. Validate the YAML, open a PR, watch CI.

| Action | Old | New |
| --- | --- | --- |
| actions/checkout | v4 | v7.0.1 |
| actions/cache | v4 | v6.1.0 |
| actions/setup-node | v4 | v7.1.0 |
| actions/setup-go | v5 | v7.1.0 |
| actions/upload-artifact | v4 | v7.0.2 |
| actions/github-script | v7 | v9.0.0 |
| dorny/paths-filter | v3 | v4.0.3 |
| golangci/golangci-lint-action | v7 | v9.3.0 |
| google-github-actions/auth | v2 | v3.0.0 |
| google-github-actions/setup-gcloud | v2 | v3.0.1 |
| hashicorp/setup-terraform | v3 | v4.0.1 |
| pnpm/action-setup | v4 | v6.1.1 |
| step-security/harden-runner | v2.22.2 | v2.23.0 |
| golangci-lint | v2.11 | v2.15 |
| Terraform CLI | 1.9.x | 1.17.x |
| vercel CLI | 65.8.0 | unchanged |

Release-note review (no input changes were needed):

- Most majors are just the node20 → node24 runtime move and/or an ESM rebuild
  (checkout, cache, setup-go, upload-artifact, paths-filter, setup-terraform,
  golangci-lint-action v9, pnpm/action-setup v5).
- setup-node v5/v6: automatic caching now only kicks in for npm; we already
  set `cache: 'pnpm'` explicitly, so no change.
- upload-artifact v7: adds opt-in unzipped uploads (`archive: false`); default
  zipped behaviour is unchanged.
- github-script v9: `require('@actions/github')` breaks and `getOctokit` is
  injected; our script only uses `github.rest` + `context`.
- auth v3: removes deprecated inputs; we only use `workload_identity_provider`
  and `service_account`, and the `project_id` output is still present.
- setup-gcloud v3: tool cache off by default (irrelevant on hosted runners).
- golangci-lint-action v8: absolute paths with `working-directory`, requires
  golangci-lint >= 2.1.
- pnpm/action-setup v6: adds pnpm 11/12 support; still honours
  `packageManager` (pnpm 10.30.3).
- golangci-lint 2.15: `golangci-lint config verify` and `golangci-lint run`
  in `backend/` are clean with the existing `.golangci.yml`.
- Terraform 1.10–1.17: upgrade notes only touch the s3/azure backends,
  `terraform test`, provisioners and Linux kernel minimums; we use the gcs
  backend and plain resources. `required_version = ">= 1.7.0"` already allows
  it. The CI `tf-plan` job runs on this PR (ci.yml is in the infra filter) and
  is the check.
- vercel CLI 65 → 66 is a major and only runs in the deploy job (not on PRs),
  so it can't be verified here. Left pinned; bump in a separate change.
  Dependabot won't track it (it's a `pnpm dlx` arg, not a manifest).

## Log

- 2026-11-24 09:47 — Created plan; resolved latest tags to SHAs, reviewed
  release notes, pinned actions, bumped golangci-lint to v2.15 and Terraform
  to 1.17.x, added dependabot.yml (with a 3-day cooldown). Validated workflows
  with `@action-validator/cli` and dependabot.yml against the SchemaStore
  schema.
- 2026-11-24 09:55 — First CI run green (changes, ci, e2e, tf-plan "no
  changes" on Terraform 1.17). Fixed the setup-go "Restore cache failed"
  warning (it looked for go.sum at the repo root) with
  `cache-dependency-path: backend/go.sum`.
- 2026-11-24 10:00 — Second CI run green with no cache warnings. Marked done
  (deploy.yml pins are only exercised after merge).
- 2026-11-24 10:54 — First Dependabot run: #117 (google provider ~> 6.0 → ~> 8.12) failed tf-plan because Dependabot PRs get no Actions secrets, and #118 duplicated #114. Closed both. tf-plan now skips Dependabot PRs, and the terraform ecosystem ignores semver-major bumps (do those as planned upgrades with a reviewed plan).
