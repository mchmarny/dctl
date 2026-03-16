package data

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/go-github/v83/github"
	"github.com/mchmarny/devpulse/pkg/net"
)

const (
	upsertRepoMetricHistorySQL = `INSERT INTO repo_metric_history (org, repo, date, stars, forks)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(org, repo, date) DO UPDATE SET
			stars = ?, forks = ?
	`

	selectRepoMetricHistorySQL = `SELECT org, repo, date, stars, forks
		FROM repo_metric_history
		WHERE org = COALESCE(?, org)
		  AND repo = COALESCE(?, repo)
		ORDER BY org, repo, date
	`
)

func GetRepoMetricHistory(db *sql.DB, org, repo *string) ([]*RepoMetricHistory, error) {
	if db == nil {
		return nil, ErrDBNotInitialized
	}

	rows, err := db.Query(selectRepoMetricHistorySQL, org, repo)
	if err != nil {
		return nil, fmt.Errorf("failed to query repo metric history: %w", err)
	}
	defer rows.Close()

	list := make([]*RepoMetricHistory, 0)
	for rows.Next() {
		m := &RepoMetricHistory{}
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

const backfillDays = 30

// ImportRepoMetricHistory backfills daily star and fork counts for the last 30 days.
func ImportRepoMetricHistory(ctx context.Context, dbPath, token, owner, repo string) error {
	client := github.NewClient(net.GetOAuthClient(ctx, token))

	r, resp, err := client.Repositories.Get(ctx, owner, repo)
	if err != nil || resp.StatusCode != http.StatusOK {
		return fmt.Errorf("error getting repo %s/%s: %w", owner, repo, err)
	}
	checkRateLimit(resp)

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

	db, err := GetDB(dbPath)
	if err != nil {
		return fmt.Errorf("error getting DB: %w", err)
	}
	defer db.Close()

	return upsertMetricHistory(db, owner, repo, history)
}

func countRecentStarsByDay(ctx context.Context, client *github.Client, owner, repo string, cutoff time.Time) (map[string]int, error) {
	counts := make(map[string]int)

	// ListStargazers returns oldest first. Find total pages, then page backward.
	_, resp, err := client.Activity.ListStargazers(ctx, owner, repo, &github.ListOptions{PerPage: 100, Page: 1})
	if err != nil {
		return nil, fmt.Errorf("error listing stargazers: %w", err)
	}
	checkRateLimit(resp)

	lastPage := resp.LastPage
	if lastPage == 0 {
		lastPage = 1
	}

	for page := lastPage; page >= 1; page-- {
		stargazers, resp, err := client.Activity.ListStargazers(ctx, owner, repo, &github.ListOptions{PerPage: 100, Page: page})
		if err != nil {
			return nil, fmt.Errorf("error listing stargazers page %d: %w", page, err)
		}
		checkRateLimit(resp)

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
		checkRateLimit(resp)

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

func buildDailyTotals(currentStars, currentForks int, starsByDay, forksByDay map[string]int, days int) []*RepoMetricHistory {
	now := time.Now().UTC()
	dates := make([]string, days+1)
	for i := 0; i <= days; i++ {
		dates[days-i] = now.AddDate(0, 0, -i).Format("2006-01-02")
	}

	result := make([]*RepoMetricHistory, len(dates))
	stars := currentStars
	forks := currentForks

	for i := len(dates) - 1; i >= 0; i-- {
		result[i] = &RepoMetricHistory{
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

func upsertMetricHistory(db *sql.DB, owner, repo string, history []*RepoMetricHistory) error {
	tx, err := db.Begin()
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

// ImportAllRepoMetricHistory backfills metric history for all known org/repo pairs.
func ImportAllRepoMetricHistory(ctx context.Context, dbPath, token string) error {
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
		if err := ImportRepoMetricHistory(ctx, dbPath, token, r.Org, r.Repo); err != nil {
			slog.Error("metric history failed", "org", r.Org, "repo", r.Repo, "error", err)
		}
	}

	return nil
}
