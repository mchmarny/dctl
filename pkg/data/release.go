package data

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/go-github/v83/github"
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

type ReleaseCadenceSeries struct {
	Months []string `json:"months" yaml:"months"`
	Total  []int    `json:"total" yaml:"total"`
	Stable []int    `json:"stable" yaml:"stable"`
}

type ReleaseDownloadsSeries struct {
	Months    []string `json:"months" yaml:"months"`
	Downloads []int    `json:"downloads" yaml:"downloads"`
}

type ReleaseDownloadsByTagSeries struct {
	Tags      []string `json:"tags" yaml:"tags"`
	Downloads []int    `json:"downloads" yaml:"downloads"`
}

func ImportReleases(dbPath, token, owner, repo string) error {
	ctx := context.Background()
	client := github.NewClient(net.GetOAuthClient(ctx, token))

	db, err := GetDB(dbPath)
	if err != nil {
		return fmt.Errorf("error getting DB: %w", err)
	}
	defer db.Close()

	stmt, err := db.Prepare(insertReleaseSQL)
	if err != nil {
		return fmt.Errorf("error preparing release insert: %w", err)
	}

	assetStmt, err := db.Prepare(insertReleaseAssetSQL)
	if err != nil {
		return fmt.Errorf("error preparing release asset insert: %w", err)
	}

	opt := &github.ListOptions{PerPage: pageSizeDefault, Page: 1}

	for {
		releases, resp, err := client.Repositories.ListReleases(ctx, owner, repo, opt)
		if err != nil || resp.StatusCode != http.StatusOK {
			return fmt.Errorf("error listing releases %s/%s: %w", owner, repo, err)
		}
		checkRateLimit(resp)

		if len(releases) == 0 {
			break
		}

		tx, err := db.Begin()
		if err != nil {
			return fmt.Errorf("error starting release tx: %w", err)
		}

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

			if _, err := tx.Stmt(stmt).Exec(
				owner, repo, tag, name, publishedAt, pre,
				name, publishedAt, pre,
			); err != nil {
				rollbackTransaction(tx)
				return fmt.Errorf("error inserting release %s: %w", tag, err)
			}

			for _, a := range r.Assets {
				aName := a.GetName()
				if aName == "" {
					continue
				}
				ct := a.GetContentType()
				sz := a.GetSize()
				dc := a.GetDownloadCount()
				if _, err := tx.Stmt(assetStmt).Exec(
					owner, repo, tag, aName, ct, sz, dc,
					ct, sz, dc,
				); err != nil {
					rollbackTransaction(tx)
					return fmt.Errorf("error inserting release asset %s/%s: %w", tag, aName, err)
				}
			}
		}

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("error committing release tx: %w", err)
		}

		slog.Debug("imported releases", "org", owner, "repo", repo, "count", len(releases))

		if resp.NextPage == 0 {
			break
		}
		opt.Page = resp.NextPage
	}

	return nil
}

func ImportAllReleases(dbPath, token string) error {
	db, err := GetDB(dbPath)
	if err != nil {
		return fmt.Errorf("error getting DB: %w", err)
	}
	defer db.Close()

	list, err := GetAllOrgRepos(db)
	if err != nil {
		return fmt.Errorf("error getting org/repo list: %w", err)
	}

	for _, r := range list {
		if err := ImportReleases(dbPath, token, r.Org, r.Repo); err != nil {
			slog.Error("error importing releases", "org", r.Org, "repo", r.Repo, "error", err)
		}
	}

	return nil
}

func GetReleaseCadence(db *sql.DB, org, repo *string, months int) (*ReleaseCadenceSeries, error) {
	if db == nil {
		return nil, errDBNotInitialized
	}

	since := time.Now().UTC().AddDate(0, -months, 0).Format("2006-01-02")

	rows, err := db.Query(selectReleaseCadenceSQL, org, repo, since)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("failed to query release cadence: %w", err)
	}
	defer rows.Close()

	s := &ReleaseCadenceSeries{
		Months: make([]string, 0),
		Total:  make([]int, 0),
		Stable: make([]int, 0),
	}

	for rows.Next() {
		var month string
		var total, stable int
		if err := rows.Scan(&month, &total, &stable); err != nil {
			return nil, fmt.Errorf("failed to scan release cadence row: %w", err)
		}
		s.Months = append(s.Months, month)
		s.Total = append(s.Total, total)
		s.Stable = append(s.Stable, stable)
	}

	return s, nil
}

func GetReleaseDownloads(db *sql.DB, org, repo *string, months int) (*ReleaseDownloadsSeries, error) {
	if db == nil {
		return nil, errDBNotInitialized
	}

	since := time.Now().UTC().AddDate(0, -months, 0).Format("2006-01-02")

	rows, err := db.Query(selectReleaseDownloadsSQL, org, repo, since)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("failed to query release downloads: %w", err)
	}
	defer rows.Close()

	s := &ReleaseDownloadsSeries{
		Months:    make([]string, 0),
		Downloads: make([]int, 0),
	}

	for rows.Next() {
		var month string
		var downloads int
		if err := rows.Scan(&month, &downloads); err != nil {
			return nil, fmt.Errorf("failed to scan release downloads row: %w", err)
		}
		s.Months = append(s.Months, month)
		s.Downloads = append(s.Downloads, downloads)
	}

	return s, nil
}

func GetReleaseDownloadsByTag(db *sql.DB, org, repo *string, months int) (*ReleaseDownloadsByTagSeries, error) {
	if db == nil {
		return nil, errDBNotInitialized
	}

	since := time.Now().UTC().AddDate(0, -months, 0).Format("2006-01-02")

	rows, err := db.Query(selectReleaseDownloadsByTagSQL, org, repo, since, org, repo, since)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("failed to query release downloads by tag: %w", err)
	}
	defer rows.Close()

	s := &ReleaseDownloadsByTagSeries{
		Tags:      make([]string, 0),
		Downloads: make([]int, 0),
	}

	for rows.Next() {
		var tag string
		var downloads int
		if err := rows.Scan(&tag, &downloads); err != nil {
			return nil, fmt.Errorf("failed to scan release downloads by tag row: %w", err)
		}
		s.Tags = append(s.Tags, tag)
		s.Downloads = append(s.Downloads, downloads)
	}

	return s, nil
}
