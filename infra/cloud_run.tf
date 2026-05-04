resource "google_cloud_run_v2_service" "api" {
  name     = var.service_name
  location = var.region
  ingress  = "INGRESS_TRAFFIC_ALL"

  template {
    service_account = google_service_account.runtime.email
    timeout         = "300s"
    max_instance_request_concurrency = 80

    scaling {
      min_instance_count = 0
      max_instance_count = 4
    }

    containers {
      # Image is owned by the deploy workflow (`gcloud run services update --image=...`),
      # not by Terraform. Without ignore_changes, every `terraform apply` would
      # roll the service back to whatever image was current at the last plan.
      image = "europe-west1-docker.pkg.dev/${var.project_id}/cloud-run-source-deploy/${var.service_name}:latest"

      resources {
        limits = {
          cpu    = "1"
          memory = "512Mi"
        }
        cpu_idle          = false # equivalent to --cpu-boost / always-allocated CPU
        startup_cpu_boost = true
      }

      env {
        name  = "LOG_LEVEL"
        value = "info"
      }
      env {
        name  = "CORS_ORIGIN"
        value = "https://freewhen.simonperryman.co.uk"
      }
      env {
        name  = "COOKIE_DOMAIN"
        value = ".freewhen.simonperryman.co.uk"
      }
      env {
        name  = "FRONTEND_URL"
        value = "https://freewhen.simonperryman.co.uk"
      }
      env {
        name  = "MAGIC_LINK_FROM_EMAIL"
        value = "onboarding@resend.dev"
      }
      env {
        name  = "AUTH_EMAIL_DISABLED"
        value = "false"
      }
      env {
        name  = "GOOGLE_REDIRECT_URL"
        value = "https://api.freewhen.simonperryman.co.uk/auth/google/callback"
      }
      env {
        name  = "GOOGLE_CALENDAR_REDIRECT_URL"
        value = "https://api.freewhen.simonperryman.co.uk/auth/google/calendar/callback"
      }

      env {
        name = "DATABASE_URL"
        value_source {
          secret_key_ref {
            secret  = data.google_secret_manager_secret.database_url.secret_id
            version = "latest"
          }
        }
      }
      env {
        name = "CALENDAR_ACCESS_SECRET"
        value_source {
          secret_key_ref {
            secret  = data.google_secret_manager_secret.calendar_access.secret_id
            version = "latest"
          }
        }
      }
      env {
        name = "RESEND_API_KEY"
        value_source {
          secret_key_ref {
            secret  = data.google_secret_manager_secret.resend_api_key.secret_id
            version = "latest"
          }
        }
      }
      env {
        name = "GOOGLE_CLIENT_ID"
        value_source {
          secret_key_ref {
            secret  = data.google_secret_manager_secret.google_client_id.secret_id
            version = "latest"
          }
        }
      }
      env {
        name = "GOOGLE_CLIENT_SECRET"
        value_source {
          secret_key_ref {
            secret  = data.google_secret_manager_secret.google_client_secret.secret_id
            version = "latest"
          }
        }
      }
    }
  }

  lifecycle {
    ignore_changes = [
      template[0].containers[0].image,
      client,
      client_version,
    ]
  }
}

resource "google_cloud_run_v2_service_iam_member" "public_invoker" {
  name     = google_cloud_run_v2_service.api.name
  location = google_cloud_run_v2_service.api.location
  role     = "roles/run.invoker"
  member   = "allUsers"
}
