# Delete Command Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add a `delete` CLI command that selectively removes imported data for a specific org or org/repo, and add `--force` flag to `reset`.

**Architecture:** New `DeleteRepoData()` function in data layer deletes from 5 tables in a single transaction. New CLI command mirrors import's flag structure. Confirmation prompt unless `--force`.

**Tech Stack:** Go, SQLite (modernc.org/sqlite), urfave/cli/v2, testify

---

### Task 1: Add `forceFlag` to CLI flags

**Files:**
- Modify: `pkg/cli/app.go:28-50` (add flag in var block)

**Step 1: Add the flag definition**

In `pkg/cli/app.go`, add `forceFlag` to the existing `var` block (after `formatFlag`):

```go
forceFlag = &urfave.BoolFlag{
    Name:  "force",
    Usage: "Skip confirmation prompt",
}
```

**Step 2: Verify it compiles**

Run: `cd /Users/mchmarny/dev/devpulse && go build ./...`
Expected: success (flag is defined but not yet referenced by any command)

**Step 3: Commit**

```bash
git add pkg/cli/app.go
git commit -S -m "feat: add forceFlag definition"
```

---

### Task 2: Add `--force` flag to `reset` command

**Files:**
- Modify: `pkg/cli/reset.go`

**Step 1: Add forceFlag to reset command flags**

In `pkg/cli/reset.go:18`, change `Flags` from `[]cli.Flag{debugFlag}` to `[]cli.Flag{debugFlag, forceFlag}`.

**Step 2: Skip confirmation when `--force` is set**

In `cmdReset`, after `applyFlags(c)` and `cfg := getConfig(c)`, add force check. The updated function body:

```go
func cmdReset(c *cli.Context) error {
	applyFlags(c)
	cfg := getConfig(c)

	if !c.Bool(forceFlag.Name) {
		fmt.Printf("This will permanently delete all data in %s\n", cfg.DBPath)
		fmt.Print("Are you sure? [y/N]: ")

		reader := bufio.NewReader(os.Stdin)
		answer, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("reading input: %w", err)
		}

		if strings.ToLower(strings.TrimSpace(answer)) != "y" {
			fmt.Println("Aborted.")
			return nil
		}
	}

	// close the DB before deleting the file
	if cfg.DB != nil {
		cfg.DB.Close()
		cfg.DB = nil
	}

	if err := os.Remove(cfg.DBPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("deleting database: %w", err)
	}

	slog.Info("database deleted", "path", cfg.DBPath)

	// re-initialize empty database
	if err := data.Init(cfg.DBPath); err != nil {
		return fmt.Errorf("re-initializing database: %w", err)
	}

	slog.Info("database re-initialized", "path", cfg.DBPath)
	fmt.Println("Reset complete.")
	return nil
}
```

**Step 3: Verify it compiles**

Run: `go build ./...`

**Step 4: Commit**

```bash
git add pkg/cli/reset.go
git commit -S -m "feat: add --force flag to reset command"
```

---

### Task 3: Create `DeleteRepoData` in data layer (TDD)

**Files:**
- Create: `pkg/data/delete.go`
- Create: `pkg/data/delete_test.go`

**Step 1: Write the test file**

Create `pkg/data/delete_test.go`:

```go
package data

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDeleteRepoData_NilDB(t *testing.T) {
	_, err := DeleteRepoData(nil, "org", "repo")
	assert.ErrorIs(t, err, errDBNotInitialized)
}

func TestDeleteRepoData_EmptyDB(t *testing.T) {
	db := setupTestDB(t)
	result, err := DeleteRepoData(db, "org", "repo")
	require.NoError(t, err)
	assert.Equal(t, "org", result.Org)
	assert.Equal(t, "repo", result.Repo)
	assert.Equal(t, int64(0), result.Events)
	assert.Equal(t, int64(0), result.RepoMeta)
	assert.Equal(t, int64(0), result.Releases)
	assert.Equal(t, int64(0), result.ReleaseAssets)
	assert.Equal(t, int64(0), result.State)
}

func TestDeleteRepoData_WithData(t *testing.T) {
	db := setupTestDB(t)

	// Insert a developer (required by event FK)
	_, err := db.Exec(`INSERT INTO developer (username) VALUES ('testuser')`)
	require.NoError(t, err)

	// Insert events
	_, err = db.Exec(`INSERT INTO event (org, repo, username, type, date) VALUES
		('myorg', 'myrepo', 'testuser', 'pr', '2025-01-01'),
		('myorg', 'myrepo', 'testuser', 'issue', '2025-01-02'),
		('myorg', 'otherrepo', 'testuser', 'pr', '2025-01-03')`)
	require.NoError(t, err)

	// Insert repo_meta
	_, err = db.Exec(`INSERT INTO repo_meta (org, repo, stars, forks, open_issues) VALUES
		('myorg', 'myrepo', 10, 5, 2),
		('myorg', 'otherrepo', 20, 10, 4)`)
	require.NoError(t, err)

	// Insert releases and assets
	_, err = db.Exec(`INSERT INTO release (org, repo, tag, name, published_at, prerelease) VALUES
		('myorg', 'myrepo', 'v1.0', 'Release 1', '2025-01-01', 0)`)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO release_asset (org, repo, tag, name, content_type, size, download_count) VALUES
		('myorg', 'myrepo', 'v1.0', 'binary.tar.gz', 'application/gzip', 1024, 50),
		('myorg', 'myrepo', 'v1.0', 'checksums.txt', 'text/plain', 256, 30)`)
	require.NoError(t, err)

	// Insert state
	_, err = db.Exec(`INSERT INTO state (query, org, repo, page, since) VALUES
		('pr', 'myorg', 'myrepo', 5, 1700000000)`)
	require.NoError(t, err)

	// Delete myorg/myrepo
	result, err := DeleteRepoData(db, "myorg", "myrepo")
	require.NoError(t, err)

	assert.Equal(t, int64(2), result.Events)
	assert.Equal(t, int64(1), result.RepoMeta)
	assert.Equal(t, int64(1), result.Releases)
	assert.Equal(t, int64(2), result.ReleaseAssets)
	assert.Equal(t, int64(1), result.State)

	// Verify otherrepo data is untouched
	var count int
	err = db.QueryRow(`SELECT COUNT(*) FROM event WHERE org = 'myorg' AND repo = 'otherrepo'`).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 1, count)

	// Verify developer is NOT deleted
	err = db.QueryRow(`SELECT COUNT(*) FROM developer WHERE username = 'testuser'`).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 1, count)
}

func TestDeleteRepoData_EmptyParams(t *testing.T) {
	db := setupTestDB(t)

	_, err := DeleteRepoData(db, "", "repo")
	assert.Error(t, err)

	_, err = DeleteRepoData(db, "org", "")
	assert.Error(t, err)
}
```

**Step 2: Run test to verify it fails**

Run: `cd /Users/mchmarny/dev/devpulse && go test ./pkg/data/ -run TestDeleteRepoData -v`
Expected: FAIL — `DeleteRepoData` undefined

**Step 3: Write minimal implementation**

Create `pkg/data/delete.go`:

```go
package data

import (
	"database/sql"
	"fmt"
)

const (
	deleteReleaseAssets = `DELETE FROM release_asset WHERE org = ? AND repo = ?`
	deleteReleases      = `DELETE FROM release WHERE org = ? AND repo = ?`
	deleteEvents        = `DELETE FROM event WHERE org = ? AND repo = ?`
	deleteRepoMeta      = `DELETE FROM repo_meta WHERE org = ? AND repo = ?`
	deleteState         = `DELETE FROM state WHERE org = ? AND repo = ?`
)

type DeleteResult struct {
	Org           string `json:"org" yaml:"org"`
	Repo          string `json:"repo" yaml:"repo"`
	Events        int64  `json:"events" yaml:"events"`
	RepoMeta      int64  `json:"repo_meta" yaml:"repo_meta"`
	Releases      int64  `json:"releases" yaml:"releases"`
	ReleaseAssets int64  `json:"release_assets" yaml:"release_assets"`
	State         int64  `json:"state" yaml:"state"`
}

func DeleteRepoData(db *sql.DB, org, repo string) (*DeleteResult, error) {
	if db == nil {
		return nil, errDBNotInitialized
	}

	if org == "" || repo == "" {
		return nil, fmt.Errorf("org and repo are required (got org=%q, repo=%q)", org, repo)
	}

	tx, err := db.Begin()
	if err != nil {
		return nil, fmt.Errorf("beginning delete transaction: %w", err)
	}

	result := &DeleteResult{Org: org, Repo: repo}

	deletes := []struct {
		sql   string
		field *int64
	}{
		{deleteReleaseAssets, &result.ReleaseAssets},
		{deleteReleases, &result.Releases},
		{deleteEvents, &result.Events},
		{deleteRepoMeta, &result.RepoMeta},
		{deleteState, &result.State},
	}

	for _, d := range deletes {
		res, execErr := tx.Exec(d.sql, org, repo)
		if execErr != nil {
			_ = tx.Rollback()
			return nil, fmt.Errorf("deleting from %s/%s: %w", org, repo, execErr)
		}
		n, _ := res.RowsAffected()
		*d.field = n
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("committing delete transaction: %w", err)
	}

	return result, nil
}
```

**Step 4: Run tests to verify they pass**

Run: `cd /Users/mchmarny/dev/devpulse && go test ./pkg/data/ -run TestDeleteRepoData -v -race`
Expected: all 4 tests PASS

**Step 5: Commit**

```bash
git add pkg/data/delete.go pkg/data/delete_test.go
git commit -S -m "feat: add DeleteRepoData data layer function"
```

---

### Task 4: Create `delete` CLI command

**Files:**
- Create: `pkg/cli/delete.go`
- Modify: `pkg/cli/app.go:86-93` (register command)

**Step 1: Create the CLI command**

Create `pkg/cli/delete.go`:

```go
package cli

import (
	"bufio"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/mchmarny/devpulse/pkg/data"
	"github.com/urfave/cli/v2"
)

var deleteCmd = &cli.Command{
	Name:            "delete",
	Aliases:         []string{"del"},
	HideHelpCommand: true,
	Usage:           "Delete imported data for an org or specific repos",
	UsageText: `devpulse delete --org <ORG> [--repo <REPO>...] [--force]

Examples:
  devpulse delete --org myorg                        # delete all repos in org
  devpulse delete --org myorg --repo repo1           # delete specific repo
  devpulse delete --org myorg --repo r1 --repo r2    # delete multiple repos
  devpulse delete --org myorg --force                # skip confirmation`,
	Action: cmdDelete,
	Flags: []cli.Flag{
		orgNameFlag,
		repoNameFlag,
		forceFlag,
		formatFlag,
		debugFlag,
	},
}

type DeleteCommandResult struct {
	Org      string               `json:"org" yaml:"org"`
	Repos    []*data.DeleteResult `json:"repos" yaml:"repos"`
	Duration string               `json:"duration" yaml:"duration"`
}

func cmdDelete(c *cli.Context) error {
	start := time.Now()
	applyFlags(c)

	org := c.String(orgNameFlag.Name)
	if org == "" {
		return cli.ShowSubcommandHelp(c)
	}

	cfg := getConfig(c)

	// Resolve repos
	repoSlice := c.StringSlice(repoNameFlag.Name)
	repos := repoSlice

	if len(repos) == 0 {
		// Find all repos for this org from existing data
		items, err := data.GetAllOrgRepos(cfg.DB)
		if err != nil {
			return fmt.Errorf("listing repos for org %s: %w", org, err)
		}
		for _, item := range items {
			if item.Org == org {
				repos = append(repos, item.Repo)
			}
		}
		if len(repos) == 0 {
			slog.Info("no data found for org", "org", org)
			return nil
		}
	}

	// Confirmation prompt
	if !c.Bool(forceFlag.Name) {
		fmt.Println("Delete all data for:")
		for _, r := range repos {
			fmt.Printf("  - %s/%s\n", org, r)
		}
		fmt.Print("Continue? [y/N]: ")

		reader := bufio.NewReader(os.Stdin)
		answer, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("reading input: %w", err)
		}
		if strings.ToLower(strings.TrimSpace(answer)) != "y" {
			fmt.Println("Aborted.")
			return nil
		}
	}

	res := &DeleteCommandResult{
		Org:   org,
		Repos: make([]*data.DeleteResult, 0, len(repos)),
	}

	for _, r := range repos {
		slog.Info("deleting", "org", org, "repo", r)
		dr, err := data.DeleteRepoData(cfg.DB, org, r)
		if err != nil {
			slog.Error("failed to delete repo data", "org", org, "repo", r, "error", err)
			continue
		}
		res.Repos = append(res.Repos, dr)
	}

	res.Duration = time.Since(start).String()

	if err := encode(res); err != nil {
		return fmt.Errorf("encoding result: %w", err)
	}

	return nil
}
```

**Step 2: Register the command in app.go**

In `pkg/cli/app.go:86-93`, add `deleteCmd` to the `Commands` slice:

```go
Commands: []*urfave.Command{
    authCmd,
    importCmd,
    deleteCmd,
    substituteCmd,
    queryCmd,
    serverCmd,
    resetCmd,
},
```

**Step 3: Verify it compiles**

Run: `go build ./...`

**Step 4: Run full test suite**

Run: `cd /Users/mchmarny/dev/devpulse && make test`
Expected: all tests pass

**Step 5: Run linter**

Run: `cd /Users/mchmarny/dev/devpulse && make lint`
Expected: no lint errors

**Step 6: Commit**

```bash
git add pkg/cli/delete.go pkg/cli/app.go
git commit -S -m "feat: add delete CLI command"
```

---

### Task 5: Final qualification

**Step 1: Run full qualify**

Run: `cd /Users/mchmarny/dev/devpulse && make qualify`
Expected: test + lint + vulnerability scan all pass

**Step 2: Verify CLI help**

Run: `go run ./cmd/devpulse/ delete --help`
Expected: shows usage with --org, --repo, --force, --format, --debug flags
