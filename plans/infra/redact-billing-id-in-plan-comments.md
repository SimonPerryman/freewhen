# Redact the billing account ID from Terraform plan comments

Status: done

## Goal

The `tf-plan` CI job posts the plan output as a PR comment. Marking `billing_account_id` as
`sensitive` hides the variable but not the `google_billing_budget.monthly` resource ID, which
embeds it (`billingAccounts/<id>/budgets/...`). Once the repo is public those comments are too, so
keep the ID out of them. It isn't a credential; this is tidiness.

## Plan

1. Delete the old plan comments that contain the ID.
2. In `ci.yml`, rewrite `billingAccounts/<id>` to `billingAccounts/REDACTED` in `plan.txt` before
   it's posted.

## Log

- 2026-11-24 09:10 — Created plan. Deleted the 15 `github-actions[bot]` plan comments (PRs #12–#39)
  containing the ID; re-checked issue/PR comments, PR bodies and issues: none left. The ID is still
  in git history (a plan log line added in #12, removed in #70); left as is since it isn't a secret.
- 2026-11-24 09:12 — Added the `sed` redaction to the `tf-plan` step. Workflow-only change, so
  `make lint`/`make test` don't cover it; apply/deploy logs already mask it as a GitHub secret.
