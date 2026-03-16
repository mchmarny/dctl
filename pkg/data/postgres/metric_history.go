package postgres

import (
	"context"
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
	// upsertRepoMetricHistorySQL: 7 params
	upsertRepoMetricHistorySQL = `INSERT INTO repo_metric_history (org, repo, date, stars, forks)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT(org, repo, date) DO UPDATE SET
			stars = $6, forks = $7
	`

	// selectRepoMetricHistorySQL: $1=org, $2=repo
	selectRepoMetricHistorySQL = `SELECT org, repo, date, stars, forks
		FROM repo_metric_history
		WHERE org = COALESCE($1, org)
		  AND repo = COALESCE($2, repo)
		ORDER BY org, repo, date
	`

	backfillDays = 30
)

func (s *Store) GetRepoMetricHistory(org, repo *string) ([]*data.RepoMetricHistory, error) {
	if s.db == nil {
		return nil, data.ErrDBNotInitialized
	}

	rows, err := s.db.Query(selectRepoMetricHistorySQL, org, repo)
	if err != nil {
		return nil, fmt.Errorf("failed to query repo metric history: %w", err)
	}
	defer rows.Close()

	list := make([]*data.RepoMetricHistory, 0)
	for rows.Next() {
		m := &data.RepoMetricHistory{}
		if err := rows.Scan(&m.Org, &m.Repo, &m.Date, &m.Stars, &m.Forks); err != nil {
			return nil, fmt.Errorf("failed to scan repo metric history row: %w", err)
		}
		list = append(list, m)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	return list, nil
}

func (s *Store) ImportRepoMetricHistory(ctx context.Context, token, owner, repo string) error {
	client := github.NewClient(net.GetOAuthClient(ctx, token))

	r, resp, err := client.Repositories.Get(ctx, owner, repo)
	if err != nil || resp.StatusCode != http.StatusOK {
		return fmt.Errorf("error getting repo %s/%s: %w", owner, repo, err)
	}
	ghutil.CheckRateLimit(resp)

	currentStars := r.GetStargazersCount()
	currentForks := r.GetForksCount()
	cutoff := time.Now().AddDate(0, 0, -backfillDays).UTC()

	starsByDay, err := countRecentStarsByDay(ctx, client, owner, repo, cutoff)
	if err != nil {
		return fmt.Errorf("error counting stars: %w", err)
	}

	forksByDay, err := countRecentForksByDay(ctx, client, owner, repo, cutoff)
	if err != nil {
		return fmt.Errorf("error counting forks: %w", err)
	}

	history := buildDailyTotals(currentStars, currentForks, starsByDay, forksByDay, backfillDays)

	return s.upsertMetricHistory(owner, repo, history)
}

func countRecentStarsByDay(ctx context.Context, client *github.Client, owner, repo string, cutoff time.Time) (map[string]int, error) {
	counts := make(map[string]int)

	_, resp, err := client.Activity.ListStargazers(ctx, owner, repo, &github.ListOptions{PerPage: 100, Page: 1})
	if err != nil {
		return nil, fmt.Errorf("error listing stargazers: %w", err)
	}
	ghutil.CheckRateLimit(resp)

	lastPage := resp.LastPage
	if lastPage == 0 {
		lastPage = 1
	}

	for page := lastPage; page >= 1; page-- {
		stargazers, resp, err := client.Activity.ListStargazers(ctx, owner, repo, &github.ListOptions{PerPage: 100, Page: page})
		if err != nil {
			return nil, fmt.Errorf("error listing stargazers page %d: %w", page, err)
		}
		ghutil.CheckRateLimit(resp)

		if len(stargazers) == 0 {
			break
		}

		allOlder := true
		for _, sg := range stargazers {
			if sg.StarredAt == nil {
				continue
			}
			t := sg.StarredAt.Time
			if t.Before(cutoff) {
				continue
			}
			allOlder = false
			day := t.Format("2006-01-02")
			counts[day]++
		}

		if allOlder {
			break
		}
	}

	return counts, nil
}

func countRecentForksByDay(ctx context.Context, client *github.Client, owner, repo string, cutoff time.Time) (map[string]int, error) {
	counts := make(map[string]int)
	opt := &github.RepositoryListForksOptions{
		Sort:        "newest",
		ListOptions: github.ListOptions{PerPage: 100, Page: 1},
	}

	for {
		forks, resp, err := client.Repositories.ListForks(ctx, owner, repo, opt)
		if err != nil {
			return nil, fmt.Errorf("error listing forks: %w", err)
		}
		ghutil.CheckRateLimit(resp)

		if len(forks) == 0 {
			break
		}

		allOlder := true
		for _, f := range forks {
			t := f.GetCreatedAt().Time
			if t.Before(cutoff) {
				continue
			}
			allOlder = false
			day := t.Format("2006-01-02")
			counts[day]++
		}

		if allOlder {
			break
		}

		if resp.NextPage == 0 {
			break
		}
		opt.ListOptions.Page = resp.NextPage
	}

	return counts, nil
}

func buildDailyTotals(currentStars, currentForks int, starsByDay, forksByDay map[string]int, days int) []*data.RepoMetricHistory {
	now := time.Now().UTC()
	dates := make([]string, days+1)
	for i := 0; i <= days; i++ {
		dates[days-i] = now.AddDate(0, 0, -i).Format("2006-01-02")
	}

	result := make([]*data.RepoMetricHistory, len(dates))
	stars := currentStars
	forks := currentForks

	for i := len(dates) - 1; i >= 0; i-- {
		result[i] = &data.RepoMetricHistory{
			Date:  dates[i],
			Stars: stars,
			Forks: forks,
		}
		stars -= starsByDay[dates[i]]
		forks -= forksByDay[dates[i]]
		if stars < 0 {
			stars = 0
		}
		if forks < 0 {
			forks = 0
		}
	}

	return result
}

func (s *Store) upsertMetricHistory(owner, repo string, history []*data.RepoMetricHistory) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	stmt, err := tx.Prepare(upsertRepoMetricHistorySQL)
	if err != nil {
		rollbackTransaction(tx)
		return fmt.Errorf("failed to prepare metric history statement: %w", err)
	}

	for _, h := range history {
		if _, err := stmt.Exec(owner, repo, h.Date, h.Stars, h.Forks, h.Stars, h.Forks); err != nil {
			rollbackTransaction(tx)
			return fmt.Errorf("failed to upsert metric history %s: %w", h.Date, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit metric history: %w", err)
	}

	slog.Debug("metric history done", "org", owner, "repo", repo, "days", len(history))
	return nil
}

func (s *Store) ImportAllRepoMetricHistory(ctx context.Context, token string) error {
	list, err := s.GetAllOrgRepos()
	if err != nil {
		return fmt.Errorf("error getting org/repo list: %w", err)
	}

	for _, r := range list {
		if err := s.ImportRepoMetricHistory(ctx, token, r.Org, r.Repo); err != nil {
			slog.Error("metric history failed", "org", r.Org, "repo", r.Repo, "error", err)
		}
	}

	return nil
}
