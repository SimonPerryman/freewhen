# FreeWhen infrastructure

Terraform configuration for FreeWhen's GCP project and GitHub Actions wiring. See `docs/infra-notes.md` for the first-time setup checklist and rationale.

## What lives here

- `cloud_run.tf` — backend Cloud Run service (env vars, secrets, scaling, IAM)
- `artifact_registry.tf` — container image repo
- `iam.tf` — service accounts and role bindings (including `tf-plan` / `tf-apply`)
- `github.tf` — Workload Identity Federation pool + provider, plus GitHub Actions variables/secrets and repo hardening (workflow token permissions, Dependabot alerts, `main` ruleset gated by `enable_branch_protection`)
- `secrets.tf` — Secret Manager entries
- `billing.tf` — billing budget and alert channel (£5/month)
- `main.tf`, `variables.tf` — providers and inputs
- `terraform.tfvars`, `secrets.auto.tfvars` — values (the latter is gitignored)

## Workflow

```bash
make tf-plan      # always run before merging infra changes
make tf-apply     # normally not needed locally — CI applies on push to main
```

The `tf-plan` CI job posts the plan as a PR comment. `terraform apply` runs in CI on merge to `main` via the `tf-apply` service account.

## Rules

- **Do not edit GCP or GitHub settings in the web console.** The next `terraform apply` will revert them. This includes Cloud Run env vars, IAM bindings, GitHub Actions variables, and secrets.
- Emergency console hotfixes must be followed by a matching HCL change before the next normal apply.
- Migrations are not managed here — they run on backend boot via `store.Migrate()`.
