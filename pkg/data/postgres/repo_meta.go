package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/go-github/v83/github"
	"github.com/mchmarny/devpulse/pkg/data"
	"github.com/mchmarny/devpulse/pkg/data/ghutil"
	"github.com/mchmarny/devpulse/pkg/net"
)

const (
	upsertRepoMetaSQL = `INSERT INTO repo_meta (org, repo, stars, forks, open_issues,
		language, license, archived,
		has_coc, has_contributing, has_readme, has_issue_template, has_pr_template, community_health_pct,
		updated_at, last_import_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)
		ON CONFLICT(org, repo) DO UPDATE SET
			stars = $17, forks = $18, open_issues = $19, language = $20, license = $21, archived = $22,
			has_coc = $23, has_contributing = $24, has_readme = $25, has_issue_template = $26, has_pr_template = $27, community_health_pct = $28,
			updated_at = $29, last_import_at = $30
	`

	updateLastImportAtSQL = `UPDATE repo_meta SET last_import_at = $1 WHERE org = $2 AND repo = $3`

	// selectRepoMetaUpdatedAtSQL: $1=org, $2=repo
	selectRepoMetaUpdatedAtSQL = `SELECT COALESCE(updated_at, '')
		FROM repo_meta
		WHERE org = $1 AND repo = $2
	`

	// selectRepoMetaSQL: $1=org, $2=repo
	selectRepoMetaSQL = `SELECT org, repo, stars, forks, open_issues, language, license, archived,
			has_coc, has_contributing, has_readme, has_issue_template, has_pr_template, community_health_pct,
			updated_at
		FROM repo_meta
		WHERE org = COALESCE($1, org)
		  AND repo = COALESCE($2, repo)
		ORDER BY org, repo
	`

	// selectRepoOverviewSQL: $1=since, $2=org
	selectRepoOverviewSQL = `SELECT
			rm.org, rm.repo, rm.stars, rm.forks, rm.open_issues,
			COUNT(e.type), COUNT(DISTINCT e.username),
			COUNT(DISTINCT CASE WHEN d.reputation IS NOT NULL THEN e.username END),
			rm.language, rm.license, rm.archived,
			rm.last_import_at
		FROM repo_meta rm
		LEFT JOIN event e ON rm.org = e.org AND rm.repo = e.repo AND e.date >= $1
		LEFT JOIN developer d ON e.username = d.username
		WHERE rm.org = COALESCE($2, rm.org)
		GROUP BY rm.org, rm.repo, rm.stars, rm.forks, rm.open_issues,
			rm.language, rm.license, rm.archived, rm.last_import_at
		ORDER BY rm.org, rm.repo
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
				now := time.Now().UTC().Format("2006-01-02T15:04:05Z")
				_, _ = s.db.Exec(updateLastImportAtSQL, now, owner, repo)
				return nil
			}
		}
	}

	client := github.NewClient(net.GetOAuthClient(ctx, token))

	r, resp, err := client.Repositories.Get(ctx, owner, repo)
	if err != nil || resp.StatusCode != http.StatusOK {
		return fmt.Errorf("error getting repo %s/%s: %w", owner, repo, err)
	}
	if rlErr := ghutil.CheckRateLimit(ctx, resp); rlErr != nil {
		return rlErr
	}

	cp, rlErr := fetchCommunityProfile(ctx, client, owner, repo)
	if rlErr != nil {
		return rlErr
	}

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
		lang, license, archived, cp.coc, cp.contributing, cp.readme, cp.issueTmpl, cp.prTmpl, cp.healthPct, now, now,
		r.GetStargazersCount(), r.GetForksCount(), r.GetOpenIssuesCount(),
		lang, license, archived, cp.coc, cp.contributing, cp.readme, cp.issueTmpl, cp.prTmpl, cp.healthPct, now, now,
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

type communityProfile struct {
	coc, contributing, readme, issueTmpl, prTmpl, healthPct int
}

func fetchCommunityProfile(ctx context.Context, client *github.Client, owner, repo string) (communityProfile, error) {
	var cp communityProfile

	profile, resp, err := client.Repositories.GetCommunityHealthMetrics(ctx, owner, repo)
	if err != nil {
		slog.Warn("failed to get community profile", "org", owner, "repo", repo, "error", err)
		return cp, nil
	}
	if resp != nil {
		if rlErr := ghutil.CheckRateLimit(ctx, resp); rlErr != nil {
			return cp, rlErr
		}
	}

	if profile != nil {
		if profile.Files != nil {
			if profile.Files.CodeOfConduct != nil {
				cp.coc = 1
			}
			if profile.Files.Contributing != nil {
				cp.contributing = 1
			}
			if profile.Files.Readme != nil {
				cp.readme = 1
			}
			if profile.Files.IssueTemplate != nil {
				cp.issueTmpl = 1
			}
			if profile.Files.PullRequestTemplate != nil {
				cp.prTmpl = 1
			}
		}
		if profile.HealthPercentage != nil {
			cp.healthPct = *profile.HealthPercentage
		}
	}

	return cp, nil
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
		var archived, hasCoc, hasContrib, hasReadme, hasIssueTmpl, hasPRTmpl int
		if err := rows.Scan(&m.Org, &m.Repo, &m.Stars, &m.Forks, &m.OpenIssues,
			&m.Language, &m.License, &archived,
			&hasCoc, &hasContrib, &hasReadme, &hasIssueTmpl, &hasPRTmpl, &m.CommunityHealthPct,
			&m.UpdatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan repo meta row: %w", err)
		}
		m.Archived = archived != 0
		m.HasCoC = hasCoc != 0
		m.HasContributing = hasContrib != 0
		m.HasReadme = hasReadme != 0
		m.HasIssueTemplate = hasIssueTmpl != 0
		m.HasPRTemplate = hasPRTmpl != 0
		list = append(list, m)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	return list, nil
}

func (s *Store) GetRepoOverview(org *string, months int) ([]*data.RepoOverview, error) {
	if s.db == nil {
		return nil, data.ErrDBNotInitialized
	}

	since := sinceDate(months)

	rows, err := s.db.Query(selectRepoOverviewSQL, since, org)
	if err != nil {
		return nil, fmt.Errorf("failed to query repo overview: %w", err)
	}
	defer rows.Close()

	list := make([]*data.RepoOverview, 0)
	for rows.Next() {
		r := &data.RepoOverview{}
		var archived int
		if err := rows.Scan(&r.Org, &r.Repo, &r.Stars, &r.Forks, &r.OpenIssues,
			&r.Events, &r.Contributors, &r.Scored,
			&r.Language, &r.License, &archived, &r.LastImport); err != nil {
			return nil, fmt.Errorf("failed to scan repo overview row: %w", err)
		}
		r.Archived = archived != 0
		list = append(list, r)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating repo overview rows: %w", err)
	}

	return list, nil
}
