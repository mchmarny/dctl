package data

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/go-github/v83/github"
	"github.com/mchmarny/dctl/pkg/net"
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
)

type ReleaseCadenceSeries struct {
	Months []string `json:"months"`
	Total  []int    `json:"total"`
	Stable []int    `json:"stable"`
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
				publishedAt = r.PublishedAt.Time.Format("2006-01-02T15:04:05Z")
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
	if err != nil && err != sql.ErrNoRows {
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
