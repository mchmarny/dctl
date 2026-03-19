package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/google/go-github/v83/github"
	"github.com/mchmarny/devpulse/pkg/data"
	"github.com/mchmarny/devpulse/pkg/data/ghutil"
	"github.com/mchmarny/devpulse/pkg/net"
)

const (
	insertReleaseSQL = `INSERT INTO release (org, repo, tag, name, published_at, prerelease)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(org, repo, tag) DO UPDATE SET
			name = ?, published_at = ?, prerelease = ?
	`

	selectReleaseCadenceSQL = `SELECT
			substr(published_at, 1, 7) AS month,
			COUNT(*) AS total,
			SUM(CASE WHEN prerelease = 0 THEN 1 ELSE 0 END) AS stable
		FROM release
		WHERE org = COALESCE(?, org)
		  AND repo = COALESCE(?, repo)
		  AND published_at >= ?
		GROUP BY month
		ORDER BY month
	`

	insertReleaseAssetSQL = `INSERT INTO release_asset (org, repo, tag, name, content_type, size, download_count)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(org, repo, tag, name) DO UPDATE SET
			content_type = ?, size = ?, download_count = ?
	`

	selectReleaseDownloadsSQL = `SELECT
			substr(r.published_at, 1, 7) AS month,
			SUM(ra.download_count) AS downloads
		FROM release_asset ra
		JOIN release r ON ra.org = r.org AND ra.repo = r.repo AND ra.tag = r.tag
		WHERE ra.org = COALESCE(?, ra.org)
		  AND ra.repo = COALESCE(?, ra.repo)
		  AND r.published_at >= ?
		GROUP BY month
		ORDER BY month
	`

	selectMergedPRDeploymentsSQL = `SELECT
			substr(e.merged_at, 1, 7) AS month,
			COUNT(*) AS cnt
		FROM event e
		JOIN developer d ON e.username = d.username
		WHERE e.type = 'pr'
		  AND e.state = 'merged'
		  AND e.merged_at IS NOT NULL
		  AND e.org = COALESCE(?, e.org)
		  AND e.repo = COALESCE(?, e.repo)
		  AND IFNULL(d.entity, '') = COALESCE(?, IFNULL(d.entity, ''))
		  AND e.merged_at >= ?
		  ` + botExcludeSQL + `
		GROUP BY month
		ORDER BY month
	`

	selectLatestReleaseSQL = `SELECT COALESCE(MAX(published_at), '')
		FROM release
		WHERE org = ? AND repo = ?
	`

	selectReleaseDownloadsByTagSQL = `WITH recent AS (
			SELECT r.org, r.repo, r.tag, r.published_at
			FROM release r
			WHERE r.org = COALESCE(?, r.org)
			  AND r.repo = COALESCE(?, r.repo)
			  AND r.published_at >= ?
			ORDER BY r.published_at DESC
			LIMIT 9
		), top AS (
			SELECT ra.org, ra.repo, ra.tag, r.published_at
			FROM release_asset ra
			JOIN release r ON ra.org = r.org AND ra.repo = r.repo AND ra.tag = r.tag
			WHERE ra.org = COALESCE(?, ra.org)
			  AND ra.repo = COALESCE(?, ra.repo)
			  AND r.published_at >= ?
			GROUP BY ra.org, ra.repo, ra.tag
			ORDER BY SUM(ra.download_count) DESC
			LIMIT 1
		), combined AS (
			SELECT org, repo, tag, published_at FROM recent
			UNION
			SELECT org, repo, tag, published_at FROM top
		)
		SELECT c.tag, COALESCE(SUM(ra.download_count), 0) AS downloads
		FROM combined c
		LEFT JOIN release_asset ra ON c.org = ra.org AND c.repo = ra.repo AND c.tag = ra.tag
		GROUP BY c.tag, c.published_at
		ORDER BY c.published_at
	`
)

func (s *Store) ImportReleases(ctx context.Context, token, owner, repo string) error {
	client := github.NewClient(net.GetOAuthClient(ctx, token))

	var latestPublishedAt string
	if scanErr := s.db.QueryRow(selectLatestReleaseSQL, owner, repo).Scan(&latestPublishedAt); scanErr != nil {
		return fmt.Errorf("querying latest release for %s/%s: %w", owner, repo, scanErr)
	}

	stmt, err := s.db.Prepare(insertReleaseSQL)
	if err != nil {
		return fmt.Errorf("error preparing release insert: %w", err)
	}
	defer stmt.Close()

	assetStmt, err := s.db.Prepare(insertReleaseAssetSQL)
	if err != nil {
		return fmt.Errorf("error preparing release asset insert: %w", err)
	}
	defer assetStmt.Close()

	opt := &github.ListOptions{PerPage: pageSizeDefault, Page: 1}

	for {
		releases, resp, listErr := client.Repositories.ListReleases(ctx, owner, repo, opt)
		if listErr != nil || resp.StatusCode != http.StatusOK {
			return fmt.Errorf("error listing releases %s/%s: %w", owner, repo, listErr)
		}
		if err := ghutil.CheckRateLimit(ctx, resp); err != nil {
			return err
		}

		if len(releases) == 0 {
			break
		}

		seenOld, upsertErr := upsertReleasePage(s.db, stmt, assetStmt, owner, repo, releases, latestPublishedAt)
		if upsertErr != nil {
			return upsertErr
		}

		slog.Debug("releases done", "org", owner, "repo", repo, "count", len(releases))

		if seenOld || resp.NextPage == 0 {
			break
		}
		opt.Page = resp.NextPage
	}

	return nil
}

func upsertReleasePage(db *sql.DB, stmt, assetStmt *sql.Stmt, owner, repo string, releases []*github.RepositoryRelease, latestPublishedAt string) (bool, error) {
	tx, err := db.Begin()
	if err != nil {
		return false, fmt.Errorf("error starting release tx: %w", err)
	}

	txStmt := tx.Stmt(stmt)
	txAssetStmt := tx.Stmt(assetStmt)

	seenOld := false
	for _, r := range releases {
		tag := r.GetTagName()
		name := r.GetName()
		var publishedAt string
		if r.PublishedAt != nil {
			publishedAt = r.PublishedAt.Format("2006-01-02T15:04:05Z")
		}
		pre := 0
		if r.GetPrerelease() {
			pre = 1
		}

		if latestPublishedAt != "" && publishedAt != "" && publishedAt < latestPublishedAt {
			seenOld = true
			break
		}

		if _, execErr := txStmt.Exec(
			owner, repo, tag, name, publishedAt, pre,
			name, publishedAt, pre,
		); execErr != nil {
			rollbackTransaction(tx)
			return false, fmt.Errorf("error inserting release %s: %w", tag, execErr)
		}

		for _, a := range r.Assets {
			aName := a.GetName()
			if aName == "" {
				continue
			}
			if _, execErr := txAssetStmt.Exec(
				owner, repo, tag, aName, a.GetContentType(), a.GetSize(), a.GetDownloadCount(),
				a.GetContentType(), a.GetSize(), a.GetDownloadCount(),
			); execErr != nil {
				rollbackTransaction(tx)
				return false, fmt.Errorf("error inserting release asset %s/%s: %w", tag, aName, execErr)
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return false, fmt.Errorf("error committing release tx: %w", err)
	}

	return seenOld, nil
}

func (s *Store) ImportAllReleases(ctx context.Context, token string) error {
	list, err := s.GetAllOrgRepos()
	if err != nil {
		return fmt.Errorf("error getting org/repo list: %w", err)
	}

	for _, r := range list {
		if err := s.ImportReleases(ctx, token, r.Org, r.Repo); err != nil {
			slog.Error("releases failed", "org", r.Org, "repo", r.Repo, "error", err)
		}
	}

	return nil
}

func (s *Store) GetReleaseCadence(org, repo, entity *string, months int) (*data.ReleaseCadenceSeries, error) {
	if s.db == nil {
		return nil, data.ErrDBNotInitialized
	}

	since := sinceDate(months)

	rows, err := s.db.Query(selectReleaseCadenceSQL, org, repo, since)
	if err != nil {
		return nil, fmt.Errorf("failed to query release cadence: %w", err)
	}
	defer rows.Close()

	sr := &data.ReleaseCadenceSeries{
		Months:      make([]string, 0),
		Total:       make([]int, 0),
		Stable:      make([]int, 0),
		Deployments: make([]int, 0),
	}

	for rows.Next() {
		var month string
		var total, stable int
		if scanErr := rows.Scan(&month, &total, &stable); scanErr != nil {
			return nil, fmt.Errorf("failed to scan release cadence row: %w", scanErr)
		}
		sr.Months = append(sr.Months, month)
		sr.Total = append(sr.Total, total)
		sr.Stable = append(sr.Stable, stable)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	if len(sr.Months) > 0 {
		sr.Deployments = append(sr.Deployments, sr.Total...)
		return sr, nil
	}

	fallbackRows, err := s.db.Query(selectMergedPRDeploymentsSQL, org, repo, entity, since)
	if err != nil {
		return nil, fmt.Errorf("failed to query merged PR deployments: %w", err)
	}
	defer fallbackRows.Close()

	for fallbackRows.Next() {
		var month string
		var cnt int
		if scanErr := fallbackRows.Scan(&month, &cnt); scanErr != nil {
			return nil, fmt.Errorf("failed to scan merged PR deployment row: %w", scanErr)
		}
		sr.Months = append(sr.Months, month)
		sr.Deployments = append(sr.Deployments, cnt)
	}

	if err := fallbackRows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	return sr, nil
}

func (s *Store) GetReleaseDownloads(org, repo *string, months int) (*data.ReleaseDownloadsSeries, error) { //nolint:dupl,nolintlint // different types and SQL than GetContainerActivity
	if s.db == nil {
		return nil, data.ErrDBNotInitialized
	}

	since := sinceDate(months)

	rows, err := s.db.Query(selectReleaseDownloadsSQL, org, repo, since)
	if err != nil {
		return nil, fmt.Errorf("failed to query release downloads: %w", err)
	}
	defer rows.Close()

	sr := &data.ReleaseDownloadsSeries{
		Months:    make([]string, 0),
		Downloads: make([]int, 0),
	}

	for rows.Next() {
		var month string
		var downloads int
		if err := rows.Scan(&month, &downloads); err != nil {
			return nil, fmt.Errorf("failed to scan release downloads row: %w", err)
		}
		sr.Months = append(sr.Months, month)
		sr.Downloads = append(sr.Downloads, downloads)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	return sr, nil
}

func (s *Store) GetReleaseDownloadsByTag(org, repo *string, months int) (*data.ReleaseDownloadsByTagSeries, error) {
	if s.db == nil {
		return nil, data.ErrDBNotInitialized
	}

	since := sinceDate(months)

	rows, err := s.db.Query(selectReleaseDownloadsByTagSQL, org, repo, since, org, repo, since)
	if err != nil {
		return nil, fmt.Errorf("failed to query release downloads by tag: %w", err)
	}
	defer rows.Close()

	sr := &data.ReleaseDownloadsByTagSeries{
		Tags:      make([]string, 0),
		Downloads: make([]int, 0),
	}

	for rows.Next() {
		var tag string
		var downloads int
		if err := rows.Scan(&tag, &downloads); err != nil {
			return nil, fmt.Errorf("failed to scan release downloads by tag row: %w", err)
		}
		sr.Tags = append(sr.Tags, tag)
		sr.Downloads = append(sr.Downloads, downloads)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	return sr, nil
}
