# Importing data

All data is stored locally in an embedded [SQLite](https://www.sqlite.org/) database. Subsequent imports only download new data since the last run.

> All commands assume you have already [authenticated](../README.md#1-authenticate).

## Import all (recommended)

Import events, affiliations, substitutions, metadata, and releases in one command:

```shell
dctl import all --org <org>
```

Target a specific repo:

```shell
dctl import all --org <org> --repo <repo>
```

Force a full re-import (clears pagination state):

```shell
dctl import all --org <org> --fresh
```

By default, `dctl` downloads the last 6 months of data. Use `--months` to adjust:

```shell
dctl import all --org <org> --months 12
```

## Individual import commands

### Events

Import GitHub activity data (PRs, reviews, issues, comments, forks):

```shell
dctl import events --org <org> --repo <repo>
```

Omit `--repo` to import all repos in the org. Use `--fresh` to re-import from page 1.

### Affiliations

Enrich developer profiles with company affiliations from [cncf/gitdm](https://github.com/cncf/gitdm) and GitHub profile data:

```shell
dctl import affiliations
```

### Substitutions

Create entity name mappings to normalize inconsistent affiliations:

```shell
dctl import substitutions --type entity --old "GOOGLE LLC" --new "GOOGLE"
```

Substitutions are saved and re-applied on every subsequent import.

### Metadata

Import repository metadata (stars, forks, language, license):

```shell
dctl import metadata --org <org> --repo <repo>
```

Omit flags to import metadata for all previously imported repos.

### Releases

Import release tags and publish dates:

```shell
dctl import releases --org <org> --repo <repo>
```

### Updates

Re-import all previously configured orgs/repos plus affiliations and substitutions:

```shell
dctl import updates
```

## Debug output

Add `--debug` to any command for verbose logging:

```shell
dctl --debug import all --org <org>
```
