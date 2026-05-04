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
