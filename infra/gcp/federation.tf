locals {
  # List of roles that will be assigned to the publisher service account
  publisher_roles = toset([
    "roles/artifactregistry.writer",
    "roles/iam.serviceAccountUser",
    "roles/logging.logWriter",
    "roles/monitoring.metricWriter",
    "roles/run.developer",
    "roles/storage.objectAdmin",
    "roles/storage.objectViewer",
  ])
}

# Service account to be used for federated auth to publish to GCR
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

# Allow github-actions-user to use the Compute Engine default service account for GKE
resource "google_service_account_iam_member" "compute_service_account_user" {
  service_account_id = "projects/${var.project_id}/serviceAccounts/${data.google_project.project.number}-compute@developer.gserviceaccount.com"
  role               = "roles/iam.serviceAccountUser"
  member             = "serviceAccount:${google_service_account.github_actions_user.email}"
}

