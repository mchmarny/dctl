package data

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"log/slog"

	"github.com/google/go-github/v45/github"
)

var (
	usernameRegEx = regexp.MustCompile(`@([A-Za-z0-9_]+)`)
)

func mapUserToDeveloper(u *github.User) *Developer {
	return &Developer{
		Username:   trim(u.Login),
		FullName:   trim(u.Name),
		Email:      trim(u.Email),
		AvatarURL:  trim(u.AvatarURL),
		ProfileURL: trim(u.HTMLURL),
		Entity:     trim(u.Company),
	}
}

func mapGitHubUserToDeveloperListItem(u *github.User) *DeveloperListItem {
	return &DeveloperListItem{
		Username: trim(u.Login),
		Entity:   trim(u.Company),
	}
}

func trim(s *string) string {
	if s != nil {
		return strings.ReplaceAll(strings.TrimSpace(*s), "@", "")
	}
	return ""
}

func parseDate(t *time.Time) string {
	if t != nil {
		return t.Format("2006-01-02")
	}
	return time.Now().UTC().Format("2006-01-02")
}

func rateInfo(r *github.Rate) string {
	if r == nil {
		return ""
	}
	return fmt.Sprintf("rate:%d/%d until:%s", r.Remaining, r.Limit, r.Reset.Format("15:04"))
}

func GetGitHubDeveloper(ctx context.Context, client *http.Client, username string) (*Developer, error) {
	if username == "" {
		return nil, errors.New("username is required")
	}

	usr, resp, err := github.NewClient(client).Users.Get(ctx, username)
	if err != nil {
		return nil, fmt.Errorf("failed to list repositories for: %s: %w", username, err)
	}

	slog.Debug("got details for user", "username", username, "rate", rateInfo(&resp.Rate))

	return mapUserToDeveloper(usr), nil
}

func SearchGitHubUsers(ctx context.Context, client *http.Client, query string, limit int) ([]*DeveloperListItem, error) {
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

	r := make([]*DeveloperListItem, len(list.Users))
	for i, u := range list.Users {
		r[i] = mapGitHubUserToDeveloperListItem(u)
	}

	return r, nil
}

func getLabels(labels []*github.Label) []string {
	if labels == nil {
		return make([]string, 0)
	}

	r := make([]string, 0)
	for _, l := range labels {
		if l != nil {
			r = append(r, strings.ToLower(trim(l.Name)))
		}
	}
	return r
}

func getUsernames(users ...*github.User) []string {
	if users == nil {
		return make([]string, 0)
	}

	r := make([]string, 0)
	for _, u := range users {
		if u != nil {
			r = append(r, trim(u.Login))
		}
	}
	return r
}

func parseUsers(body *string) []string {
	if body == nil {
		return make([]string, 0)
	}
	return usernameRegEx.FindAllString(*body, -1)
}
