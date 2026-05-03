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
