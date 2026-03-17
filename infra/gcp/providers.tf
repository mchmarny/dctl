# Required terraform and GCP provider versions
terraform {
  required_version = ">= 1.13"

  required_providers {
    google = {
      source  = "hashicorp/google"
      version = "~> 7.9"
    }
  }
}

provider "google" {
  project = var.project_id
}