package data

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"

	"github.com/google/go-github/v83/github"
)

const (
	selectRepoLike = `SELECT org, repo, COUNT(*) as event_count  
		FROM event 
		WHERE repo like ?
		GROUP BY org, repo
		ORDER BY org DESC, repo DESC
		LIMIT ?
	`
)

type CountedItem struct {
	Name  string `json:"name,omitempty" yaml:"name,omitempty"`
	Count int    `json:"count,omitempty" yaml:"count,omitempty"`
}

type Repo struct {
	Name        string `json:"name,omitempty" yaml:"name,omitempty"`
	FullName    string `json:"full_name,omitempty" yaml:"fullName,omitempty"`
	Description string `json:"description,omitempty" yaml:"description,omitempty"`
	URL         string `json:"url,omitempty" yaml:"url,omitempty"`
}

type ListItem struct {
	Value string `json:"value,omitempty" yaml:"value,omitempty"`
	Text  string `json:"text,omitempty" yaml:"text,omitempty"`
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
		return nil, fmt.Errorf("failed to prepare repo like statement: %w", err)
	}

	query = fmt.Sprintf("%%%s%%", query)
	rows, err := stmt.Query(query, limit)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("failed to execute select statement: %w", err)
	}
	defer rows.Close()

	list := make([]*ListItem, 0)
	for rows.Next() {
		var org, repo string
		var count int
		if err := rows.Scan(&org, &repo, &count); err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}
		e := &ListItem{
			Value: fmt.Sprintf("%s/%s", org, repo),
			Text:  fmt.Sprintf("%s/%s (%d events)", org, repo, count),
		}
		list = append(list, e)
	}

	return list, nil
}

func GetOrgRepoNames(ctx context.Context, client *http.Client, org string) ([]string, error) {
	list, err := GetOrgRepos(ctx, client, org)
	if err != nil {
		return nil, err
	}
	repos := make([]string, 0)
	for _, r := range list {
		repos = append(repos, r.Name)
	}
	return repos, nil
}

func GetOrgRepos(ctx context.Context, client *http.Client, org string) ([]*Repo, error) {
	if org == "" {
		return nil, errors.New("org is required")
	}

	opt := &github.RepositoryListByUserOptions{}
	items, _, err := github.NewClient(client).Repositories.ListByUser(ctx, org, opt)
	if err != nil {
		return nil, fmt.Errorf("failed to list repositories for: %s: %w", org, err)
	}

	list := make([]*Repo, 0)
	for _, r := range items {
		list = append(list, mapRepo(r))
	}

	return list, nil
}
