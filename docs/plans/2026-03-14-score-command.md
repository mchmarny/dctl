# Score Command Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Extract reputation scoring from the import command into a standalone `score` CLI command with `--org` (required) and optional `--repo`/`--deep` flags.

**Architecture:** Add org/repo COALESCE filters to the two reputation SQL queries, update `ImportReputation` and `ImportDeepReputation` signatures to accept optional org/repo, create `pkg/cli/score.go` following the delete command pattern, remove reputation steps from import/update flows.

**Tech Stack:** Go, urfave/cli/v2, SQLite, testify

---

### Task 1: Add org/repo filters to data layer

**Files:**
- Modify: `pkg/data/reputation.go` (SQL constants + function signatures)
- Modify: `pkg/data/reputation_test.go` (update callers, add filter tests)

**Step 1: Update SQL constants**

In `pkg/data/reputation.go`, add org/repo COALESCE clauses to `selectStaleReputationUsernamesSQL` and `selectLowestReputationUsernamesSQL`:

```go
selectStaleReputationUsernamesSQL = `SELECT DISTINCT d.username
    FROM developer d
    JOIN event e ON d.username = e.username
    WHERE d.username NOT LIKE '%[bot]'
      AND d.username NOT IN ('copilot','github-copilot','claude','anthropic-claude')
      AND e.org = COALESCE(?, e.org)
      AND e.repo = COALESCE(?, e.repo)
      AND (d.reputation IS NULL
       OR d.reputation_updated_at IS NULL
       OR d.reputation_updated_at < ?)
`

selectLowestReputationUsernamesSQL = `SELECT d.username
    FROM developer d
    JOIN event e ON d.username = e.username
    WHERE d.reputation IS NOT NULL
      AND d.username NOT LIKE '%[bot]'
      AND d.username NOT IN ('copilot','github-copilot','claude','anthropic-claude')
      AND e.org = COALESCE(?, e.org)
      AND e.repo = COALESCE(?, e.repo)
      AND (d.reputation_deep IS NULL OR d.reputation_deep = 0
       OR d.reputation_updated_at IS NULL
       OR d.reputation_updated_at < ?)
    GROUP BY d.username
    ORDER BY d.reputation ASC
    LIMIT ?
`
```

**Step 2: Update function signatures**

Change `getStaleReputationUsernames`:
```go
func getStaleReputationUsernames(db *sql.DB, org, repo *string, threshold string) ([]string, error) {
```
Update the query call to pass `org, repo, threshold`.

Change `getLowestReputationUsernames`:
```go
func getLowestReputationUsernames(db *sql.DB, org, repo *string, threshold string, limit int) ([]string, error) {
```
Update the query call to pass `org, repo, threshold, limit`.

Change `ImportReputation`:
```go
func ImportReputation(db *sql.DB, org, repo *string) (*ReputationResult, error) {
```
Pass `org, repo` through to `getStaleReputationUsernames`.

Change `ImportDeepReputation`:
```go
func ImportDeepReputation(db *sql.DB, token string, limit int, org, repo *string) (*DeepReputationResult, error) {
```
Pass `org, repo` through to `getLowestReputationUsernames`.

**Step 3: Write tests for org/repo filtering**

Add to `reputation_test.go`:

```go
func TestGetStaleReputationUsernames_FilterByOrg(t *testing.T) {
    db := setupTestDB(t)

    devs := []*Developer{
        {Username: "orguser", FullName: "Org User"},
        {Username: "otheruser", FullName: "Other User"},
    }
    require.NoError(t, SaveDevelopers(db, devs))

    _, err := db.Exec(`INSERT INTO event (org, repo, username, type, date, url, mentions, labels)
        VALUES
        ('nvidia', 'repo1', 'orguser', 'pr', '2025-01-10', 'http://example.com', '', ''),
        ('other', 'repo2', 'otheruser', 'pr', '2025-01-10', 'http://example.com', '', '')`)
    require.NoError(t, err)

    org := "nvidia"
    usernames, err := getStaleReputationUsernames(db, &org, nil, "2025-01-15T00:00:00Z")
    require.NoError(t, err)
    assert.Contains(t, usernames, "orguser")
    assert.NotContains(t, usernames, "otheruser")
}

func TestGetStaleReputationUsernames_FilterByOrgAndRepo(t *testing.T) {
    db := setupTestDB(t)

    devs := []*Developer{
        {Username: "repouser", FullName: "Repo User"},
        {Username: "otherrepo", FullName: "Other Repo"},
    }
    require.NoError(t, SaveDevelopers(db, devs))

    _, err := db.Exec(`INSERT INTO event (org, repo, username, type, date, url, mentions, labels)
        VALUES
        ('nvidia', 'skyhook', 'repouser', 'pr', '2025-01-10', 'http://example.com', '', ''),
        ('nvidia', 'other', 'otherrepo', 'pr', '2025-01-10', 'http://example.com', '', '')`)
    require.NoError(t, err)

    org := "nvidia"
    repo := "skyhook"
    usernames, err := getStaleReputationUsernames(db, &org, &repo, "2025-01-15T00:00:00Z")
    require.NoError(t, err)
    assert.Contains(t, usernames, "repouser")
    assert.NotContains(t, usernames, "otherrepo")
}
```

**Step 4: Update existing test callers**

All existing calls to these functions pass `nil, nil` for org/repo:
- `getStaleReputationUsernames(db, nil, nil, threshold)`
- `getLowestReputationUsernames(db, nil, nil, threshold, limit)`
- `ImportReputation(db, nil, nil)`
- `ImportDeepReputation(db, token, limit, nil, nil)`

**Step 5: Run tests**

Run: `make test`
Expected: All pass

**Step 6: Commit**

```
feat: add org/repo filters to reputation scoring queries
```

---

### Task 2: Create score CLI command

**Files:**
- Create: `pkg/cli/score.go`

**Step 1: Create score.go**

Follow the `deleteCmd` pattern in `pkg/cli/delete.go`:

```go
package cli

import (
    "fmt"
    "log/slog"
    "time"

    "github.com/mchmarny/devpulse/pkg/data"
    "github.com/urfave/cli/v2"
)

var scoreCmd = &cli.Command{
    Name:            "score",
    HideHelpCommand: true,
    Usage:           "Compute reputation scores for developers in an org or repo",
    UsageText: `devpulse score --org <ORG> [--repo <REPO>] [--deep <N>]

Examples:
  devpulse score --org myorg                        # shallow-score all org contributors
  devpulse score --org myorg --repo repo1           # shallow-score repo contributors
  devpulse score --org myorg --deep 10              # also deep-score 10 lowest
  devpulse score --org myorg --repo repo1 --deep 5  # scoped deep scoring`,
    Action: cmdScore,
    Flags: []cli.Flag{
        orgNameFlag,
        repoNameFlag,
        deepFlag,
        formatFlag,
        debugFlag,
    },
}

type ScoreResult struct {
    Org            string                    `json:"org" yaml:"org"`
    Repo           string                    `json:"repo,omitempty" yaml:"repo,omitempty"`
    Reputation     *data.ReputationResult    `json:"reputation" yaml:"reputation"`
    DeepReputation *data.DeepReputationResult `json:"deep_reputation,omitempty" yaml:"deep_reputation,omitempty"`
    Duration       string                    `json:"duration" yaml:"duration"`
}

func cmdScore(c *cli.Context) error {
    start := time.Now()
    applyFlags(c)

    org := c.String(orgNameFlag.Name)
    if org == "" {
        return cli.ShowSubcommandHelp(c)
    }

    cfg := getConfig(c)

    // Use first repo from slice if provided (score is single-repo scoped)
    var repo string
    if repos := c.StringSlice(repoNameFlag.Name); len(repos) > 0 {
        repo = repos[0]
    }

    orgPtr := &org
    var repoPtr *string
    if repo != "" {
        repoPtr = &repo
    }

    // Shallow reputation (local DB only)
    slog.Info("scoring reputation", "org", org, "repo", repo)
    repResult, err := data.ImportReputation(cfg.DB, orgPtr, repoPtr)
    if err != nil {
        return fmt.Errorf("failed to compute reputation scores: %w", err)
    }

    res := &ScoreResult{
        Org:        org,
        Repo:       repo,
        Reputation: repResult,
    }

    // Deep reputation (GitHub API)
    if deep := c.Int(deepFlag.Name); deep > 0 {
        token, tokenErr := getGitHubToken()
        if tokenErr != nil {
            return fmt.Errorf("failed to get GitHub token: %w", tokenErr)
        }
        if token == "" {
            return fmt.Errorf("GitHub token required for deep scoring")
        }

        slog.Info("deep scoring", "org", org, "repo", repo, "limit", deep)
        deepResult, deepErr := data.ImportDeepReputation(cfg.DB, token, deep, orgPtr, repoPtr)
        if deepErr != nil {
            return fmt.Errorf("failed to compute deep reputation scores: %w", deepErr)
        }
        res.DeepReputation = deepResult
    }

    res.Duration = time.Since(start).String()

    if err := encode(res); err != nil {
        return fmt.Errorf("encoding result: %w", err)
    }

    return nil
}
```

**Step 2: Run tests**

Run: `make test`
Expected: All pass (no test file needed for the CLI command itself — it follows the same pattern as delete which has no unit tests)

**Step 3: Commit**

```
feat: add score CLI command for reputation scoring
```

---

### Task 3: Remove reputation from import command

**Files:**
- Modify: `pkg/cli/import.go`

**Step 1: Remove reputation from cmdImport**

In `cmdImport()`, remove steps 5 and 6 (lines ~173-191) — the reputation and deep reputation blocks. Remove the `slog.Info("reputation")` line added earlier. Remove `Reputation` and `DeepReputation` fields from `ImportResult`.

Remove `deepFlag` from `importCmd.Flags`.

Remove from `ImportResult`:
```go
Reputation     *data.ReputationResult        `json:"reputation,omitempty" yaml:"reputation,omitempty"`
DeepReputation *data.DeepReputationResult    `json:"deep_reputation,omitempty" yaml:"deep_reputation,omitempty"`
```

**Step 2: Remove reputation from cmdUpdate**

In `cmdUpdate()`, remove the reputation and deep reputation blocks (the `slog.Info("reputation")`, `ImportReputation` call, deep reputation block, and the corresponding result fields).

Remove `Reputation` and `DeepReputation` from the result struct literal.

**Step 3: Clean up deepFlag**

Remove the `deepFlag` variable definition from `import.go` since it's no longer used there. Move it to `score.go` or keep as shared — check if it's only used by score now.

**Step 4: Run tests**

Run: `make test`
Expected: All pass

**Step 5: Commit**

```
refactor: remove reputation scoring from import command
```

---

### Task 4: Register score command

**Files:**
- Modify: `pkg/cli/app.go` (line ~91-99)

**Step 1: Add scoreCmd to command list**

Add `scoreCmd` to the `Commands` slice in `newApp()`:

```go
Commands: []*urfave.Command{
    authCmd,
    importCmd,
    deleteCmd,
    scoreCmd,
    substituteCmd,
    queryCmd,
    serverCmd,
    resetCmd,
},
```

**Step 2: Run qualify**

Run: `make qualify`
Expected: All tests pass, lint clean, no vulnerabilities

**Step 3: Commit**

```
feat: register score command in CLI app
```

---

### Task 5: Update existing reputation tests for new signatures

**Files:**
- Modify: `pkg/data/reputation_test.go`

This task ensures all existing tests compile with the updated function signatures. The bulk of changes are adding `nil, nil` for org/repo params.

Key tests to update:
- `TestImportReputation_NilDB`: `ImportReputation(nil, nil, nil)`
- `TestImportReputation_EmptyDB`: `ImportReputation(db, nil, nil)`
- `TestImportReputation_ComputesShallowScores`: `ImportReputation(db, nil, nil)`
- `TestImportDeepReputation_NilDB`: `ImportDeepReputation(nil, "token", 5, nil, nil)`
- `TestImportDeepReputation_EmptyToken`: `ImportDeepReputation(db, "", 5, nil, nil)`
- `TestImportDeepReputation_ZeroLimit`: `ImportDeepReputation(db, "token", 0, nil, nil)`
- `TestImportDeepReputation_NoCandidates`: `ImportDeepReputation(db, "token", 5, nil, nil)`
- `TestGetStaleReputationUsernames_*`: add `nil, nil,` before threshold
- `TestGetLowestReputationUsernames_*`: add `nil, nil,` before threshold

Note: This task may be done together with Task 1 to keep the code compiling at each step.

**Run:** `make qualify`
Expected: All pass, lint clean

**Commit:**
```
test: update reputation tests for org/repo filter signatures
```
