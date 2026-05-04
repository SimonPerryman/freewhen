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

  # Roles for the tf-apply SA. Broader than tf-plan because it must mutate
  # everything TF manages: project-level IAM bindings, service accounts, the
  # WIF pool/provider, Cloud Run, Artifact Registry, and bucket-scoped IAM
  # on the state bucket. `roles/viewer` is included so refresh paths that
  # need read access outside an admin role (e.g. listing) keep working.
  # GitHub Actions variables/secrets are managed via PAT, not this SA.
  tf_apply_sa_roles = [
    "roles/viewer",
    "roles/iam.workloadIdentityPoolAdmin",
    "roles/iam.serviceAccountAdmin",
    "roles/resourcemanager.projectIamAdmin",
    "roles/run.admin",
    "roles/artifactregistry.admin",
    "roles/secretmanager.viewer",
    # Needed by `google_monitoring_notification_channel` (billing alert
    # email channel in `billing.tf`). The first CI tf-apply on main hit a
    # 403 trying to delete the channel during a botched destroy plan.
    "roles/monitoring.editor",
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

# `storage.admin` (bucket-scoped) — needed because `objectAdmin` doesn't
# grant `storage.buckets.getIamPolicy`, which TF requires to refresh the
# state of any `google_storage_bucket_iam_member`. The read-only safety
# property is preserved by project-level `roles/viewer` — this SA can
# manage this one bucket but cannot mutate any actual GCP resources.
resource "google_storage_bucket_iam_member" "tf_plan_state_admin" {
  bucket = "freewhen-tfstate"
  role   = "roles/storage.admin"
  member = "serviceAccount:${google_service_account.tf_plan.email}"
}

resource "google_service_account_iam_member" "tf_plan_wif_binding" {
  service_account_id = google_service_account.tf_plan.name
  role               = "roles/iam.workloadIdentityUser"
  member             = "principalSet://iam.googleapis.com/projects/${var.project_number}/locations/global/workloadIdentityPools/${google_iam_workload_identity_pool.github.workload_identity_pool_id}/attribute.repository/${var.github_owner}/${var.github_repo}"
}

# Service account for `terraform apply` in CI on pushes to main. Branch
# scoping is enforced at the workflow level (`if: github.ref ==
# 'refs/heads/main'`), matching the deploy SA pattern — same trust
# boundary, same review surface.

resource "google_service_account" "tf_apply" {
  account_id   = "freewhen-tf-apply"
  display_name = "FreeWhen Terraform apply"
}

resource "google_project_iam_member" "tf_apply_roles" {
  for_each = toset(local.tf_apply_sa_roles)
  project  = var.project_id
  role     = each.value
  member   = "serviceAccount:${google_service_account.tf_apply.email}"
}

# Setting `template.service_account` on the Cloud Run service requires
# `iam.serviceAccountUser` on the runtime SA being assigned.
resource "google_service_account_iam_member" "tf_apply_actas_runtime" {
  service_account_id = local.runtime_sa_id
  role               = "roles/iam.serviceAccountUser"
  member             = "serviceAccount:${google_service_account.tf_apply.email}"
}

# Bucket-scoped admin so apply can read+write state, take the GCS lock,
# and manage `google_storage_bucket_iam_member` bindings on the state bucket.
resource "google_storage_bucket_iam_member" "tf_apply_state_admin" {
  bucket = "freewhen-tfstate"
  role   = "roles/storage.admin"
  member = "serviceAccount:${google_service_account.tf_apply.email}"
}

resource "google_service_account_iam_member" "tf_apply_wif_binding" {
  service_account_id = google_service_account.tf_apply.name
  role               = "roles/iam.workloadIdentityUser"
  member             = "principalSet://iam.googleapis.com/projects/${var.project_number}/locations/global/workloadIdentityPools/${google_iam_workload_identity_pool.github.workload_identity_pool_id}/attribute.repository/${var.github_owner}/${var.github_repo}"
}
