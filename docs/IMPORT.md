# Importing data

All data is stored locally in an embedded [SQLite](https://www.sqlite.org/) database. Subsequent imports only download new data since the last run.

> All commands assume you have already [authenticated](../README.md#1-authenticate).

## Import all (recommended)

Import events, affiliations, substitutions, metadata, and releases in one command:

```shell
devpulse import all --org <org>
```

Target a specific repo:

```shell
devpulse import all --org <org> --repo <repo>
```

Force a full re-import (clears pagination state):

```shell
devpulse import all --org <org> --fresh
```

By default, `devpulse` downloads the last 6 months of data. Use `--months` to adjust:

```shell
devpulse import all --org <org> --months 12
```

## Individual import commands

### Events

Import GitHub activity data (PRs, reviews, issues, comments, forks):

```shell
devpulse import events --org <org> --repo <repo>
```

Omit `--repo` to import all repos in the org. Use `--fresh` to re-import from page 1.

### Affiliations

Enrich developer profiles with company affiliations from [cncf/gitdm](https://github.com/cncf/gitdm) and GitHub profile data:

```shell
devpulse import affiliations
```

### Substitutions

Create entity name mappings to normalize inconsistent affiliations:

```shell
devpulse import substitutions --type entity --old "GOOGLE LLC" --new "GOOGLE"
```

Substitutions are saved and re-applied on every subsequent import.

### Metadata

Import repository metadata (stars, forks, language, license):

```shell
devpulse import metadata --org <org> --repo <repo>
```

Omit flags to import metadata for all previously imported repos.

### Releases

Import release tags and publish dates:

```shell
devpulse import releases --org <org> --repo <repo>
```

### Updates

Re-import all previously configured orgs/repos plus affiliations and substitutions:

```shell
devpulse import updates
```

## Debug output

Add `--debug` to any command for verbose logging:

```shell
devpulse --debug import all --org <org>
```
