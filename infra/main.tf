terraform {
  required_version = ">= 1.7.0"

  required_providers {
    google = {
      source  = "hashicorp/google"
      version = "~> 8.12"
    }
    github = {
      source = "integrations/github"
      # 6.13 floor: standalone github_repository_vulnerability_alerts and the
      # optional ruleset allowed_merge_methods used in github.tf.
      version = "~> 6.13"
    }
  }

  backend "gcs" {
    bucket = "freewhen-tfstate"
    prefix = "freewhen"
  }
}

provider "google" {
  project = var.project_id
  region  = var.region

  # billingbudgets.googleapis.com (and a handful of other "user project" APIs)
  # require the x-goog-user-project header. Without these, calls fall back to
  # GCP's default consumer project which doesn't have the API enabled.
  user_project_override = true
  billing_project       = var.project_id
}

provider "github" {
  owner = var.github_owner
  token = var.github_token
}
