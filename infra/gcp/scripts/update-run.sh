#!/usr/bin/env bash
set -euo pipefail

# Updates the devpulse Cloud Run service and sync job to use env vars
# for debug/log-json flags instead of CLI args. Safe to delete after running.

PROJECT_ID="${PROJECT_ID:?PROJECT_ID must be set}"
REGION="${REGION:-us-west1}"

echo "Updating Cloud Run resources in project: ${PROJECT_ID} (region: ${REGION})"

echo "Updating devpulse service..."
gcloud run services update devpulse \
    --project="${PROJECT_ID}" \
    --region="${REGION}" \
    --set-env-vars DEVPULSE_DEBUG=true,DEVPULSE_LOG_JSON=true

echo "Updating devpulse-sync job..."
gcloud run jobs update devpulse-sync \
    --project="${PROJECT_ID}" \
    --region="${REGION}" \
    --args "sync,--config,https://raw.githubusercontent.com/mchmarny/devpulse/main/config/nvidia.yaml" \
    --set-env-vars DEVPULSE_DEBUG=true,DEVPULSE_LOG_JSON=true

echo "Done. Both resources updated."
