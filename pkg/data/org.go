package data

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"log/slog"

	"github.com/google/go-github/v83/github"
)

const (
	selectOrgEntityPercent = `SELECT
			entity,
			ROUND(100.0 * events / (SUM(events) OVER ())) AS percent 
		FROM (  
			SELECT 
				d.entity,
				COUNT(*) as events
			FROM developer d
			JOIN event e ON d.username = e.username
			WHERE (d.entity <> '' OR d.entity is null) 
			AND e.date >= ?
			AND d.entity = COALESCE(?, d.entity)
			AND e.org = COALESCE(?, e.org)
			AND e.repo = COALESCE(?, e.repo)
			AND d.entity NOT IN (%s)
			GROUP BY d.entity
		) dt 
		ORDER BY 2 DESC 
	`

	selectDeveloperPercent = `SELECT
			username,
			ROUND(100.0 * events / (SUM(events) OVER ())) AS percent
		FROM (
			SELECT
				d.username,
				COUNT(*) as events
			FROM developer d
			JOIN event e ON d.username = e.username
			WHERE e.date >= ?
			AND d.entity = COALESCE(?, d.entity)
			AND e.org = COALESCE(?, e.org)
			AND e.repo = COALESCE(?, e.repo)
			AND d.username NOT IN (%s)
			AND d.username NOT LIKE '%%[bot]'
			GROUP BY d.username
		) dt
		ORDER BY 2 DESC
	`

	selectOrgLike = `SELECT org, COUNT(DISTINCT repo) as repo_count, COUNT(*) as event_count  
		FROM event 
		WHERE org like ?
		GROUP BY org
		ORDER BY org DESC
		LIMIT ?
	`

	selectAllOrgRepos = `SELECT DISTINCT org, repo FROM event order by 1, 1`
)

type Org struct {
	URL         string `json:"url,omitempty"`
	Name        string `json:"name,omitempty"`
	Company     string `json:"company,omitempty"`
	Description string `json:"description,omitempty"`
}

type OrgRepoItem struct {
	Org  string `json:"org,omitempty"`
	Repo string `json:"repo,omitempty"`
}

func mapOrg(r *github.Organization) *Org {
	return &Org{
		Name:        trim(r.Login),
		Company:     trim(r.Company),
		Description: trim(r.Description),
		URL:         trim(r.URL),
	}
}

// GetAllOrgRepos returns a list of repo percentages for the given organization.
func GetAllOrgRepos(db *sql.DB) ([]*OrgRepoItem, error) {
	if db == nil {
		return nil, errDBNotInitialized
	}

	stmt, err := db.Prepare(selectAllOrgRepos)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare developer percentages statement: %w", err)
	}

	rows, err := stmt.Query()
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("failed to execute select statement: %w", err)
	}
	defer rows.Close()

	list := make([]*OrgRepoItem, 0)
	for rows.Next() {
		e := &OrgRepoItem{}
		if err := rows.Scan(&e.Org, &e.Repo); err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}
		list = append(list, e)
	}

	return list, nil
}

func getPercentages(db *sql.DB, sqlStr string, entity, org, repo *string, ex []string, months int) ([]*CountedItem, error) {
	if db == nil {
		return nil, errDBNotInitialized
	}

	since := time.Now().UTC().AddDate(0, -months, 0).Format("2006-01-02")

	params := make([]string, len(ex))
	qArgs := []interface{}{since, entity, org, repo}

	for i, v := range ex {
		params[i] = "?"
		qArgs = append(qArgs, v)
	}

	stmt, err := db.Prepare(fmt.Sprintf(sqlStr, strings.Join(params, ",")))
	if err != nil {
		return nil, fmt.Errorf("failed to prepare percentages statement: %w", err)
	}

	rows, err := stmt.Query(qArgs...)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("failed to execute select statement: %w", err)
	}
	defer rows.Close()

	list := make([]*CountedItem, 0)
	for rows.Next() {
		e := &CountedItem{}
		if err := rows.Scan(&e.Name, &e.Count); err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}
		list = append(list, e)
	}

	return list, nil
}

// GetOrgRepoPercentages returns a list of repo percentages for the given organization.
func GetDeveloperPercentages(db *sql.DB, entity, org, repo *string, ex []string, months int) ([]*CountedItem, error) {
	return getPercentages(db, selectDeveloperPercent, entity, org, repo, ex, months)
}

// GetEntityPercentages returns a list of entity percentages for the given repository.
func GetEntityPercentages(db *sql.DB, entity, org, repo *string, ex []string, months int) ([]*CountedItem, error) {
	return getPercentages(db, selectOrgEntityPercent, entity, org, repo, ex, months)
}

// GetOrgLike returns a list of orgs and repos that match the given pattern.
func GetOrgLike(db *sql.DB, query string, limit int) ([]*ListItem, error) {
	if db == nil {
		return nil, errDBNotInitialized
	}

	if query == "" {
		return nil, errors.New("query is required")
	}

	stmt, err := db.Prepare(selectOrgLike)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare org like statement: %w", err)
	}

	query = fmt.Sprintf("%%%s%%", query)
	rows, err := stmt.Query(query, limit)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("failed to execute select statement: %w", err)
	}
	defer rows.Close()

	list := make([]*ListItem, 0)
	for rows.Next() {
		e := &ListItem{}
		var repoCount, eventCount int
		if err := rows.Scan(&e.Value, &repoCount, &eventCount); err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}
		e.Text = fmt.Sprintf("%s (%d repos, %d events)", e.Value, repoCount, eventCount)
		list = append(list, e)
	}

	return list, nil
}

func GetUserOrgs(ctx context.Context, client *http.Client, username string, limit int) ([]*Org, error) {
	if username == "" {
		return nil, errors.New("username is required")
	}

	slog.Debug("listing repositories", "username", username, "limit", limit)

	opt := &github.ListOptions{}
	if limit > 0 {
		opt.PerPage = limit
	}

	items, _, err := github.NewClient(client).Organizations.List(ctx, username, opt)
	if err != nil {
		return nil, fmt.Errorf("failed to list repositories for: %s: %w", username, err)
	}

	list := make([]*Org, 0)
	for _, r := range items {
		slog.Debug("org", "value", r)
		list = append(list, mapOrg(r))
	}

	return list, nil
}
