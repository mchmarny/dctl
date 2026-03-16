package data

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/google/go-github/v83/github"
	"github.com/mchmarny/devpulse/pkg/net"
)

const (
	upsertContainerVersionSQL = `INSERT INTO container_version (org, repo, package, version_id, tag, created_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(org, repo, package, version_id) DO UPDATE SET
			tag = excluded.tag,
			created_at = excluded.created_at
	`

	selectLatestContainerVersionSQL = `SELECT COALESCE(MAX(created_at), '')
		FROM container_version
		WHERE org = ? AND repo = ? AND package = ?
	`

	selectContainerActivitySQL = `SELECT
		substr(cv.created_at, 1, 7) AS month,
		COUNT(*) AS versions
	FROM container_version cv
	WHERE cv.org = COALESCE(?, cv.org)
	  AND cv.repo = COALESCE(?, cv.repo)
	  AND cv.created_at >= ?
	GROUP BY month
	ORDER BY month
	`
)

// ImportContainerVersions fetches container package versions from the GitHub API
// and stores them. Repos without container packages are silently skipped.
func ImportContainerVersions(ctx context.Context, dbPath, token, org, repo string) error {
	client := github.NewClient(net.GetOAuthClient(ctx, token))

	matched, err := listRepoContainerPackages(ctx, client, org, repo)
	if err != nil {
		return err
	}
	if len(matched) == 0 {
		return nil
	}

	db, err := GetDB(dbPath)
	if err != nil {
		return fmt.Errorf("getting DB: %w", err)
	}
	defer db.Close()

	total, err := upsertContainerVersions(ctx, client, db, org, repo, matched)
	if err != nil {
		return err
	}

	if total > 0 {
		slog.Info("container versions", "org", org, "repo", repo, "versions", total)
	}

	return nil
}

// listRepoContainerPackages returns container packages that belong to the given repo.
func listRepoContainerPackages(ctx context.Context, client *github.Client, org, repo string) ([]*github.Package, error) {
	opts := &github.PackageListOptions{
		PackageType: github.Ptr("container"),
		ListOptions: github.ListOptions{PerPage: 100},
	}

	var matched []*github.Package
	var total int

	for {
		packages, resp, err := client.Organizations.ListPackages(ctx, org, opts)
		if err != nil {
			if resp != nil && (resp.StatusCode == 404 || resp.StatusCode == 403) {
				slog.Debug("container packages not accessible", "org", org, "status", resp.StatusCode)
				return nil, nil
			}
			return nil, fmt.Errorf("listing packages for %s: %w", org, err)
		}
		checkRateLimit(resp)

		total += len(packages)

		for _, pkg := range packages {
			// Match by repository field if available, or by package name (often lowercase repo name)
			if pkg.Repository != nil && strings.EqualFold(pkg.Repository.GetName(), repo) {
				matched = append(matched, pkg)
			} else if pkg.Repository == nil && strings.EqualFold(pkg.GetName(), repo) {
				matched = append(matched, pkg)
			}
		}

		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	slog.Debug("container packages", "org", org, "repo", repo, "total", total, "matched", len(matched))
	return matched, nil
}

// upsertContainerVersions fetches new versions for the matched packages and stores them.
// It queries the latest stored created_at per package to skip already-imported versions.
func upsertContainerVersions(ctx context.Context, client *github.Client, db *sql.DB, org, repo string, packages []*github.Package) (int, error) {
	tx, err := db.Begin()
	if err != nil {
		return 0, fmt.Errorf("beginning container version tx: %w", err)
	}

	stmt, err := tx.Prepare(upsertContainerVersionSQL)
	if err != nil {
		rollbackTransaction(tx)
		return 0, fmt.Errorf("preparing container version upsert: %w", err)
	}

	var total int
	for _, pkg := range packages {
		pkgName := pkg.GetName()

		var latestCreatedAt string
		if err := db.QueryRow(selectLatestContainerVersionSQL, org, repo, pkgName).Scan(&latestCreatedAt); err != nil {
			rollbackTransaction(tx)
			return 0, fmt.Errorf("querying latest container version for %s/%s/%s: %w", org, repo, pkgName, err)
		}

		n, fetchErr := fetchAndStoreVersions(ctx, client, stmt, org, repo, pkgName, latestCreatedAt)
		if fetchErr != nil {
			rollbackTransaction(tx)
			return 0, fetchErr
		}
		total += n
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("committing container version tx: %w", err)
	}

	return total, nil
}

// fetchAndStoreVersions pages through versions of a single container package.
// When sinceCreatedAt is non-empty, it stops paging once all versions on a page
// are at or before that timestamp (API returns newest-first).
func fetchAndStoreVersions(ctx context.Context, client *github.Client, stmt *sql.Stmt, org, repo, pkgName, sinceCreatedAt string) (int, error) {
	var count int
	opts := &github.PackageListOptions{
		ListOptions: github.ListOptions{PerPage: 100},
	}

	for {
		versions, resp, err := client.Organizations.PackageGetAllVersions(ctx, org, "container", pkgName, opts)
		if err != nil {
			return 0, fmt.Errorf("listing versions for %s/%s: %w", org, pkgName, err)
		}
		checkRateLimit(resp)

		seenOld := false
		for _, v := range versions {
			tag := extractContainerTag(v)
			createdAt := ""
			if v.CreatedAt != nil {
				createdAt = v.CreatedAt.Format("2006-01-02T15:04:05Z")
			}

			if sinceCreatedAt != "" && createdAt <= sinceCreatedAt {
				seenOld = true
				break
			}

			if _, execErr := stmt.Exec(org, repo, pkgName, v.GetID(), tag, createdAt); execErr != nil {
				return 0, fmt.Errorf("inserting container version %d: %w", v.GetID(), execErr)
			}
			count++
		}

		if seenOld || resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	return count, nil
}

// ImportAllContainerVersions imports container versions for all previously imported repos.
func ImportAllContainerVersions(ctx context.Context, dbPath, token string) error {
	db, err := GetDB(dbPath)
	if err != nil {
		return fmt.Errorf("getting DB: %w", err)
	}
	defer db.Close()

	list, err := GetAllOrgRepos(db)
	if err != nil {
		return fmt.Errorf("getting org/repo list: %w", err)
	}

	for _, r := range list {
		if err := ImportContainerVersions(ctx, dbPath, token, r.Org, r.Repo); err != nil {
			slog.Error("container versions failed", "org", r.Org, "repo", r.Repo, "error", err)
		}
	}

	return nil
}

// GetContainerActivity returns monthly container version publish counts.
func GetContainerActivity(db *sql.DB, org, repo *string, months int) (*ContainerActivitySeries, error) { //nolint:dupl,nolintlint // different types and SQL than GetReleaseDownloads
	if db == nil {
		return nil, errDBNotInitialized
	}

	since := sinceDate(months)

	rows, err := db.Query(selectContainerActivitySQL, org, repo, since)
	if err != nil {
		return nil, fmt.Errorf("querying container activity: %w", err)
	}
	defer rows.Close()

	s := &ContainerActivitySeries{
		Months:   make([]string, 0),
		Versions: make([]int, 0),
	}

	for rows.Next() {
		var month string
		var count int
		if err := rows.Scan(&month, &count); err != nil {
			return nil, fmt.Errorf("scanning container activity row: %w", err)
		}
		s.Months = append(s.Months, month)
		s.Versions = append(s.Versions, count)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	return s, nil
}

// extractContainerTag gets the first tag from a package version,
// trying the metadata JSON first (API response), then the webhook-style field.
func extractContainerTag(v *github.PackageVersion) string {
	if v.Metadata != nil {
		var meta github.PackageMetadata
		if err := json.Unmarshal(v.Metadata, &meta); err == nil {
			if meta.Container != nil && len(meta.Container.Tags) > 0 {
				return meta.Container.Tags[0]
			}
		}
	}
	if v.ContainerMetadata != nil && v.ContainerMetadata.Tag != nil {
		return v.ContainerMetadata.Tag.GetName()
	}
	return ""
}
