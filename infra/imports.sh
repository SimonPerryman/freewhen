#!/usr/bin/env bash
# Import existing GCP + GitHub resources into Terraform state.
#
# Idempotent: re-running is a no-op once a resource is in state. Order matters
# where one resource references another (pool before provider, SA before its
# bindings). Run with `bash imports.sh` from the `infra/` directory after
# `terraform init`.
#
# Required env vars:
#   PROJECT_ID, PROJECT_NUMBER, REGION, GITHUB_OWNER, GITHUB_REPO, SERVICE_NAME

set -euo pipefail

: "${PROJECT_ID:?}"
: "${PROJECT_NUMBER:?}"
: "${REGION:?}"
: "${GITHUB_OWNER:?}"
: "${GITHUB_REPO:?}"
: "${SERVICE_NAME:?}"

tf() {
  terraform import "$@" || echo "  (already imported, skipping)"
}

# Workload Identity Federation
tf google_iam_workload_identity_pool.github \
  "projects/${PROJECT_NUMBER}/locations/global/workloadIdentityPools/github-pool"

tf google_iam_workload_identity_pool_provider.github \
  "projects/${PROJECT_NUMBER}/locations/global/workloadIdentityPools/github-pool/providers/github-provider"

# Service accounts
tf google_service_account.runtime \
  "projects/${PROJECT_ID}/serviceAccounts/freewhen-runtime@${PROJECT_ID}.iam.gserviceaccount.com"

tf google_service_account.deploy \
  "projects/${PROJECT_ID}/serviceAccounts/freewhen-deploy@${PROJECT_ID}.iam.gserviceaccount.com"

tf google_service_account.tf_plan \
  "projects/${PROJECT_ID}/serviceAccounts/freewhen-tf-plan@${PROJECT_ID}.iam.gserviceaccount.com"

# Project-level IAM bindings (deploy SA)
for role in run.admin iam.serviceAccountUser secretmanager.secretAccessor \
            artifactregistry.writer cloudbuild.builds.editor storage.objectAdmin; do
  tf "google_project_iam_member.deploy_roles[\"roles/${role}\"]" \
    "${PROJECT_ID} roles/${role} serviceAccount:freewhen-deploy@${PROJECT_ID}.iam.gserviceaccount.com"
done

# Project-level IAM bindings (tf-plan SA)
tf 'google_project_iam_member.tf_plan_roles["roles/viewer"]' \
  "${PROJECT_ID} roles/viewer serviceAccount:freewhen-tf-plan@${PROJECT_ID}.iam.gserviceaccount.com"

# Service-account-level bindings (WIF + actAs)
tf google_service_account_iam_member.deploy_actas_runtime \
  "projects/${PROJECT_ID}/serviceAccounts/freewhen-runtime@${PROJECT_ID}.iam.gserviceaccount.com roles/iam.serviceAccountUser serviceAccount:freewhen-deploy@${PROJECT_ID}.iam.gserviceaccount.com"

tf google_service_account_iam_member.deploy_wif_binding \
  "projects/${PROJECT_ID}/serviceAccounts/freewhen-deploy@${PROJECT_ID}.iam.gserviceaccount.com roles/iam.workloadIdentityUser principalSet://iam.googleapis.com/projects/${PROJECT_NUMBER}/locations/global/workloadIdentityPools/github-pool/attribute.repository/${GITHUB_OWNER}/${GITHUB_REPO}"

tf google_service_account_iam_member.tf_plan_wif_binding \
  "projects/${PROJECT_ID}/serviceAccounts/freewhen-tf-plan@${PROJECT_ID}.iam.gserviceaccount.com roles/iam.workloadIdentityUser principalSet://iam.googleapis.com/projects/${PROJECT_NUMBER}/locations/global/workloadIdentityPools/github-pool/attribute.repository/${GITHUB_OWNER}/${GITHUB_REPO}"

# State bucket admin (tf-plan SA — needs write for state lock objects)
tf google_storage_bucket_iam_member.tf_plan_state_admin \
  "freewhen-tfstate roles/storage.objectAdmin serviceAccount:freewhen-tf-plan@${PROJECT_ID}.iam.gserviceaccount.com"

# Artifact Registry
tf google_artifact_registry_repository.cloud_run_source_deploy \
  "projects/${PROJECT_ID}/locations/${REGION}/repositories/cloud-run-source-deploy"

# Cloud Run service + public invoker
tf google_cloud_run_v2_service.api \
  "projects/${PROJECT_ID}/locations/${REGION}/services/${SERVICE_NAME}"

tf google_cloud_run_v2_service_iam_member.public_invoker \
  "projects/${PROJECT_ID}/locations/${REGION}/services/${SERVICE_NAME} roles/run.invoker allUsers"

# GitHub Actions variables + secrets
for var in GCP_SERVICE_NAME GCP_REGION GCP_MEMORY GCP_MAX_INSTANCES; do
  tf "github_actions_variable.ci[\"${var}\"]" "${GITHUB_REPO}:${var}"
done

for sec in gcp_wif_provider gcp_deploy_sa gcp_tf_plan_sa; do
  upper=$(echo "$sec" | tr '[:lower:]' '[:upper:]')
  tf "github_actions_secret.${sec}" "${GITHUB_REPO}/${upper}"
done

echo
echo "Imports done. Run 'terraform plan' — expect 'No changes.'"
echo "Any diff means HCL doesn't match reality; fix the HCL, not the console."
