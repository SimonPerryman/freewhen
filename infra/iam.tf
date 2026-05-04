locals {
  # Roles match what's currently bound on the deploy SA in production.
  # `storage.admin` and `storage.objectAdmin` are both present (admin
  # implies objectAdmin, but live policy has both as separate bindings —
  # likely added during deploy IAM debugging). Keep both to avoid drift.
  deploy_sa_roles = [
    "roles/run.admin",
    "roles/iam.serviceAccountUser",
    "roles/secretmanager.secretAccessor",
    "roles/artifactregistry.writer",
    "roles/cloudbuild.editor",
    "roles/storage.admin",
    "roles/storage.objectAdmin",
  ]

  tf_plan_sa_roles = [
    "roles/viewer",
  ]
}

# Workload Identity Federation: GitHub Actions OIDC -> GCP service accounts.

resource "google_iam_workload_identity_pool" "github" {
  workload_identity_pool_id = "github"
  display_name              = "GitHub Actions Pool"
}

resource "google_iam_workload_identity_pool_provider" "github" {
  workload_identity_pool_id          = google_iam_workload_identity_pool.github.workload_identity_pool_id
  workload_identity_pool_provider_id = "freewhen"
  display_name                       = "My GitHub repo Provider"

  attribute_mapping = {
    "google.subject"             = "assertion.sub"
    "attribute.repository"       = "assertion.repository"
    "attribute.repository_owner" = "assertion.repository_owner"
    "attribute.actor"            = "assertion.actor"
  }

  attribute_condition = "assertion.repository == '${var.github_owner}/${var.github_repo}'"

  oidc {
    issuer_uri = "https://token.actions.githubusercontent.com"
  }
}

# Cloud Run runs as the default Compute Engine service account. No
# dedicated runtime SA — hardcoded here rather than looked up via a
# data source because that requires the Compute Engine API enabled,
# which we don't otherwise use.
locals {
  runtime_sa_email = "${var.project_number}-compute@developer.gserviceaccount.com"
  runtime_sa_id    = "projects/${var.project_id}/serviceAccounts/${local.runtime_sa_email}"
}

# Deploy service account: CI assumes this via WIF to push images and deploy.

resource "google_service_account" "deploy" {
  account_id   = "freewhen-deploy"
  display_name = "FreeWhen Github Actions deployer"
}

resource "google_project_iam_member" "deploy_roles" {
  for_each = toset(local.deploy_sa_roles)
  project  = var.project_id
  role     = each.value
  member   = "serviceAccount:${google_service_account.deploy.email}"
}

# Deploy SA needs to actAs the runtime SA (compute default) to deploy revisions.
resource "google_service_account_iam_member" "deploy_actas_runtime" {
  service_account_id = local.runtime_sa_id
  role               = "roles/iam.serviceAccountUser"
  member             = "serviceAccount:${google_service_account.deploy.email}"
}

resource "google_service_account_iam_member" "deploy_wif_binding" {
  service_account_id = google_service_account.deploy.name
  role               = "roles/iam.workloadIdentityUser"
  member             = "principalSet://iam.googleapis.com/projects/${var.project_number}/locations/global/workloadIdentityPools/${google_iam_workload_identity_pool.github.workload_identity_pool_id}/attribute.repository/${var.github_owner}/${var.github_repo}"
}

# Read-only service account for `terraform plan` in CI. Cannot apply.

resource "google_service_account" "tf_plan" {
  account_id   = "freewhen-tf-plan"
  display_name = "FreeWhen Terraform plan (read-only)"
}

resource "google_project_iam_member" "tf_plan_roles" {
  for_each = toset(local.tf_plan_sa_roles)
  project  = var.project_id
  role     = each.value
  member   = "serviceAccount:${google_service_account.tf_plan.email}"
}

# objectAdmin (not objectViewer) because the GCS backend writes a lock file
# even on `terraform init` / `plan`. The read-only safety property is
# preserved by project-level `roles/viewer` — this SA can manage state
# objects in its own bucket but cannot mutate any actual GCP resources.
resource "google_storage_bucket_iam_member" "tf_plan_state_admin" {
  bucket = "freewhen-tfstate"
  role   = "roles/storage.objectAdmin"
  member = "serviceAccount:${google_service_account.tf_plan.email}"
}

resource "google_service_account_iam_member" "tf_plan_wif_binding" {
  service_account_id = google_service_account.tf_plan.name
  role               = "roles/iam.workloadIdentityUser"
  member             = "principalSet://iam.googleapis.com/projects/${var.project_number}/locations/global/workloadIdentityPools/${google_iam_workload_identity_pool.github.workload_identity_pool_id}/attribute.repository/${var.github_owner}/${var.github_repo}"
}
