resource "google_artifact_registry_repository" "cloud_run_source_deploy" {
  location      = var.region
  repository_id = "cloud-run-source-deploy"
  description   = "Auto-created by `gcloud run deploy --source .` builds."
  format        = "DOCKER"
}
