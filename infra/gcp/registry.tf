# Remote Artifact Registry repository that proxies GitHub Container Registry.
# Cloud Run pulls images through this repo — no need to copy images from GHCR.
resource "google_artifact_registry_repository" "default" {
  repository_id = "${var.prefix}-remote"
  project       = var.project_id
  location      = var.location
  format        = "DOCKER"
  mode          = "REMOTE_REPOSITORY"
  description   = "Remote repository proxying GHCR for ${var.prefix} images"

  remote_repository_config {
    docker_repository {
      custom_repository {
        uri = "https://ghcr.io"
      }
    }
  }

  # Cleanup policy to remove old cached images
  cleanup_policies {
    id     = "keep-recent"
    action = "KEEP"
    most_recent_versions {
      keep_count = 10
    }
  }

  depends_on = [google_project_service.default]
}

# Grant the Cloud Run runtime SA permission to pull images through the remote repo
resource "google_artifact_registry_repository_iam_member" "cloudrun_reader" {
  repository = google_artifact_registry_repository.default.name
  location   = google_artifact_registry_repository.default.location
  role       = "roles/artifactregistry.reader"
  member     = "serviceAccount:${google_service_account.cloudrun.email}"
}

# Grant the GitHub Actions SA permission to read images during Cloud Run deploys
# (the control plane validates the image using the caller's credentials)
resource "google_artifact_registry_repository_iam_member" "github_actions_reader" {
  repository = google_artifact_registry_repository.default.name
  location   = google_artifact_registry_repository.default.location
  role       = "roles/artifactregistry.reader"
  member     = "serviceAccount:${google_service_account.github_actions_user.email}"
}
