# List of GCP APIs to enable in this project
locals {
  services = [
  "artifactregistry.googleapis.com",
  "cloudbuild.googleapis.com",
  "cloudresourcemanager.googleapis.com",
  "compute.googleapis.com",
  "container.googleapis.com",
  "containerfilesystem.googleapis.com",
  "containerregistry.googleapis.com",
  "iam.googleapis.com",
  "iamcredentials.googleapis.com",
  "logging.googleapis.com",
  "monitoring.googleapis.com",
  "run.googleapis.com",
  "servicecontrol.googleapis.com",
  "servicemanagement.googleapis.com",
  "serviceusage.googleapis.com",
  "storage-api.googleapis.com",
  ]
}

# Data source to access GCP project metadata 
data "google_project" "project" {}


resource "google_project_service" "default" {
  for_each = toset(local.services)

  project = var.project_id
  service = each.value

  disable_on_destroy = false
}