package ghutil

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"log/slog"

	"github.com/google/go-github/v83/github"
	"github.com/mchmarny/devpulse/pkg/data"
)

var usernameRegEx = regexp.MustCompile(`@([A-Za-z0-9_]+)`)

func MapUserToDeveloper(u *github.User) *data.Developer {
	return &data.Developer{
		Username:   Trim(u.Login),
		FullName:   Trim(u.Name),
		Email:      Deref(u.Email),
		AvatarURL:  Deref(u.AvatarURL),
		ProfileURL: Deref(u.HTMLURL),
		Entity:     Trim(u.Company),
	}
}

func Deref(s *string) string {
	if s != nil {
		return strings.TrimSpace(*s)
	}
	return ""
}

func MapGitHubUserToDeveloperListItem(u *github.User) *data.DeveloperListItem {
	return &data.DeveloperListItem{
		Username: Trim(u.Login),
		Entity:   Trim(u.Company),
	}
}

func Trim(s *string) string {
	if s != nil {
		return strings.ReplaceAll(strings.TrimSpace(*s), "@", "")
	}
	return ""
}

func ParseDate(t *time.Time) string {
	if t != nil {
		return t.Format("2006-01-02")
	}
	return time.Now().UTC().Format("2006-01-02")
}

func RateInfo(r *github.Rate) string {
	if r == nil {
		return ""
	}
	return fmt.Sprintf("rate:%d/%d until:%s", r.Remaining, r.Limit, r.Reset.Format("15:04"))
}

func GetGitHubDeveloper(ctx context.Context, client *http.Client, username string) (*data.Developer, error) {
	if username == "" {
		return nil, errors.New("username is required")
	}

	usr, resp, err := github.NewClient(client).Users.Get(ctx, username)
	if err != nil {
		return nil, fmt.Errorf("failed to list repositories for: %s: %w", username, err)
	}

	slog.Debug("got details for user", "username", username, "rate", RateInfo(&resp.Rate))

	return MapUserToDeveloper(usr), nil
}

func SearchGitHubUsers(ctx context.Context, client *http.Client, query string, limit int) ([]*data.DeveloperListItem, error) {
	if query == "" {
		return nil, errors.New("query is required")
	}

	opts := &github.SearchOptions{
		ListOptions: github.ListOptions{
			PerPage: limit,
		},
	}
	list, resp, err := github.NewClient(client).Search.Users(ctx, query, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to search users for: %s: %w", query, err)
	}

	if list == nil || len(list.Users) == 0 {
		return nil, nil
	}

	slog.Debug("get user",
		"query", query,
		"status", resp.Status,
		"status_code", resp.StatusCode,
		"has_more", list.IncompleteResults,
		"matched", list.Total,
		"returned", len(list.Users),
	)

	r := make([]*data.DeveloperListItem, len(list.Users))
	for i, u := range list.Users {
		r[i] = MapGitHubUserToDeveloperListItem(u)
	}

	return r, nil
}

func GetLabels(labels []*github.Label) []string {
	if labels == nil {
		return make([]string, 0)
	}

	r := make([]string, 0)
	for _, l := range labels {
		if l != nil {
			r = append(r, strings.ToLower(Trim(l.Name)))
		}
	}
	return r
}

func GetUsernames(users ...*github.User) []string {
	if users == nil {
		return make([]string, 0)
	}

	r := make([]string, 0)
	for _, u := range users {
		if u != nil {
			r = append(r, Trim(u.Login))
		}
	}
	return r
}

func ParseUsers(body *string) []string {
	if body == nil {
		return make([]string, 0)
	}
	return usernameRegEx.FindAllString(*body, -1)
}

func MapRepo(r *github.Repository) *data.Repo {
	return &data.Repo{
		Name:        Trim(r.Name),
		FullName:    Trim(r.FullName),
		Description: Trim(r.Description),
		URL:         Trim(r.HTMLURL),
	}
}

func MapOrg(r *github.Organization) *data.Org {
	return &data.Org{
		Name:        Trim(r.Login),
		Company:     Trim(r.Company),
		Description: Trim(r.Description),
		URL:         Trim(r.URL),
	}
}

func GetUserOrgs(ctx context.Context, client *http.Client, username string, limit int) ([]*data.Org, error) {
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

	list := make([]*data.Org, 0)
	for _, r := range items {
		slog.Debug("org", "value", r)
		list = append(list, MapOrg(r))
	}

	return list, nil
}

func GetOrgRepos(ctx context.Context, client *http.Client, org string) ([]*data.Repo, error) {
	if org == "" {
		return nil, errors.New("org is required")
	}

	ghClient := github.NewClient(client)
	opt := &github.RepositoryListByUserOptions{
		ListOptions: github.ListOptions{PerPage: 100},
	}

	var list []*data.Repo

	for {
		items, resp, err := ghClient.Repositories.ListByUser(ctx, org, opt)
		if err != nil {
			return nil, fmt.Errorf("failed to list repositories for: %s: %w", org, err)
		}
		CheckRateLimit(resp)

		for _, r := range items {
			list = append(list, MapRepo(r))
		}

		if resp.NextPage == 0 {
			break
		}
		opt.Page = resp.NextPage
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
