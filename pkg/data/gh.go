package data

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/google/go-github/v44/github"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
)

var (
	usernameRegEx = regexp.MustCompile(`@([A-Za-z0-9_]+)`)
)

func mapUserToDeveloper(u *github.User) *Developer {
	return &Developer{
		Username:   trim(u.Login),
		Updated:    toDate(u.UpdatedAt),
		ID:         *u.ID,
		FullName:   trim(u.Name),
		Email:      trim(u.Email),
		AvatarURL:  trim(u.AvatarURL),
		ProfileURL: trim(u.HTMLURL),
		Entity:     trim(u.Company),
		Location:   trim(u.Location),
	}
}

func mapGitHubUserToDeveloperListItem(u *github.User) *DeveloperListItem {
	return &DeveloperListItem{
		Username:    trim(u.Login),
		Entity:      trim(u.Company),
		UpdatedDate: toDate(u.UpdatedAt),
	}
}

func toDate(t *github.Timestamp) string {
	if t == nil {
		return time.Now().UTC().Format("2006-01-02")
	}
	return t.Time.Format("2006-01-02")
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
		return nil, errors.Wrapf(err, "failed to list repositories for: %s", username)
	}

	log.Debugf("got details for user: %s, %s", username, rateInfo(&resp.Rate))

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
		return nil, errors.Wrapf(err, "failed to search users for: %s", query)
	}

	if list == nil || len(list.Users) == 0 {
		return nil, nil
	}

	log.WithFields(log.Fields{
		"query":       query,
		"status":      resp.Status,
		"status_code": resp.StatusCode,
		"has_more":    list.IncompleteResults,
		"matched":     list.Total,
		"returned":    len(list.Users),
	}).Debug("get user")

	r := make([]*DeveloperListItem, len(list.Users))
	for i, u := range list.Users {
		r[i] = mapGitHubUserToDeveloperListItem(u)
	}

	return r, nil
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
