package data

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"

	"github.com/google/go-github/v45/github"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
)

const (
	selectRepoLike = `SELECT org, repo, COUNT(DISTINCT id) as event_count  
		FROM event 
		WHERE repo like ?
		GROUP BY org, repo
		ORDER BY org DESC, repo DESC
		LIMIT ?
	`
)

type CountedItem struct {
	Name  string `json:"name,omitempty"`
	Count int    `json:"count,omitempty"`
}

type Repo struct {
	Name        string `json:"name,omitempty"`
	FullName    string `json:"full_name,omitempty"`
	Description string `json:"description,omitempty"`
	URL         string `json:"url,omitempty"`
}

type ListItem struct {
	Value string `json:"value,omitempty"`
	Text  string `json:"text,omitempty"`
}

func mapRepo(r *github.Repository) *Repo {
	return &Repo{
		Name:        trim(r.Name),
		FullName:    trim(r.FullName),
		Description: trim(r.Description),
		URL:         trim(r.HTMLURL),
	}
}

// GetRepoLike returns a list of repos that match the given pattern.
func GetRepoLike(db *sql.DB, query string, limit int) ([]*ListItem, error) {
	if db == nil {
		return nil, errDBNotInitialized
	}

	if query == "" {
		return nil, errors.New("query is required")
	}

	stmt, err := db.Prepare(selectRepoLike)
	if err != nil {
		return nil, errors.Wrap(err, "failed to prepare repo like statement")
	}

	query = fmt.Sprintf("%%%s%%", query)
	rows, err := stmt.Query(query, limit)
	if err != nil && err != sql.ErrNoRows {
		return nil, errors.Wrap(err, "failed to execute select statement")
	}
	defer rows.Close()

	list := make([]*ListItem, 0)
	for rows.Next() {
		var org, repo string
		var count int
		if err := rows.Scan(&org, &repo, &count); err != nil {
			return nil, errors.Wrap(err, "failed to scan row")
		}
		e := &ListItem{
			Value: fmt.Sprintf("%s/%s", org, repo),
			Text:  fmt.Sprintf("%s/%s (%d events)", org, repo, count),
		}
		list = append(list, e)
	}

	return list, nil
}

func GetOrgRepos(ctx context.Context, client *http.Client, org string) ([]*Repo, error) {
	if org == "" {
		return nil, errors.New("org is required")
	}

	opt := &github.RepositoryListOptions{}
	items, resp, err := github.NewClient(client).Repositories.List(ctx, org, opt)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to list repositories for: %s", org)
	}

	log.WithFields(log.Fields{
		"org":         org,
		"status":      resp.Status,
		"status_code": resp.StatusCode,
		"count":       len(items),
	}).Debug("list repositories")

	list := make([]*Repo, 0)
	for _, r := range items {
		list = append(list, mapRepo(r))
	}

	return list, nil
}
