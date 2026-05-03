# Secret Manager secrets are read by reference, never managed by TF — values
# stay edited via `gcloud secrets versions add` to keep plaintext out of state.

data "google_secret_manager_secret" "database_url" {
  secret_id = "freewhen-database-url"
}

data "google_secret_manager_secret" "calendar_access" {
  secret_id = "freewhen-calendar-secret"
}

data "google_secret_manager_secret" "resend_api_key" {
  secret_id = "freewhen-resend-api-key"
}

data "google_secret_manager_secret" "google_client_id" {
  secret_id = "freewhen-google-client-id"
}

data "google_secret_manager_secret" "google_client_secret" {
  secret_id = "freewhen-google-client-secret"
}
