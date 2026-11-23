variable "project_id" {
  description = "GCP project ID."
  type        = string
}

variable "project_number" {
  description = "GCP project number. Used for WIF provider attribute conditions and IAM principal subjects."
  type        = string
}

variable "region" {
  description = "GCP region for Cloud Run, Artifact Registry, and the state bucket."
  type        = string
}

variable "github_owner" {
  description = "GitHub user or org that owns the repo."
  type        = string
}

variable "github_repo" {
  description = "GitHub repo name (without owner)."
  type        = string
}

variable "service_name" {
  description = "Cloud Run service name."
  type        = string
}

variable "gcp_wif_provider" {
  description = "Full WIF provider resource name. Public — it's just an OIDC trust binding identifier, not a credential."
  type        = string
}

variable "github_token" {
  description = "GitHub fine-grained PAT with Actions/Secrets/Variables/Administration read+write on the repo. Set via TF_VAR_github_token env var; never commit to tfvars."
  type        = string
  sensitive   = true
}

variable "enable_branch_protection" {
  description = "Create the main-branch ruleset. Keep false while the repo is private: GitHub returns 403 for rulesets on free private repos. Set true after making the repo public."
  type        = bool
  default     = false
}

variable "billing_account_id" {
  description = "GCP billing account ID (e.g. AAAAAA-BBBBBB-CCCCCC) for the budget alert. Lives in secrets.auto.tfvars (gitignored) or TF_VAR_billing_account_id; mildly sensitive."
  type        = string
  sensitive   = true
}

variable "alert_email" {
  description = "Email address that receives billing budget alerts. Lives in secrets.auto.tfvars (gitignored) or TF_VAR_alert_email."
  type        = string
  sensitive   = true
}
