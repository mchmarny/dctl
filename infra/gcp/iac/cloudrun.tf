# Service account used as the Cloud Run runtime identity
resource "google_service_account" "cloudrun" {
  account_id   = "${var.prefix}-run"
  display_name = "Cloud Run runtime identity for ${var.prefix}"
}

locals {
  # Roles required by the Cloud Run runtime service account
  cloudrun_roles = toset([
    "roles/alloydb.client",              # Connect to AlloyDB instances
    "roles/logging.logWriter",           # Write structured logs
    "roles/monitoring.metricWriter",     # Write custom metrics
    "roles/run.invoker",                  # Cloud Scheduler triggers job executions
    "roles/secretmanager.secretAccessor", # Read connection string secret
  ])
}

# Project-level role bindings for the Cloud Run service account
resource "google_project_iam_member" "cloudrun_roles" {
  for_each = local.cloudrun_roles
  project  = var.project_id
  role     = each.value
  member   = "serviceAccount:${google_service_account.cloudrun.email}"
}
