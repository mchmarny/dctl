output "PROJECT_ID" {
  value       = data.google_project.project.name
  description = "Project ID to use in Auth action for GCP in GitHub."
}

output "SERVICE_ACCOUNT" {
  value       = google_service_account.github_actions_user.email
  description = "Service account to use in GitHub Action for federated auth."
}

output "IDENTITY_PROVIDER" {
  value       = google_iam_workload_identity_pool_provider.github_provider.name
  description = "Provider ID to use in Auth action for GCP in GitHub."
}

output "ARTIFACT_REGISTRY" {
  value       = "${var.location}-docker.pkg.dev/${var.project_id}/${google_artifact_registry_repository.default.repository_id}/mchmarny/devpulse"
  description = "Artifact Registry remote repo path for GHCR images (includes GHCR namespace)."
}

output "CLOUDRUN_SERVICE_ACCOUNT" {
  value       = google_service_account.cloudrun.email
  description = "Service account used as Cloud Run runtime identity."
}
