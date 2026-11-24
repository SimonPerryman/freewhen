project_id       = "freewhen-493819"
project_number   = "350910532607"
region           = "europe-west1"
github_owner     = "SimonPerryman"
github_repo      = "freewhen"
service_name     = "freewhen-api"
gcp_wif_provider = "projects/350910532607/locations/global/workloadIdentityPools/github/providers/freewhen"

# Main-branch ruleset (infra/github.tf). Needs the repo to be public.
enable_branch_protection = true
