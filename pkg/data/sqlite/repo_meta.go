package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/go-github/v83/github"
	"github.com/mchmarny/devpulse/pkg/data"
	"github.com/mchmarny/devpulse/pkg/net"
)

const (
	upsertRepoMetaSQL = `INSERT INTO repo_meta (org, repo, stars, forks, open_issues, language, license, archived, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(org, repo) DO UPDATE SET
			stars = ?, forks = ?, open_issues = ?, language = ?, license = ?, archived = ?, updated_at = ?
	`

	selectRepoMetaUpdatedAtSQL = `SELECT COALESCE(updated_at, '')
		FROM repo_meta
		WHERE org = ? AND repo = ?
	`

	selectRepoMetaSQL = `SELECT org, repo, stars, forks, open_issues, language, license, archived, updated_at
		FROM repo_meta
		WHERE org = COALESCE(?, org)
		  AND repo = COALESCE(?, repo)
		ORDER BY org, repo
	`
)

func (s *Store) ImportRepoMeta(ctx context.Context, token, owner, repo string) error {
	var lastUpdated string
	if scanErr := s.db.QueryRow(selectRepoMetaUpdatedAtSQL, owner, repo).Scan(&lastUpdated); scanErr != nil && scanErr != sql.ErrNoRows {
		return fmt.Errorf("querying repo meta updated_at for %s/%s: %w", owner, repo, scanErr)
	}
	if lastUpdated != "" {
		if t, parseErr := time.Parse("2006-01-02T15:04:05Z", lastUpdated); parseErr == nil {
			if time.Since(t) < 24*time.Hour {
				slog.Debug("metadata fresh, skipping", "org", owner, "repo", repo, "updated_at", lastUpdated)
				return nil
			}
		}
	}

	client := github.NewClient(net.GetOAuthClient(ctx, token))

	r, resp, err := client.Repositories.Get(ctx, owner, repo)
	if err != nil || resp.StatusCode != http.StatusOK {
		return fmt.Errorf("error getting repo %s/%s: %w", owner, repo, err)
	}
	checkRateLimit(resp)

	now := time.Now().UTC().Format("2006-01-02T15:04:05Z")
	lang := r.GetLanguage()
	var license string
	if r.License != nil {
		license = r.License.GetSPDXID()
	}
	archived := 0
	if r.GetArchived() {
		archived = 1
	}

	_, err = s.db.Exec(upsertRepoMetaSQL,
		owner, repo, r.GetStargazersCount(), r.GetForksCount(), r.GetOpenIssuesCount(),
		lang, license, archived, now,
		r.GetStargazersCount(), r.GetForksCount(), r.GetOpenIssuesCount(),
		lang, license, archived, now,
	)
	if err != nil {
		return fmt.Errorf("error upserting repo meta %s/%s: %w", owner, repo, err)
	}

	today := time.Now().UTC().Format("2006-01-02")
	_, err = s.db.Exec(upsertRepoMetricHistorySQL,
		owner, repo, today, r.GetStargazersCount(), r.GetForksCount(),
		r.GetStargazersCount(), r.GetForksCount(),
	)
	if err != nil {
		return fmt.Errorf("error upserting repo metric history %s/%s: %w", owner, repo, err)
	}

	slog.Debug("metadata done", "org", owner, "repo", repo)
	return nil
}

func (s *Store) ImportAllRepoMeta(ctx context.Context, token string) error {
	list, err := s.GetAllOrgRepos()
	if err != nil {
		return fmt.Errorf("error getting org/repo list: %w", err)
	}

	for _, r := range list {
		if err := s.ImportRepoMeta(ctx, token, r.Org, r.Repo); err != nil {
			slog.Error("metadata failed", "org", r.Org, "repo", r.Repo, "error", err)
		}
	}

	return nil
}

func (s *Store) GetRepoMetas(org, repo *string) ([]*data.RepoMeta, error) {
	if s.db == nil {
		return nil, data.ErrDBNotInitialized
	}

	rows, err := s.db.Query(selectRepoMetaSQL, org, repo)
	if err != nil {
		return nil, fmt.Errorf("failed to query repo meta: %w", err)
	}
	defer rows.Close()

	list := make([]*data.RepoMeta, 0)
	for rows.Next() {
		m := &data.RepoMeta{}
		var archived int
		if err := rows.Scan(&m.Org, &m.Repo, &m.Stars, &m.Forks, &m.OpenIssues,
			&m.Language, &m.License, &archived, &m.UpdatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan repo meta row: %w", err)
		}
		m.Archived = archived != 0
		list = append(list, m)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	return list, nil
}
