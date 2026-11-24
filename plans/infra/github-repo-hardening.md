# GitHub repo hardening (post-public)

Status: done

## Goal

The repo is about to go public. Move the GitHub hardening that
`plans/infra/public-readiness.md` listed as manual settings into Terraform
where the `integrations/github` provider can manage it, without importing the
`github_repository` resource (which would fight the private → public
visibility flip). List what the provider can't manage as manual
post-public steps.

## Plan

1. **Provider pin.** Bump `integrations/github` from `~> 6.0` to `~> 6.13`.
   The lock file is gitignored, so CI already resolves 6.13.x. The pin just
   makes the floor explicit: `github_repository_vulnerability_alerts`
   (standalone) and an optional `allowed_merge_methods` on ruleset
   `pull_request` (optional since 6.10.1) are only checked against 6.13.0.
2. **Workflow token defaults** (`github_workflow_repository_permissions`):
   `default_workflow_permissions = "read"`,
   `can_approve_pull_request_reviews = false`. Works on private repos, so not
   gated. Workflows already declare `permissions: contents: read` at the top
   level, so this is defence in depth for any workflow added later.
3. **Dependabot alerts** (`github_repository_vulnerability_alerts`): a
   standalone resource, so no need to manage `github_repository`. Works on
   private repos, not gated. Dependabot *security updates* (auto-PRs) are left
   to the owner (a manual toggle, or `github_repository_dependabot_security_updates` later).
4. **Branch protection on `main`** (`github_repository_ruleset`), gated by a
   new `enable_branch_protection` variable (default `false`, `count`).
   GitHub returns 403 for rulesets/branch protection on free private repos,
   so the apply only works after going public. Rules:
   - Require a PR, 0 approvals (solo dev), squash only.
   - Required status checks `changes` and `ci` from `ci.yml`, pinned to the
     GitHub Actions app (integration id 15368) so a commit status posted by
     something else can't satisfy them. `e2e` and `tf-plan` are conditional
     (path filters / fork skip), so they're not required. `changes` is
     required because if it fails, `e2e`/`tf-plan` are skipped and would
     otherwise let the PR through.
   - Non-strict (branch need not be up to date with main). Squash-merging
     parallel PRs would otherwise force a rebase + CI rerun each time.
   - Block force pushes (`non_fast_forward`) and deletion.
   - Bypass: repository Admin role (actor id 5), `always`, so the owner is
     never locked out.
5. **Can't manage without `github_repository` / not in provider → manual**:
   - Fork PR workflow approval ("Require approval for all outside
     collaborators"). The provider has no resource for
     `/actions/permissions/fork-pr-contributor-approval`.
   - Secret scanning + push protection. Only exposed via
     `github_repository.security_and_analysis`; not importing that resource.
6. **PAT scope.** `TF_GITHUB_TOKEN` (and the local `TF_VAR_github_token`)
   needs **Administration: Read and write** on the repo for the new
   resources. Update the variable description and docs.
7. Docs: "Going public" checklist in `docs/infra-notes.md`; `infra/README.md`
   file list; one line in `docs/onboarding.html` infra section.
8. Validate: `terraform fmt -check`, `terraform validate`, try
   `make tf-plan`. No apply.

### Turning branch protection on

After flipping the repo public: set `enable_branch_protection = true` in
`infra/terraform.tfvars` in a PR. The merge to main runs `tf-apply` in CI and
creates the ruleset.

## Log

- 2026-11-23 20:50 — Created plan. Checked provider 6.13.0 schema:
  `github_repository_ruleset`, `github_workflow_repository_permissions`,
  `github_repository_vulnerability_alerts` all available; no resource for
  fork PR approval; secret scanning only on `github_repository`.
- 2026-11-23 21:05 — Added the three resources + `enable_branch_protection`
  to `infra/github.tf` / `variables.tf`, bumped the github pin to `~> 6.13`,
  widened the `github_token` description to include Administration.
  `terraform fmt -check` clean, `validate` passes (Terraform 1.9.8, provider
  6.13.0, `init -backend=false`). `make tf-plan` fails locally at backend init
  (no GCP application-default credentials), as expected; no apply run.
- 2026-11-23 21:10 — Docs: "Going public" checklist in `docs/infra-notes.md`,
  `infra/README.md` file list, onboarding hosting bullet. `make test` passes;
  `make lint` only fails on gofmt/oxfmt CRLF noise in untouched files
  (`gofmt -l` on the `HEAD` blobs is clean); oxlint, typecheck and lint-pii pass.
- 2026-11-23 21:12 — Done. Remaining: owner adds Administration to the PAT,
  flips visibility, sets `enable_branch_protection = true`, does the manual
  steps in the checklist.
- 2026-11-24 09:50 — Repo is public; set `enable_branch_protection = true` in
  `infra/terraform.tfvars`. Merging this runs `tf-apply`, which creates the
  ruleset. Allow auto-merge is a repo setting outside Terraform (no
  `github_repository` import), so it's set once with `gh` after the ruleset exists.
