locals {
  github_actions_variables = {
    GCP_SERVICE_NAME  = var.service_name
    GCP_REGION        = var.region
    GCP_MEMORY        = "512Mi"
    GCP_MAX_INSTANCES = "4"
  }
}

resource "github_actions_variable" "ci" {
  for_each      = local.github_actions_variables
  repository    = var.github_repo
  variable_name = each.key
  value         = each.value
}

# GitHub Actions secrets: WIF provider name + the SA emails CI assumes.
# Values are read from environment at apply time so they don't sit in tfvars.
# Set TF_VAR_gcp_wif_provider, etc., or pass --var on the command line.

variable "gcp_wif_provider" {
  description = "Full WIF provider resource name. Stored as the GCP_WIF_PROVIDER repo secret."
  type        = string
  sensitive   = true
}

resource "github_actions_secret" "gcp_wif_provider" {
  repository      = var.github_repo
  secret_name     = "GCP_WIF_PROVIDER"
  plaintext_value = var.gcp_wif_provider
}

resource "github_actions_secret" "gcp_deploy_sa" {
  repository      = var.github_repo
  secret_name     = "GCP_DEPLOY_SA"
  plaintext_value = google_service_account.deploy.email
}

resource "github_actions_secret" "gcp_tf_plan_sa" {
  repository      = var.github_repo
  secret_name     = "GCP_TF_PLAN_SA"
  plaintext_value = google_service_account.tf_plan.email
}
