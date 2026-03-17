# Artifact Registry repository for demo API server images
resource "google_artifact_registry_repository" "default" {
  repository_id = var.prefix
  project       = var.project_id
  location      = var.location
  format        = "DOCKER"
  description   = "Docker repository for images"

  # Cleanup policy to remove old images
  cleanup_policies {
    id     = "keep-recent"
    action = "KEEP"
    most_recent_versions {
      keep_count = 10
    }
  }

  depends_on = [google_project_service.default]
}

# Grant the GitHub Actions service account permission to push images
resource "google_artifact_registry_repository_iam_member" "github_actions_writer" {
  repository = google_artifact_registry_repository.default.name
  location   = google_artifact_registry_repository.default.location
  role       = "roles/artifactregistry.writer"
  member     = "serviceAccount:${google_service_account.github_actions_user.email}"
}

# Grant the GitHub Actions service account permission to read images
resource "google_artifact_registry_repository_iam_member" "github_actions_reader" {
  repository = google_artifact_registry_repository.default.name
  location   = google_artifact_registry_repository.default.location
  role       = "roles/artifactregistry.reader"
  member     = "serviceAccount:${google_service_account.github_actions_user.email}"
}
