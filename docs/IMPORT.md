# Importing data

By default, data is stored locally in an embedded [SQLite](https://www.sqlite.org/) database (`~/.devpulse/data.db`). Pass a `postgres://` URI via `--db` or `DEVPULSE_DB` to use PostgreSQL instead. Subsequent imports only download new data since the last run.

> All commands assume you have already [authenticated](../README.md#1-authenticate). Importing private repositories requires the default `repo` scope (omit `--public` during `devpulse auth`).

## Import an org (recommended)

Import events, affiliations, metadata, releases, and reputation:

```shell
devpulse import --org <org> --repo <repo>
devpulse import --org <org> --repo <repo1> --repo <repo2>
```

Force a full re-import (clears pagination state):

```shell
devpulse import --org <org> --fresh
```

By default, `devpulse` downloads the last 6 months of events. Use `--months` to adjust:

```shell
devpulse import --org <org> --months 12
```

## Update all previously imported data

Run `import` with no flags to refresh all previously imported orgs/repos:

```shell
devpulse import
```

This re-imports events, affiliations, substitutions, metadata, releases, metric history, container versions, and reputation for every org/repo already in the database.

Repos are imported in parallel. Use `--concurrency` to control how many repos run at once (default: 3):

```shell
devpulse import --concurrency 2
```

Higher values speed up imports but consume more GitHub API quota. Values above the default trigger a warning.

## Incremental imports

Subsequent imports are faster than the first run:

- **Events** resume from the last imported page (pagination state is stored in the DB)
- **Releases** stop fetching once they reach already-known releases
- **Container versions** stop fetching once they reach already-known versions
- **Repo metadata** skips the GitHub API call if updated within the last 24 hours
- **PR size backfill** only fetches details for PRs missing size data

## What gets imported

| Step | Data | Source |
|------|------|--------|
| Events | PRs, reviews, issues, comments, forks | GitHub API |
| Affiliations | Developer-to-company mappings | [cncf/gitdm](https://github.com/cncf/gitdm) + GitHub profiles |
| Substitutions | Entity name normalizations | Local DB (user-defined via `devpulse substitute`) |
| Metadata | Stars, forks, open issues, language, license | GitHub API |
| Metric history | Daily star/fork counts (30-day backfill) | GitHub API (ListStargazers, ListForks) |
| Releases | Tags, publish dates, asset downloads | GitHub API |
| Reputation | Shallow contributor reputation scores (no API calls) | Local DB |

## Flags

| Flag | Description | Default |
|------|-------------|---------|
| `--org` | GitHub organization or user | (required for first import) |
| `--repo` | Repository name (repeatable, required with --org) | — |
| `--months` | Months of event history to import | 6 |
| `--fresh` | Clear pagination state and re-import from scratch | false |
| `--concurrency` | Number of repos to import in parallel | 3 |
| `--format` | Output format: `json` or `yaml` | json |
| `--debug` | Enable verbose logging | false |
| `--log-json` | Output logs in JSON format | false |

## Debug output

Add `--debug` to any subcommand for verbose logging:

```shell
devpulse import --debug --org <org>
```

For structured JSON log output (useful in cloud environments), add `--log-json`:

```shell
devpulse import --debug --log-json --org <org>
```
