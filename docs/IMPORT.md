# Importing data

All data is stored locally in an embedded [SQLite](https://www.sqlite.org/) database. Subsequent imports only download new data since the last run.

> All commands assume you have already [authenticated](../README.md#1-authenticate). Importing private repositories requires the default `repo` scope (omit `--public` during `devpulse auth`).

## Import an org (recommended)

Import events, affiliations, metadata, releases, and reputation in one command:

```shell
devpulse import --org <org>
```

Target specific repos:

```shell
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

Deep-score the N lowest-reputation contributors via the GitHub API (runs after the main import):

```shell
devpulse import --org <org> --deep 10
```

## Update all previously imported data

Run `import` with no flags to refresh all previously imported orgs/repos:

```shell
devpulse import
```

This re-imports events, affiliations, substitutions, metadata, releases, metric history, and reputation for every org/repo already in the database.

## What gets imported

| Step | Data | Source |
|------|------|--------|
| Events | PRs, reviews, issues, comments, forks | GitHub API |
| Affiliations | Developer-to-company mappings | [cncf/gitdm](https://github.com/cncf/gitdm) + GitHub profiles |
| Substitutions | Entity name normalizations | Local DB (user-defined via `devpulse substitute`) |
| Metadata | Stars, forks, open issues, language, license | GitHub API |
| Metric history | Daily star/fork counts (30-day backfill) | GitHub API (ListStargazers, ListForks) |
| Releases | Tags, publish dates, asset downloads | GitHub API |
| Reputation | Contributor reputation scores | Local DB + optional GitHub API (with `--deep`) |

## Flags

| Flag | Description | Default |
|------|-------------|---------|
| `--org` | GitHub organization or user | (required for first import) |
| `--repo` | Repository name (repeatable) | all repos in org |
| `--months` | Months of event history to import | 6 |
| `--fresh` | Clear pagination state and re-import from scratch | false |
| `--deep` | Deep-score N lowest-reputation contributors via GitHub API | 0 (disabled) |
| `--format` | Output format: `json` or `yaml` | json |
| `--debug` | Enable verbose logging | false |

## Debug output

Add `--debug` to any command for verbose logging:

```shell
devpulse --debug import --org <org>
```
