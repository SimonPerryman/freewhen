# GCP billing budget alert

Status: done

## Goal

Get an early warning if FreeWhen's GCP spend ever leaves the free tier. Right now there's nothing watching — `infra/cloud_run.tf` caps `max_instance_count = 4` so runaway compute is bounded, but a misbehaving loop or unexpected egress could still rack up double-digit dollars a day before anyone notices. A budget alert at low thresholds is the cheapest, lowest-risk safety net.

Explicitly **not** doing the auto-disable-billing Cloud Function pattern: too much blast radius (kills Cloud Run, Secret Manager, etc. with a multi-minute lag) and too much surface area to maintain for a hobby project.

## Plan

1. **Find billing account ID.** `gcloud billing accounts list` — needed for the `billing_account` argument on `google_billing_budget`. Record it as a TF variable (don't hardcode; it's mildly sensitive and per-environment).

2. **Add `infra/billing.tf`** with:
   - `google_billing_budget` resource scoped to the project (`projects/<project_number>` filter — already have `project_number` wired up from prior infra work).
   - `amount.specified_amount` = $5 USD/month (small enough to fire fast, large enough to cover incidental free-tier overflow without nuisance alerts).
   - `threshold_rules` at 0.5, 0.9, 1.0, 1.5 of the budget — the 1.5x rule is the "something is genuinely wrong" alert.
   - `all_updates_rule.monitoring_notification_channels` pointing at an email notification channel (see step 3).
   - `display_name = "FreeWhen monthly spend"` so it's recognisable in the console.

3. **Add `google_monitoring_notification_channel`** of type `email` with `labels.email_address` = my email. Reference it from the budget. (Budgets can also email "billing admins" directly without a channel, but an explicit channel is more portable and shows up consistently in the Console.)

4. **Provider permissions check.** The existing TF service account (`tf-plan` / whatever runs `terraform apply` in CI per `plans/infra/terraform-migration.md`) needs `roles/billing.user` (or `billing.costsManager`) on the **billing account**, not the project. This is the most likely place for the apply to fail on first run — call it out and grant via console if needed (it's a one-time click on the billing account IAM page).

5. **Add billing account ID to `terraform.tfvars`** (or `variables.tf` with a sensitive default pulled from env). Confirm `terraform.tfvars` is gitignored — if not, use a `TF_VAR_billing_account_id` env var in CI instead.

6. **Plan + apply locally first**, verify the budget appears in Console → Billing → Budgets & alerts, then let CI take over on subsequent changes.

7. **Smoke-test the alert** by temporarily setting the budget to $0.01 and confirming an email lands within ~24h (GCP's data freshness for budgets is up to a day). Reset to $5 after.

8. **Document in `docs/infra-notes.md`** under a new "Cost guardrails" subsection: max_instance_count cap + budget alert + link to the budget in Console.

## Notification routing

Use the **explicit** route: a `google_monitoring_notification_channel` of type `email` with the destination address read from a TF variable (`alert_email`), so the address lives in `terraform.tfvars` (gitignored) rather than in the repo. The budget references the channel via `all_updates_rule.monitoring_notification_channels` and sets `disable_default_iam_recipients = true` so we don't *also* implicitly email every billing admin.

Rejected alternative: leaving `disable_default_iam_recipients = false` and relying on GCP's implicit "email all Billing Account Admins/Users" behaviour. Simpler, but opaque — anyone added to billing IAM later silently starts receiving alerts.

## Currency

`specified_amount.currency_code = "GBP"` — billing account is GBP. Budget amount: **£5/month**.

## Open questions

- Whether to also add a Cloud Run-specific budget filter (`services` filter on the budget) so we get a separate signal for "Cloud Run alone is spending money" vs. "everything combined". Probably overkill for v1 — start with one project-wide budget.

## Log

- 2026-05-04 — Created plan.
- 2026-05-04 — Implemented. Billing account `XXXXXX-XXXXXX-XXXXXX` confirmed. Granted `roles/billing.costsManager` to `freewhen-tf-apply` and `roles/billing.viewer` to `freewhen-tf-plan` on the billing account (out-of-band — TF can't bootstrap this without billing.admin). Enabled `billingbudgets.googleapis.com` on the project. Added `infra/billing.tf` (notification channel + budget at 0.5/0.9/1.0/1.5 thresholds, £5/month, `disable_default_iam_recipients = true`), new `billing_account_id` + `alert_email` vars (sensitive, in `secrets.auto.tfvars`), matching `GCP_BILLING_ACCOUNT_ID` / `ALERT_EMAIL` GitHub Actions secrets, and `TF_VAR_*` wiring in CI plan + apply jobs. Added `user_project_override = true` + `billing_project = var.project_id` on the google provider — billingbudgets requires the `x-goog-user-project` header. Local apply succeeded; budget visible at Console → Billing → Budgets & alerts. `terraform plan` is zero-diff. Step 7 (smoke-test by setting budget to £0.01) deferred — not worth burning a day of alert quota for a routing path that's already exercised by the channel verification email.
- 2026-05-04 — Documented under "Cost guardrails" in `docs/infra-notes.md`.
- 2026-05-04 — Local apply had drifted from main: `billing.tf`,
  `billing_account_id`/`alert_email` vars, and the matching `GCP_*` GitHub
  secrets existed in real state but were never committed. When the
  terraform-migration PR (#11) merged and ran `tf-apply` on main, it tried
  to destroy the orphaned resources and 403'd on the notification channel
  (tf-apply SA lacked `roles/monitoring.editor`). Fix in PR #12: ship the
  billing files to main + add `roles/monitoring.editor` to the tf-apply
  SA's project roles. Local plan zero-diff after the role grant.
