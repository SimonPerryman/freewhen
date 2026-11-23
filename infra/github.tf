locals {
  github_actions_variables = {
    GCP_SERVICE_NAME = var.service_name
    GCP_REGION       = var.region
  }
}

resource "github_actions_variable" "ci" {
  for_each      = local.github_actions_variables
  repository    = var.github_repo
  variable_name = each.key
  value         = each.value
}

# GitHub Actions secrets: WIF provider name + the SA emails CI assumes.
# The WIF provider name isn't actually sensitive — it's a public OIDC
# trust binding identifier — so it lives in tfvars. SA emails are
# computed from the resources TF manages.

# GitHub doesn't return secret values via the API, so TF can't verify
# them on plan — without ignore_changes, every plan would re-write the
# value. The values themselves are deterministic (TF-managed SA emails
# or the public WIF provider name), so ignoring is safe.

resource "github_actions_secret" "gcp_wif_provider" {
  repository  = var.github_repo
  secret_name = "GCP_WIF_PROVIDER"
  value       = var.gcp_wif_provider
  lifecycle {
    ignore_changes = [value]
  }
}

resource "github_actions_secret" "gcp_deploy_sa" {
  repository  = var.github_repo
  secret_name = "GCP_DEPLOY_SA"
  value       = google_service_account.deploy.email
  lifecycle {
    ignore_changes = [value]
  }
}

resource "github_actions_secret" "gcp_tf_plan_sa" {
  repository  = var.github_repo
  secret_name = "GCP_TF_PLAN_SA"
  value       = google_service_account.tf_plan.email
  lifecycle {
    ignore_changes = [value]
  }
}

resource "github_actions_secret" "gcp_tf_apply_sa" {
  repository  = var.github_repo
  secret_name = "GCP_TF_APPLY_SA"
  value       = google_service_account.tf_apply.email
  lifecycle {
    ignore_changes = [value]
  }
}

# Billing budget inputs — needed by CI so plan/apply can reconcile billing.tf.
# Seeded from secrets.auto.tfvars (or TF_VAR_*) on the first local apply, then
# read from GitHub secrets in CI.
resource "github_actions_secret" "billing_account_id" {
  repository  = var.github_repo
  secret_name = "GCP_BILLING_ACCOUNT_ID"
  value       = var.billing_account_id
  lifecycle {
    ignore_changes = [value]
  }
}

resource "github_actions_secret" "alert_email" {
  repository  = var.github_repo
  secret_name = "ALERT_EMAIL"
  value       = var.alert_email
  lifecycle {
    ignore_changes = [value]
  }
}

# ---------------------------------------------------------------------------
# Repository hardening
#
# Deliberately NOT managing `github_repository` itself: importing it would make
# Terraform own `visibility` and fight the private -> public flip. Everything
# below is a standalone resource that touches one setting.
#
# The TF_GITHUB_TOKEN PAT needs "Administration: Read and write" on the repo
# for these, on top of Actions/Secrets/Variables.
# ---------------------------------------------------------------------------

# Default GITHUB_TOKEN permissions for workflows that don't declare their own
# `permissions:` block. ci.yml / deploy.yml already set `contents: read` at the
# top level; this covers any workflow added later. Works on private repos.
resource "github_workflow_repository_permissions" "repo" {
  repository                       = var.github_repo
  default_workflow_permissions     = "read"
  can_approve_pull_request_reviews = false
}

# Dependabot alerts. Standalone resource, so no need to import
# github_repository. Dependabot security-update PRs are left as a manual
# toggle. Works on private repos.
resource "github_repository_vulnerability_alerts" "repo" {
  repository = var.github_repo
  enabled    = true
}

locals {
  # The GitHub Actions app. Pinning required checks to it means a commit
  # status posted by anything else (another app, a PAT) can't satisfy them.
  github_actions_app_id = 15368
}

# Branch protection for main.
#
# GATED: GitHub returns 403 for rulesets / branch protection on free PRIVATE
# repos, so this can only be applied once the repo is public. After flipping
# visibility, set `enable_branch_protection = true` in terraform.tfvars and
# let CI apply it. See docs/infra-notes.md -> "Going public".
resource "github_repository_ruleset" "main" {
  count = var.enable_branch_protection ? 1 : 0

  name        = "main"
  repository  = var.github_repo
  target      = "branch"
  enforcement = "active"

  conditions {
    ref_name {
      include = ["refs/heads/main"]
      exclude = []
    }
  }

  # Repository Admin role (actor_id 5) can always bypass, so the owner is
  # never locked out (CI outage, emergency push).
  bypass_actors {
    actor_type  = "RepositoryRole"
    actor_id    = 5
    bypass_mode = "always"
  }

  rules {
    deletion         = true # block deleting main
    non_fast_forward = true # block force pushes

    pull_request {
      required_approving_review_count = 0 # solo dev: PR required, no approval
      allowed_merge_methods           = ["squash"]
    }

    # Only jobs that report on every PR. `e2e` and `tf-plan` are skipped by
    # path filters (tf-plan also on fork PRs), so they aren't required.
    # `changes` is required because if it fails, the conditional jobs are
    # skipped rather than failed, which would otherwise let the PR through.
    required_status_checks {
      # Non-strict: don't force a rebase + CI rerun every time another
      # squash merge lands on main.
      strict_required_status_checks_policy = false

      required_check {
        context        = "changes"
        integration_id = local.github_actions_app_id
      }
      required_check {
        context        = "ci"
        integration_id = local.github_actions_app_id
      }
    }
  }
}
