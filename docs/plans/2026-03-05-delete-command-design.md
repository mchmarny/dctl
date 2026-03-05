# Delete Command Design

## Summary

Add a `delete` CLI command that selectively removes imported data for a specific org or org/repo. Mirrors `import` flag structure. Also add `--force` flag to existing `reset` command.

## Command

```
devpulse delete --org <org> [--repo <repo>...] [--force] [--format json|yaml] [--debug]
```

- `--org` required
- `--repo` optional, repeatable — without it, deletes all repos in the org
- `--force` skips confirmation prompt
- `--format` structured output (default: json)

## Deletion Scope

Per org/repo pair, delete from these tables in order:

1. `release_asset`
2. `release`
3. `event`
4. `repo_meta`
5. `state`

Developers are never deleted (they may have events across multiple repos).

## Data Layer

```go
// pkg/data/delete.go

type DeleteResult struct {
    Org           string `json:"org"`
    Repo          string `json:"repo"`
    Events        int64  `json:"events"`
    RepoMeta      int64  `json:"repo_meta"`
    Releases      int64  `json:"releases"`
    ReleaseAssets int64  `json:"release_assets"`
    State         int64  `json:"state"`
}

func DeleteRepoData(db *sql.DB, org, repo string) (*DeleteResult, error)
```

- All 5 DELETEs in a single transaction
- Returns row counts per table
- When only `--org` provided, resolve repos via existing query functions

## CLI Layer

```go
// pkg/cli/delete.go

deleteCmd = &cli.Command{
    Name:   "delete",
    Aliases: []string{"del"},
    Usage:  "Delete imported data for an org or repo",
    Action: cmdDelete,
    Flags:  []cli.Flag{orgNameFlag, repoNameFlag, forceFlag, formatFlag, debugFlag},
}
```

### Confirmation Prompt

Unless `--force`:

```
Delete all data for:
  - myorg/repo-a
  - myorg/repo-b
Continue? [y/N]
```

### Output

Structured JSON/YAML with per-repo counts, consistent with import output.

## Reset Command Update

Add `--force` flag to `reset` command. Skip existing confirmation when set.

## Files Changed

| File | Change |
|------|--------|
| `pkg/data/delete.go` | New: `DeleteRepoData()`, `DeleteResult`, SQL constants |
| `pkg/data/delete_test.go` | New: tests (nil DB, empty DB, with data) |
| `pkg/cli/delete.go` | New: `deleteCmd`, `cmdDelete()`, confirmation prompt |
| `pkg/cli/app.go` | Register `deleteCmd` |
| `pkg/cli/reset.go` | Add `--force` flag |
| `pkg/cli/flags.go` | Add `forceFlag` (if flags centralized there) |
