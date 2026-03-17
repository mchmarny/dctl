# List of variables which can be provided ar runtime to override the specified defaults 

# Deployment Prefix
variable "prefix" {
  description = "Unique identifier for this deployment"
  type        = string
  default     = "devpulse"
}

# GCP Project 
variable "project_id" {
  description = "GCP Project ID"
  type        = string
  default     = "eidosx"
}

# GitHub Repo
variable "git_repo" {
  description = "GitHub Repo"
  type        = string
  default     = "mchmarny/devpulse"
}

# Region for Artifact Registry and Cloud Run
variable "region" {
  description = "GCP region for Artifact Registry and Cloud Run"
  type        = string
  default     = "us-west1"
}

# Artifact Registry Location
variable "location" {
  description = "GCP Artifact Registry location"
  type        = string
  default     = "us"
}
