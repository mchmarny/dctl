#!/usr/bin/env bash
set -euo pipefail

# Creates log-based metrics for the devpulse sync job.
# Requires the sync job to run with --log-json for jsonPayload fields.

PROJECT_ID="${PROJECT_ID:?PROJECT_ID must be set}"
TOKEN="$(gcloud auth print-access-token)"
API="https://logging.googleapis.com/v2/projects/${PROJECT_ID}/metrics"

echo "Creating log-based metrics in project: ${PROJECT_ID}"

# --- Counter metrics ---

echo "Creating sync_completions..."
gcloud logging metrics create sync_completions \
    --project="${PROJECT_ID}" \
    --description="Count of sync job completions" \
    --log-filter='resource.type="cloud_run_job" jsonPayload.msg="sync_summary"'

echo "Creating sync_errors..."
gcloud logging metrics create sync_errors \
    --project="${PROJECT_ID}" \
    --description="Count of sync job errors" \
    --log-filter='resource.type="cloud_run_job" severity="ERROR"'

echo "Creating sync_rate_limit_pauses..."
gcloud logging metrics create sync_rate_limit_pauses \
    --project="${PROJECT_ID}" \
    --description="Count of GitHub API rate limit pauses" \
    --log-filter='resource.type="cloud_run_job" jsonPayload.msg=~"rate limit"'

# --- Distribution metrics ---

echo "Creating sync_total_duration..."
curl -sf -X POST "${API}" \
    -H "Authorization: Bearer ${TOKEN}" \
    -H "Content-Type: application/json" \
    -d '{
      "name": "sync_total_duration",
      "description": "Total sync job duration in seconds",
      "filter": "resource.type=\"cloud_run_job\" jsonPayload.msg=\"sync_summary\"",
      "metricDescriptor": { "metricKind": "DELTA", "valueType": "DISTRIBUTION", "unit": "s" },
      "valueExtractor": "EXTRACT(jsonPayload.total_sec)",
      "bucketOptions": { "explicitBuckets": { "bounds": [10, 30, 60, 120, 300, 600] } }
    }'

echo "Creating sync_import_duration..."
curl -sf -X POST "${API}" \
    -H "Authorization: Bearer ${TOKEN}" \
    -H "Content-Type: application/json" \
    -d '{
      "name": "sync_import_duration",
      "description": "Event import phase duration in seconds",
      "filter": "resource.type=\"cloud_run_job\" jsonPayload.msg=\"sync_summary\"",
      "metricDescriptor": { "metricKind": "DELTA", "valueType": "DISTRIBUTION", "unit": "s" },
      "valueExtractor": "EXTRACT(jsonPayload.import_sec)",
      "bucketOptions": { "explicitBuckets": { "bounds": [10, 30, 60, 120, 300, 600] } }
    }'

echo "Creating sync_scoring_duration..."
curl -sf -X POST "${API}" \
    -H "Authorization: Bearer ${TOKEN}" \
    -H "Content-Type: application/json" \
    -d '{
      "name": "sync_scoring_duration",
      "description": "Deep scoring phase duration in seconds",
      "filter": "resource.type=\"cloud_run_job\" jsonPayload.msg=\"sync_summary\"",
      "metricDescriptor": { "metricKind": "DELTA", "valueType": "DISTRIBUTION", "unit": "s" },
      "valueExtractor": "EXTRACT(jsonPayload.scoring_sec)",
      "bucketOptions": { "explicitBuckets": { "bounds": [10, 30, 60, 120, 300, 600] } }
    }'

echo "Creating sync_rate_limit_wait..."
curl -sf -X POST "${API}" \
    -H "Authorization: Bearer ${TOKEN}" \
    -H "Content-Type: application/json" \
    -d '{
      "name": "sync_rate_limit_wait",
      "description": "Rate limit wait duration in seconds",
      "filter": "resource.type=\"cloud_run_job\" jsonPayload.msg=~\"rate limit\"",
      "metricDescriptor": { "metricKind": "DELTA", "valueType": "DISTRIBUTION", "unit": "s" },
      "valueExtractor": "EXTRACT(jsonPayload.wait_sec)",
      "bucketOptions": { "explicitBuckets": { "bounds": [5, 15, 30, 60, 120, 300, 600] } }
    }'

echo "Done. All metrics created."
