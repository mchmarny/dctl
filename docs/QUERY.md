# CLI queries

> Assumes you have already [authenticated](../README.md#1-authenticate) and [imported data](IMPORT.md).

All query output is JSON, suitable for piping to [jq](https://stedolan.github.io/jq/) or other tools.

## Developers

List developers matching a pattern:

```shell
devpulse query developer list --like mark
```

Get details for a specific developer:

```shell
devpulse query developer detail --name mchmarny
```

## Entities

List entities (companies/orgs) matching a pattern:

```shell
devpulse query entity list --like google
```

Get entity details with affiliated developers:

```shell
devpulse query entity detail --name GOOGLE
```

## Repositories

List repositories in an organization:

```shell
devpulse query org repos --org knative
```

## Events

Search events with filters:

```shell
devpulse query events --org knative --repo serving --type pr --since 2024-01-01
```

Available filters: `--org`, `--repo`, `--type` (pr, pr_review, issue, issue_comment, fork), `--author`, `--since`, `--label`, `--mention`, `--limit`.

Pipe to jq for post-processing:

```shell
devpulse query events --org knative --repo serving --type pr | jq '. | length'
```

Use `--limit` on any list command to control result count (default: 100, max: 500).

## Direct SQL access

The database is at `~/.devpulse/data.db`:

```shell
sqlite3 ~/.devpulse/data.db
```

Schema is defined in [pkg/data/sql/migrations/](../pkg/data/sql/migrations/). Key tables:

| Table | Primary Key | Description |
|-------|-------------|-------------|
| `developer` | `username` | Developer profiles and entity affiliations |
| `event` | `org, repo, username, type, date` | Contribution events with optional state/timing fields |
| `repo_meta` | `org, repo` | Repository metadata (stars, forks, language, license) |
| `release` | `org, repo, tag` | Release tags and publish dates |
| `state` | `query, org, repo` | Import pagination state |
| `sub` | `type, old` | Entity name substitutions |
