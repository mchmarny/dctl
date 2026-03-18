locals {
  # List of roles that will be assigned to the GitHub Actions service account
  publisher_roles = toset([
    "roles/iam.serviceAccountUser", # Act as Cloud Run runtime SA during deploys
    "roles/run.developer",          # Update Cloud Run services and jobs
  ])
}

# Service account used by GitHub Actions for federated auth to deploy to Cloud Run
resource "google_service_account" "github_actions_user" {
  account_id   = "github-actions-${var.prefix}"
  display_name = "Service Account impersonated in GitHub Actions"
}

# Project-level role bindings for the service account
resource "google_project_iam_member" "github_actions_user_roles" {
  for_each = local.publisher_roles
  project  = var.project_id
  role     = each.value
  member   = "serviceAccount:${google_service_account.github_actions_user.email}"
}

# Identity pool for GitHub action based identity's access to Google Cloud resources
resource "google_iam_workload_identity_pool" "github_pool" {
  workload_identity_pool_id = "github-actions-pool-${var.prefix}"
}

# Configuration for GitHub Identity provider
resource "google_iam_workload_identity_pool_provider" "github_provider" {
  workload_identity_pool_id          = google_iam_workload_identity_pool.github_pool.workload_identity_pool_id
  workload_identity_pool_provider_id = "github-actions-provider-${var.prefix}"
  attribute_mapping = {
    "google.subject"       = "assertion.sub"
    "attribute.aud"        = "assertion.aud"
    "attribute.actor"      = "assertion.actor"
    "attribute.repository" = "assertion.repository"
  }
  attribute_condition = "assertion.repository == '${var.git_repo}'"
  oidc {
    issuer_uri        = "https://token.actions.githubusercontent.com"
    allowed_audiences = []
  }
}

# IAM policy bindings to the service account resources created by GitHub identity
resource "google_service_account_iam_member" "pool_impersonation" {
  service_account_id = google_service_account.github_actions_user.id
  role               = "roles/iam.workloadIdentityUser"
  member             = "principalSet://iam.googleapis.com/${google_iam_workload_identity_pool.github_pool.name}/attribute.repository/${var.git_repo}"
}


