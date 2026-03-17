# GCP Infrastructure

Terraform manages IAM, Artifact Registry, and Workload Identity. The resources below are created manually with `gcloud`.

## Prerequisites

```shell
export PROJECT_ID="your-project-id"
export REGION="us-west1"
export SERVICE_ACCOUNT="devpulse-run@${PROJECT_ID}.iam.gserviceaccount.com"
```

## Secrets

Store the GitHub token and AlloyDB connection string in Secret Manager:

```shell
echo -n "<github-pat>" | gcloud secrets create devpulse-github-token \
    --project $PROJECT_ID \
    --data-file=-

echo -n "<alloydb-connection-string>" | gcloud secrets create devpulse-db \
    --project $PROJECT_ID \
    --data-file=-
```

To use multiple GitHub tokens for higher API rate limits (5,000 req/hour per token), store them comma-separated. Tokens need no scopes — they only increase the rate limit:

```shell
echo -n "<token1>,<token2>" | gcloud secrets versions add devpulse-github-token \
    --project $PROJECT_ID \
    --data-file=-
```

## AlloyDB Auth Proxy

Cloud Run uses the [AlloyDB Auth Proxy](https://cloud.google.com/alloydb/docs/auth-proxy/overview) sidecar to connect to the database. The proxy is configured as a Cloud Run sidecar container when creating the service (see below).

To run the proxy locally for testing database connectivity:

```shell
# Download the proxy binary (macOS Apple Silicon)
curl -Lo alloydb-auth-proxy \
    https://storage.googleapis.com/alloydb-auth-proxy/v1.13.11/alloydb-auth-proxy.darwin.arm64
chmod +x alloydb-auth-proxy

# Start the proxy (listens on localhost:5432)
./alloydb-auth-proxy \
    "projects/${PROJECT_ID}/locations/${REGION}/clusters/<cluster-name>/instances/<instance-name>" \
    --port 5432

# In another terminal, connect via psql or devpulse CLI
psql "host=127.0.0.1 port=5432 user=<db-user> dbname=<db-name> sslmode=disable"
```

For other platforms see the [AlloyDB Auth Proxy releases](https://github.com/GoogleCloudPlatform/alloydb-auth-proxy). The proxy authenticates using your local Application Default Credentials (`gcloud auth application-default login`).

## Cloud Run Service

```shell
gcloud run deploy devpulse \
    --image us-docker.pkg.dev/${PROJECT_ID}/devpulse-remote/mchmarny/devpulse:latest \
    --service-account $SERVICE_ACCOUNT \
    --set-secrets DEVPULSE_DB=devpulse-db:latest,GITHUB_TOKEN=devpulse-github-token:latest \
    --network devpulse \
    --subnet app \
    --vpc-egress private-ranges-only \
    --region $REGION
```

## Cloud Run Sync Job

```shell
gcloud run jobs create devpulse-sync \
    --image us-docker.pkg.dev/${PROJECT_ID}/devpulse-remote/mchmarny/devpulse:latest \
    --command /ko-app/devpulse \
    --args "sync,--config,https://raw.githubusercontent.com/mchmarny/devpulse/main/config/<config-name>.yaml,--stale,3d,--debug" \
    --service-account $SERVICE_ACCOUNT \
    --set-secrets DEVPULSE_DB=devpulse-db:latest,GITHUB_TOKEN=devpulse-github-token:latest \
    --network devpulse \
    --subnet app \
    --vpc-egress private-ranges-only \
    --region $REGION \
    --task-timeout 3600
```

## Cloud Scheduler

Schedule the sync job to run hourly:

```shell
gcloud scheduler jobs create http devpulse-sync-schedule \
    --location $REGION \
    --schedule "0 * * * *" \
    --time-zone "UTC" \
    --uri "https://${REGION}-run.googleapis.com/apis/run.googleapis.com/v1/namespaces/${PROJECT_ID}/jobs/devpulse-sync:run" \
    --http-method POST \
    --oauth-service-account-email $SERVICE_ACCOUNT
```

## Monitoring

List recent job executions:

```shell
gcloud run jobs executions list --job devpulse-sync --region $REGION --limit 5
```

Read recent sync job logs:

```shell
gcloud logging read 'resource.type="cloud_run_job" resource.labels.job_name="devpulse-sync"' \
    --limit 20 \
    --format='table(timestamp, textPayload)' \
    --freshness=1h
```

## Log-Based Metrics

Counter metrics (via `gcloud`):

```shell
gcloud logging metrics create sync_completions \
    --description="Count of sync job completions" \
    --log-filter='resource.type="cloud_run_job" jsonPayload.message="sync_summary"'

gcloud logging metrics create sync_errors \
    --description="Count of sync job errors" \
    --log-filter='resource.type="cloud_run_job" severity="ERROR"'

gcloud logging metrics create sync_rate_limit_pauses \
    --description="Count of GitHub API rate limit pauses" \
    --log-filter='resource.type="cloud_run_job" jsonPayload.message=~"rate limit"'
```

Distribution metrics require the REST API (`gcloud` doesn't support value extractors):

```shell
# Total sync duration
curl -X POST "https://logging.googleapis.com/v2/projects/${PROJECT_ID}/metrics" \
    -H "Authorization: Bearer $(gcloud auth print-access-token)" \
    -H "Content-Type: application/json" \
    -d '{
      "name": "sync_total_duration",
      "description": "Total sync job duration in seconds",
      "filter": "resource.type=\"cloud_run_job\" jsonPayload.message=\"sync_summary\"",
      "metricDescriptor": { "metricKind": "DELTA", "valueType": "DISTRIBUTION", "unit": "s" },
      "valueExtractor": "EXTRACT(jsonPayload.total_sec)",
      "bucketOptions": { "explicitBuckets": { "bounds": [10, 30, 60, 120, 300, 600] } }
    }'

# Import phase duration
curl -X POST "https://logging.googleapis.com/v2/projects/${PROJECT_ID}/metrics" \
    -H "Authorization: Bearer $(gcloud auth print-access-token)" \
    -H "Content-Type: application/json" \
    -d '{
      "name": "sync_import_duration",
      "description": "Event import phase duration in seconds",
      "filter": "resource.type=\"cloud_run_job\" jsonPayload.message=\"sync_summary\"",
      "metricDescriptor": { "metricKind": "DELTA", "valueType": "DISTRIBUTION", "unit": "s" },
      "valueExtractor": "EXTRACT(jsonPayload.import_sec)",
      "bucketOptions": { "explicitBuckets": { "bounds": [10, 30, 60, 120, 300, 600] } }
    }'

# Scoring phase duration
curl -X POST "https://logging.googleapis.com/v2/projects/${PROJECT_ID}/metrics" \
    -H "Authorization: Bearer $(gcloud auth print-access-token)" \
    -H "Content-Type: application/json" \
    -d '{
      "name": "sync_scoring_duration",
      "description": "Deep scoring phase duration in seconds",
      "filter": "resource.type=\"cloud_run_job\" jsonPayload.message=\"sync_summary\"",
      "metricDescriptor": { "metricKind": "DELTA", "valueType": "DISTRIBUTION", "unit": "s" },
      "valueExtractor": "EXTRACT(jsonPayload.scoring_sec)",
      "bucketOptions": { "explicitBuckets": { "bounds": [10, 30, 60, 120, 300, 600] } }
    }'

# Rate limit wait duration
curl -X POST "https://logging.googleapis.com/v2/projects/${PROJECT_ID}/metrics" \
    -H "Authorization: Bearer $(gcloud auth print-access-token)" \
    -H "Content-Type: application/json" \
    -d '{
      "name": "sync_rate_limit_wait",
      "description": "Rate limit wait duration in seconds",
      "filter": "resource.type=\"cloud_run_job\" jsonPayload.message=~\"rate limit\"",
      "metricDescriptor": { "metricKind": "DELTA", "valueType": "DISTRIBUTION", "unit": "s" },
      "valueExtractor": "EXTRACT(jsonPayload.wait_sec)",
      "bucketOptions": { "explicitBuckets": { "bounds": [5, 15, 30, 60, 120, 300, 600] } }
    }'
```

Metrics appear in Cloud Monitoring as `logging/user/<metric_name>`.

## Terraform

State is stored in `gs://<your-bucket-name>/devpulse/`. To plan and apply:

```shell
terraform init
terraform plan
terraform apply
```

Managed resources:

| File | Resources |
|---|---|
| `registry.tf` | AR remote repo (proxies GHCR), Cloud Run + GitHub Actions reader IAM |
| `federation.tf` | GitHub Actions SA, Workload Identity pool/provider, IAM roles |
| `cloudrun.tf` | Cloud Run runtime SA, IAM roles (AlloyDB, logging, secrets) |
| `main.tf` | GCP API enablement |
