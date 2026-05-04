# Project-wide budget alert. Cost guardrail to catch a runaway Cloud Run loop
# or unexpected egress before it racks up double-digit dollars. `max_instance_count`
# in cloud_run.tf is the hard cap; this is the early-warning system.
#
# IAM prerequisite (one-time, granted out-of-band — TF can't bootstrap its own
# billing-account access without billing.admin, which is too broad):
#   - tf-apply SA: roles/billing.costsManager on the billing account
#   - tf-plan  SA: roles/billing.viewer       on the billing account

resource "google_monitoring_notification_channel" "billing_email" {
  display_name = "FreeWhen billing alerts"
  type         = "email"
  labels = {
    email_address = var.alert_email
  }
}

resource "google_billing_budget" "monthly" {
  billing_account = var.billing_account_id
  display_name    = "FreeWhen monthly spend"

  budget_filter {
    projects = ["projects/${var.project_number}"]
  }

  amount {
    specified_amount {
      currency_code = "GBP"
      units         = "5"
    }
  }

  # 0.5 = heads-up, 0.9 = approaching cap, 1.0 = over budget,
  # 1.5 = something is genuinely wrong.
  threshold_rules {
    threshold_percent = 0.5
  }
  threshold_rules {
    threshold_percent = 0.9
  }
  threshold_rules {
    threshold_percent = 1.0
  }
  threshold_rules {
    threshold_percent = 1.5
  }

  all_updates_rule {
    monitoring_notification_channels = [
      google_monitoring_notification_channel.billing_email.id,
    ]
    # Don't also implicitly email every billing admin — keep the routing
    # explicit so adding someone to billing IAM later doesn't silently
    # subscribe them to alerts.
    disable_default_iam_recipients = true
  }
}
