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
  description = "GitHub PAT with Actions/Secrets/Variables read+write. Set via TF_VAR_github_token env var; never commit to tfvars."
  type        = string
  sensitive   = true
}
