package data

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"time"

	"github.com/google/go-github/v44/github"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
)

const (
	selectOrgEntityPercent = `SELECT
			entity,
			ROUND(100.0 * events / (SUM(events) OVER ())) AS percent 
		FROM (  
			SELECT 
				d.entity,
				COUNT(distinct e.id) as events
			FROM developer d
			JOIN event e ON d.username = e.username
			WHERE (d.entity <> '' OR d.entity is null) 
			AND e.event_date >= ?
			AND d.entity = COALESCE(?, d.entity)
			AND e.org = COALESCE(?, e.org)
			AND e.repo = COALESCE(?, e.repo)
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
				COUNT(distinct e.id) as events
			FROM developer d
			JOIN event e ON d.username = e.username
			WHERE e.event_date >= ?
			AND d.entity = COALESCE(?, d.entity)
			AND e.org = COALESCE(?, e.org)
			AND e.repo = COALESCE(?, e.repo)
			GROUP BY d.username
		) dt 
		ORDER BY 2 DESC 
	`

	selectOrgLike = `SELECT org, COUNT(DISTINCT repo) as repo_count, COUNT(DISTINCT id) as event_count  
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
		return nil, errors.Wrap(err, "failed to prepare developer percentages statement")
	}

	rows, err := stmt.Query()
	if err != nil && err != sql.ErrNoRows {
		return nil, errors.Wrap(err, "failed to execute select statement")
	}
	defer rows.Close()

	list := make([]*OrgRepoItem, 0)
	for rows.Next() {
		e := &OrgRepoItem{}
		if err := rows.Scan(&e.Org, &e.Repo); err != nil {
			return nil, errors.Wrap(err, "failed to scan row")
		}
		list = append(list, e)
	}

	return list, nil
}

// GetOrgRepoPercentages returns a list of repo percentages for the given organization.
func GetDeveloperPercentages(db *sql.DB, entity, org, repo *string, months int) ([]*CountedItem, error) {
	if db == nil {
		return nil, errDBNotInitialized
	}

	since := time.Now().UTC().AddDate(0, -months, 0).Format("2006-01-02")

	stmt, err := db.Prepare(selectDeveloperPercent)
	if err != nil {
		return nil, errors.Wrap(err, "failed to prepare developer percentages statement")
	}

	rows, err := stmt.Query(since, entity, org, repo)
	if err != nil && err != sql.ErrNoRows {
		return nil, errors.Wrap(err, "failed to execute select statement")
	}
	defer rows.Close()

	list := make([]*CountedItem, 0)
	for rows.Next() {
		e := &CountedItem{}
		if err := rows.Scan(&e.Name, &e.Count); err != nil {
			return nil, errors.Wrap(err, "failed to scan row")
		}
		list = append(list, e)
	}

	return list, nil
}

// GetEntityPercentages returns a list of entity percentages for the given repository.
func GetEntityPercentages(db *sql.DB, entity, org, repo *string, months int) ([]*CountedItem, error) {
	if db == nil {
		return nil, errDBNotInitialized
	}

	since := time.Now().UTC().AddDate(0, -months, 0).Format("2006-01-02")

	stmt, err := db.Prepare(selectOrgEntityPercent)
	if err != nil {
		return nil, errors.Wrap(err, "failed to prepare repo entity percentages statement")
	}

	rows, err := stmt.Query(since, entity, org, repo)
	if err != nil && err != sql.ErrNoRows {
		return nil, errors.Wrap(err, "failed to execute select statement")
	}
	defer rows.Close()

	list := make([]*CountedItem, 0)
	for rows.Next() {
		e := &CountedItem{}
		if err := rows.Scan(&e.Name, &e.Count); err != nil {
			return nil, errors.Wrap(err, "failed to scan row")
		}
		list = append(list, e)
	}

	return list, nil
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
		return nil, errors.Wrap(err, "failed to prepare org like statement")
	}

	query = fmt.Sprintf("%%%s%%", query)
	rows, err := stmt.Query(query, limit)
	if err != nil && err != sql.ErrNoRows {
		return nil, errors.Wrap(err, "failed to execute select statement")
	}
	defer rows.Close()

	list := make([]*ListItem, 0)
	for rows.Next() {
		e := &ListItem{}
		var repoCount, eventCount int
		if err := rows.Scan(&e.Value, &repoCount, &eventCount); err != nil {
			return nil, errors.Wrap(err, "failed to scan row")
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

	log.WithFields(log.Fields{
		"username": username,
		"limit":    limit,
	}).Debug("listing repositories...")

	opt := &github.ListOptions{}
	if limit > 0 {
		opt.PerPage = limit
	}

	items, resp, err := github.NewClient(client).Organizations.List(ctx, username, opt)

	log.WithFields(log.Fields{
		"username":    username,
		"status":      resp.Status,
		"status_code": resp.StatusCode,
		"count":       len(items),
		"limit":       opt.PerPage,
	}).Debug("got repositories")

	if err != nil {
		return nil, errors.Wrapf(err, "failed to list repositories for: %s", username)
	}

	list := make([]*Org, 0)
	for _, r := range items {
		log.Debugf("org: %+v", r)
		list = append(list, mapOrg(r))
	}

	return list, nil
}
