terraform {
  backend "gcs" {
    bucket = "eidos-tf-state"
    prefix = "devpulse"
  }
}