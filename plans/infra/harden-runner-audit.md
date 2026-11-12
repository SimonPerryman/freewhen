# Harden-Runner (audit mode) on the deploy workflow

Status: done

## Goal

Defence in depth after a GitHub Actions secret-stealer campaign. `deploy.yml`
holds GCP Workload Identity / service accounts, a Terraform GitHub token and
Vercel tokens. StepSecurity Harden-Runner in audit mode records each job's
outbound network traffic so unexpected egress is visible. Audit only: it
never blocks.

## Plan

- Add `step-security/harden-runner` as the first step of every Linux job in
  `.github/workflows/deploy.yml`, with `egress-policy: audit`.
- Pin it to the v2.22.2 commit SHA.
- No other workflow changes. Moving to `block` with an allow-list is a later
  decision, once the audit insights show the normal egress set.

## Log

- 2026-11-12 09:56 — Created plan; added Harden-Runner (audit) to all five deploy jobs
