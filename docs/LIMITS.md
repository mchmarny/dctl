# GitHub API Rate Limits

This document describes the GitHub API rate limits that affect `devpulse` during data import.

## Primary Rate Limit

GitHub allows **5,000 requests/hour** for authenticated users (PAT or OAuth token).

The importer tracks remaining requests via response headers. When remaining requests drop below 10, the importer automatically sleeps until the rate limit resets (with jitter to avoid thundering herd).

## Secondary (Abuse) Rate Limits

GitHub enforces additional limits to prevent abuse:

| Limit | Threshold | Impact |
|-------|-----------|--------|
| Concurrent requests | 100 | Unlikely to hit (importer runs up to 5 event types × `--concurrency` repos) |
| REST API points | 900/minute | Burst-heavy imports may trigger this, especially at high concurrency |
| CPU time | 90s per 60s real time | Unlikely with REST calls |

Secondary limits return HTTP 403 with a `Retry-After` header. The importer detects `AbuseRateLimitError` responses in the PR detail backfill loop and retries after the specified wait period. If no `Retry-After` header is present, it defaults to a 60-second wait.

## API Calls Per Import

Each repo import makes the following API calls:

| Operation | Calls | Notes |
|-----------|-------|-------|
| PR list | 1+ | Paginated (100 per page) |
| PR reviews (per PR) | 1 | One call per PR in the batch |
| PR detail backfill (per PR) | 0-1 | Only for PRs missing size data |
| PR review events list | 1+ | Paginated |
| Issues list | 1+ | Paginated |
| Issue comments list | 1+ | Paginated |
| Forks list | 1+ | Paginated |

### First Import

On the first import for a repo, all PRs need size data backfilled. For a repo with 200 PRs across 3 pages:

- Base paginated calls: ~8 (3 PR pages + 1 each for reviews, issues, comments, forks)
- PR reviews: ~200 (one per PR)
- PR detail backfill: ~200 (one per PR)
- **Total: ~408 calls**

### Subsequent Imports

On a recently updated DB, only new PRs need backfilling. For a repo with 20 new PRs:

- Base paginated calls: ~5
- PR reviews: ~20
- PR detail backfill: ~20
- **Total: ~45 calls**

## Practical Guidance

| Repo Size | PRs/Month | Estimated Calls | Notes |
|-----------|-----------|-----------------|-------|
| Small | < 50 | < 100 | No issues |
| Medium | 50-200 | 100-500 | Fine within hourly limit |
| Large | 200-1000 | 500-2500 | May approach primary limit across multiple repos |
| Very Large | 1000+ | 2500+ | Will hit primary limit; importer auto-waits for reset |

### Multiple Repos

When importing across many repos, calls accumulate. With 10 medium repos, expect ~1,000-5,000 calls per full import cycle. The `--concurrency` flag (default: 3) controls how many repos import in parallel — higher values finish faster but consume API quota more quickly. The importer handles this by sleeping when the primary rate limit is nearly exhausted.

### Subsequent Imports Are Cheaper

Releases, container versions, and repo metadata use incremental fetching — they stop paging once they reach already-known data. This significantly reduces API calls on repeated imports.

## Rate Limit Handling Summary

| Limit Type | Detection | Response |
|------------|-----------|----------|
| Primary (approaching) | `Rate.Remaining <= 10` | Warn + sleep until reset + jitter |
| Primary (exhausted) | `Rate.Remaining == 0` | Warn + sleep until reset + jitter |
| Secondary (abuse) | `AbuseRateLimitError` | Warn + sleep for `Retry-After` duration, then retry once |
