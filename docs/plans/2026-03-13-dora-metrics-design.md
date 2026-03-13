# DORA-Inspired Metrics Design

## Overview

Add 8 developer velocity metrics to DevPulse: 4 DORA proxy metrics derived from GitHub data, plus 4 GitHub-native velocity metrics. All metrics built from existing `event` and `release` tables with one schema change (adding `title` column).

## Phase 1: New Metrics (this work)

### DORA Proxy Metrics

#### 1. Deployment Frequency
- **Source:** Release count per month; merge-to-main PR fallback when no releases exist for org/repo
- **Integration:** Extend existing Release Cadence panel with "deployments" series
- **Endpoint:** Extend `/data/insights/release-cadence`

#### 2. Lead Time for Changes
- **Source:** Existing time-to-merge data (PR `created_at` to `merged_at`)
- **Integration:** Relabel existing Time to Merge panel to "Lead Time (PR to Merge)" with DORA context tooltip
- **Endpoint:** Reuse `/data/insights/time-to-merge` (no changes)

#### 3. Change Failure Rate
- **Source A:** Issues with "bug" label created within 7 days of a release or merge-to-main
- **Source B:** PRs with "revert" in title (case-insensitive)
- **Calculation:** `(bug_issues_near_release + revert_prs) / total_deployments` as monthly %
- **Integration:** New chart next to Lead Time panel
- **Endpoint:** New `/data/insights/change-failure-rate`

#### 4. Time to Restore
- **Source:** Time from bug-labeled issue creation to close, filtered to issues opened within 7 days of a deployment
- **Integration:** Extend existing Time to Close panel with "bug resolution" series
- **Endpoint:** Extend `/data/insights/time-to-close` with `bug_only=true` param

### GitHub-Native Velocity Metrics

#### 5. Review Latency
- **Source:** Time from PR `created_at` to first PR_REVIEW event (matched by org, repo, number)
- **Calculation:** Monthly average in hours
- **Integration:** Second series on existing PR Review Ratio panel (right Y-axis)
- **Endpoint:** New `/data/insights/review-latency`

#### 6. PR Size Distribution
- **Source:** `additions + deletions` from event table for PRs
- **Buckets:** S (<50 lines), M (50-250), L (250-1000), XL (>1000)
- **Calculation:** Monthly count per bucket as stacked bar percentages
- **Integration:** New chart near Monthly Activity panel
- **Endpoint:** New `/data/insights/pr-size`

#### 7. Contributor Momentum
- **Source:** Rolling 3-month unique active contributor count, excluding bots
- **Calculation:** `COUNT(DISTINCT username)` per month with rolling window, plus month-over-month delta
- **Integration:** Trend overlay on existing Contributor Retention panel
- **Endpoint:** New `/data/insights/contributor-momentum`

#### 8. First-Time Contributor Funnel
- **Source:** Per contributor, first event by type: first issue comment, first PR, first merged PR
- **Calculation:** Monthly funnel counts per stage
- **Integration:** New chart near Contributor Retention panel
- **Endpoint:** New `/data/insights/contributor-funnel`

## Schema Change

New migration adding `title` column to `event` table:
- `ALTER TABLE event ADD COLUMN title TEXT DEFAULT ''`
- Populated during import from GitHub API response (PR title, issue title)
- Enables revert PR detection via `LOWER(title) LIKE '%revert%'`

## Endpoint Summary

| Endpoint | Action | Response Type |
|---|---|---|
| `/data/insights/release-cadence` | Extend | Add `deployments` field |
| `/data/insights/time-to-close` | Extend | Add `bug_only` query param |
| `/data/insights/change-failure-rate` | New | month, failures, deployments, rate% |
| `/data/insights/review-latency` | New | month, count, avg_hours |
| `/data/insights/pr-size` | New | month, small, medium, large, xlarge |
| `/data/insights/contributor-funnel` | New | month, first_comment, first_pr, first_merge |
| `/data/insights/contributor-momentum` | New | month, active_count, delta |

## Phase 2: Dashboard Tabs (future follow-up)

Reorganize all 14 panels into 5 tabbed sections with lazy loading per tab. Panels load only when their tab is viewed.

**Tab order (Repository is default/first):**

| Tab | Panels |
|---|---|
| **Repository** (default) | Repo Metadata, Star/Fork History, Top Releases |
| **Activity** | Monthly Activity, PR Size Distribution, Forks & Activity |
| **Velocity** | Lead Time, Change Failure Rate, Release Cadence, Downloads |
| **Quality** | PR Review Ratio + Latency, Time to Close + Bug Resolution, Reputation |
| **Community** | Project Health, Retention + Momentum, Contributor Funnel, Entities, Collaborators |
