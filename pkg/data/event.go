package data

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"log/slog"

	"github.com/google/go-github/v83/github"
	"github.com/mchmarny/dctl/pkg/net"
)

const (
	// EventTypes is a list of event types to import
	EventTypePR           string = "pr"
	EventTypePRReview     string = "pr_review"
	EventTypeIssue        string = "issue"
	EventTypeIssueComment string = "issue_comment"
	EventTypeFork         string = "fork"

	pageSizeDefault = 100
	importBatchSize = 500
	nilNumber       = 0

	EventAgeMonthsDefault = 6

	sortField        string = "created"
	sortCommentField string = "updated"
	sortForkField    string = "newest"
	sortDirection    string = "desc"

	insertEventSQL = `INSERT INTO event (
			org, repo, username, type, date, url, mentions, labels
		) 
		VALUES (?, ?, ?, ?, ?, ?, ?, ?) 
		ON CONFLICT(org, repo, username, type, date) DO UPDATE SET 
			url = ?, mentions = ?, labels = ?
	`
)

var (
	EventTypes = []string{
		EventTypePR,
		EventTypeIssue,
		EventTypeIssueComment,
		EventTypePRReview,
		EventTypeFork,
	}
)

type Event struct {
	Org      string `json:"org,omitempty"`
	Repo     string `json:"repo,omitempty"`
	Username string `json:"username,omitempty"`
	Type     string `json:"type,omitempty"`
	Date     string `json:"date,omitempty"`
	URL      string `json:"url,omitempty"`
	Mentions string `json:"mentions,omitempty"`
	Labels   string `json:"labels,omitempty"`
}

type importer func(ctx context.Context) error

// ImportEvents imports events from GitHub for a given org/repo combination.
func UpdateEvents(dbPath, token string) (map[string]int, error) {
	if dbPath == "" || token == "" {
		return nil, errors.New("stateDir and token are required")
	}

	db, err := GetDB(dbPath)
	if err != nil {
		return nil, fmt.Errorf("error getting DB: %s: %w", dbPath, err)
	}
	defer db.Close()

	list, err := GetAllOrgRepos(db)
	if err != nil {
		return nil, fmt.Errorf("error getting org/repo list: %w", err)
	}

	results := make(map[string]int)

	for _, r := range list {
		m, err := ImportEvents(dbPath, token, r.Org, r.Repo, EventAgeMonthsDefault)
		if err != nil {
			slog.Error("error importing events", "org", r.Org, "repo", r.Repo, "error", err)
		}
		for k, v := range m {
			results[k] += v
		}
	}

	return results, nil
}

// ImportEvents imports events from GitHub for a given org/repo combination.
func ImportEvents(dbPath, token, owner, repo string, months int) (map[string]int, error) {
	if dbPath == "" || token == "" || owner == "" || repo == "" {
		return nil, errors.New("stateDir, token, owner, and repo are required")
	}

	if months < 1 {
		months = EventAgeMonthsDefault
	}

	ctx := context.Background()
	client := github.NewClient(net.GetOAuthClient(ctx, token))

	imp := &EventImporter{
		client:       client,
		dbPath:       dbPath,
		owner:        owner,
		repo:         repo,
		list:         make([]*Event, 0),
		counts:       make(map[string]int),
		users:        make(map[string]*github.User),
		state:        make(map[string]*State),
		minEventTime: time.Now().AddDate(0, -months, 0).UTC(),
	}

	importers := []importer{
		imp.importPREvents,
		imp.importPRReviewEvents,
		imp.importIssueEvents,
		imp.importIssueCommentEvents,
		imp.importForkEvents,
	}

	if err := imp.loadState(); err != nil {
		return nil, fmt.Errorf("error loading last page state: %s/%s: %w", owner, repo, err)
	}

	slog.Debug("importing events", "owner", owner, "repo", repo)
	var wg sync.WaitGroup

	errCh := make(chan error, len(importers))

	go func() {
		for err := range errCh {
			slog.Error("import error", "error", err)
		}
	}()

	for i := range importers {
		wg.Add(1)
		go func(i importer) {
			defer wg.Done()
			if err := i(ctx); err != nil {
				errCh <- err
			}
		}(importers[i])
	}

	wg.Wait()

	if err := imp.flush(); err != nil {
		return nil, fmt.Errorf("error flushing final events: %s/%s: %w", imp.owner, imp.repo, err)
	}

	return imp.counts, nil
}

type EventImporter struct {
	mu           sync.Mutex
	client       *github.Client
	dbPath       string
	owner        string
	repo         string
	list         []*Event
	counts       map[string]int
	users        map[string]*github.User
	state        map[string]*State
	minEventTime time.Time
}

func (e *EventImporter) qualifyTypeKey(t string) string {
	return e.owner + "/" + e.repo + "/" + t
}

func (e *EventImporter) add(eType, url string, usr *github.User, updated *time.Time, mentions []string, labels []string) error {
	item := &Event{
		Org:      e.owner,
		Repo:     e.repo,
		Username: usr.GetLogin(),
		Type:     eType,
		Date:     parseDate(updated),
		URL:      url,
		Mentions: strings.Join(unique(mentions), ","),
		Labels:   strings.Join(unique(labels), ","),
	}

	e.mu.Lock()
	e.list = append(e.list, item)
	e.counts[e.qualifyTypeKey(eType)]++
	e.users[item.Username] = usr
	e.mu.Unlock()

	if len(e.list) >= importBatchSize {
		if err := e.flush(); err != nil {
			return fmt.Errorf("error flushing events: %w", err)
		}
	}
	return nil
}

func (e *EventImporter) loadState() error {
	db, err := GetDB(e.dbPath)
	if err != nil {
		return fmt.Errorf("error getting DB: %s: %w", e.dbPath, err)
	}
	defer db.Close()

	for _, t := range EventTypes {
		state, err := GetState(db, t, e.owner, e.repo, e.minEventTime)
		if err != nil {
			return fmt.Errorf("error getting last page: %s/%s - %s: %w", e.owner, e.repo, t, err)
		}
		e.state[t] = state
	}

	return nil
}

func (e *EventImporter) flush() error {
	if len(e.list) == 0 {
		return nil
	}

	start := time.Now()

	var events []*Event
	var users map[string]*github.User
	var state map[string]*State

	e.mu.Lock()
	events = e.list
	users = e.users
	state = e.state
	e.list = make([]*Event, 0)
	e.mu.Unlock()

	// spool developers
	devs := make([]*Developer, 0)
	for _, v := range users {
		devs = append(devs, mapUserToDeveloper(v))
	}

	slog.Debug("flushing events and developers to db", "events", len(events), "developers", len(devs))

	db, err := GetDB(e.dbPath)
	if err != nil {
		return fmt.Errorf("error getting DB: %s: %w", e.dbPath, err)
	}
	defer db.Close()

	eventStmt, err := db.Prepare(insertEventSQL)
	if err != nil {
		return fmt.Errorf("failed to prepare event insert statement: %w", err)
	}

	devStmt, err := db.Prepare(insertDeveloperSQL)
	if err != nil {
		return fmt.Errorf("failed to prepare developer insert statement: %w", err)
	}

	stateStmt, err := db.Prepare(insertState)
	if err != nil {
		return fmt.Errorf("failed to prepare state insert statement: %w", err)
	}

	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	for i, u := range devs {
		if _, err = tx.Stmt(devStmt).Exec(u.Username,
			u.FullName, u.Email, u.AvatarURL, u.ProfileURL, u.Entity,
			u.FullName, u.Email, u.AvatarURL, u.ProfileURL, u.Entity); err != nil {
			rollbackTransaction(tx)
			return fmt.Errorf("error inserting developer[%d]: %s: %w", i, u.Username, err)
		}
	}

	for i, e := range events {
		_, err = tx.Stmt(eventStmt).Exec(e.Org, e.Repo, e.Username, e.Type, e.Date,
			e.URL, e.Mentions, e.Labels, e.URL, e.Mentions, e.Labels)
		if err != nil {
			rollbackTransaction(tx)
			return fmt.Errorf("error inserting event[%d]: %s/%s: %w", i, e.Org, e.Repo, err)
		}
	}

	for t, p := range state {
		since := p.Since.Unix()
		_, err = tx.Stmt(stateStmt).Exec(t, e.owner, e.repo, p.Page, since, p.Page, since)
		if err != nil {
			rollbackTransaction(tx)
			return fmt.Errorf("error inserting state[%s]: %s/%s with page:%d and since:%s: %w",
				t, e.owner, e.repo, p.Page, p.Since.Format("2006-01-02"), err)
		}
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	slog.Debug("successfully flushed", "duration", time.Since(start).String())

	return nil
}

func rollbackTransaction(tx *sql.Tx) {
	if err := tx.Rollback(); err != nil {
		slog.Error("error rolling back transaction", "error", err)
	}
}

func timestampToTime(ts *github.Timestamp) *time.Time {
	if ts == nil {
		return nil
	}
	return &ts.Time
}

func (e *EventImporter) isEventBatchValidAge(first *time.Time, last *time.Time) bool {
	if first == nil || last == nil {
		return false
	}

	if first.Before(e.minEventTime) && last.Before(e.minEventTime) {
		return false
	}

	return true
}

func (e *EventImporter) importPREvents(ctx context.Context) error {
	slog.Debug("starting pr event import", "page", e.state[EventTypePR].Page, "since", e.state[EventTypePR].Since.Format("2006-01-02"))

	opt := &github.PullRequestListOptions{
		State:     "all",
		Sort:      sortField,
		Direction: sortDirection,
		ListOptions: github.ListOptions{
			PerPage: pageSizeDefault,
			Page:    e.state[EventTypePR].Page,
		},
	}

	for {
		items, resp, err := e.client.PullRequests.List(ctx, e.owner, e.repo, opt)
		if err != nil || resp.StatusCode != http.StatusOK {
			net.PrintHTTPResponse(resp.Response)
			return fmt.Errorf("error listing prs, rate: %s: %w", rateInfo(&resp.Rate), err)
		}
		slog.Debug("pr events", "found", len(items), "next_page", resp.NextPage, "last_page", resp.LastPage, "rate", rateInfo(&resp.Rate))

		if len(items) == 0 {
			break
		}

		// PR list has no since option so break manually when both 1st and last event are older than the min.
		if !e.isEventBatchValidAge(timestampToTime(items[0].CreatedAt), timestampToTime(items[len(items)-1].CreatedAt)) {
			slog.Debug("pr - all returned events older than min", "min_event_time", e.minEventTime.Format("2006-01-02"))
			break
		}

		for i := range items {
			mentions := parseUsers(items[i].Body)
			mentions = append(mentions, getUsernames(items[i].Assignee)...)
			mentions = append(mentions, getUsernames(items[i].Assignees...)...)
			mentions = append(mentions, getUsernames(items[i].RequestedReviewers...)...)
			if err := e.add(EventTypePR, *items[i].HTMLURL, items[i].User, timestampToTime(items[i].UpdatedAt), mentions,
				getLabels(items[i].Labels)); err != nil {
				return fmt.Errorf("error adding pr event: %s/%s: %w", e.owner, e.repo, err)
			}
		}

		e.state[EventTypePR].Page = opt.ListOptions.Page

		if resp.NextPage == 0 {
			break
		}

		opt.Page = resp.NextPage
	}

	return nil
}

func (e *EventImporter) importIssueEvents(ctx context.Context) error {
	slog.Debug("starting issue event import", "page", e.state[EventTypeIssue].Page, "since", e.state[EventTypeIssue].Since.Format("2006-01-02"))

	opt := &github.IssueListByRepoOptions{
		State:     "all",
		Sort:      sortField,
		Direction: sortDirection,
		Since:     e.state[EventTypeIssue].Since,
		ListOptions: github.ListOptions{
			PerPage: pageSizeDefault,
			Page:    e.state[EventTypeIssue].Page,
		},
	}

	for {
		items, resp, err := e.client.Issues.ListByRepo(ctx, e.owner, e.repo, opt)
		if err != nil || resp.StatusCode != http.StatusOK {
			net.PrintHTTPResponse(resp.Response)
			return fmt.Errorf("error listing issues, rate: %s: %w", rateInfo(&resp.Rate), err)
		}
		slog.Debug("issue events", "found", len(items), "next_page", resp.NextPage, "last_page", resp.LastPage, "rate", rateInfo(&resp.Rate))

		if len(items) == 0 {
			break
		}

		for i := range items {
			mentions := parseUsers(items[i].Body)
			mentions = append(mentions, getUsernames(items[i].Assignee)...)
			mentions = append(mentions, getUsernames(items[i].Assignees...)...)
			if err := e.add(EventTypeIssue, *items[i].HTMLURL, items[i].User,
				timestampToTime(items[i].UpdatedAt), mentions, getLabels(items[i].Labels)); err != nil {
				return fmt.Errorf("error adding issue event: %s/%s: %w", e.owner, e.repo, err)
			}
		}

		e.state[EventTypeIssue].Page = opt.ListOptions.Page

		if resp.NextPage == 0 {
			break
		}

		opt.ListOptions.Page = resp.NextPage
	}

	return nil
}

func getStrPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func (e *EventImporter) importIssueCommentEvents(ctx context.Context) error {
	slog.Debug("starting issue comment event import", "page", e.state[EventTypeIssueComment].Page, "since", e.state[EventTypeIssueComment].Since.Format("2006-01-02"))

	opt := &github.IssueListCommentsOptions{
		Sort:      getStrPtr(sortField),
		Direction: getStrPtr(sortCommentField),
		Since:     &e.state[EventTypeIssueComment].Since,
		ListOptions: github.ListOptions{
			PerPage: pageSizeDefault,
			Page:    e.state[EventTypeIssueComment].Page,
		},
	}

	for {
		items, resp, err := e.client.Issues.ListComments(ctx, e.owner, e.repo, nilNumber, opt)
		if err != nil || resp.StatusCode != http.StatusOK {
			net.PrintHTTPResponse(resp.Response)
			return fmt.Errorf("error listing issue comments, rate: %s: %w", rateInfo(&resp.Rate), err)
		}
		slog.Debug("issue comment events", "found", len(items), "next_page", resp.NextPage, "last_page", resp.LastPage, "rate", rateInfo(&resp.Rate))

		if len(items) == 0 {
			break
		}

		for i := range items {
			if err := e.add(EventTypeIssueComment, *items[i].HTMLURL, items[i].User, timestampToTime(items[i].UpdatedAt), parseUsers(items[i].Body), nil); err != nil {
				return fmt.Errorf("error adding issue comment event: %s/%s: %w", e.owner, e.repo, err)
			}
		}

		e.state[EventTypeIssueComment].Page = opt.ListOptions.Page

		if resp.NextPage == 0 {
			break
		}

		opt.Page = resp.NextPage
	}

	return nil
}

func (e *EventImporter) importPRReviewEvents(ctx context.Context) error {
	slog.Debug("starting pr review event import", "page", e.state[EventTypePRReview].Page, "since", e.state[EventTypePRReview].Since.Format("2006-01-02"))

	opt := &github.PullRequestListCommentsOptions{
		Sort:      sortField,
		Direction: sortCommentField,
		Since:     e.state[EventTypePRReview].Since,
		ListOptions: github.ListOptions{
			PerPage: pageSizeDefault,
			Page:    e.state[EventTypePRReview].Page,
		},
	}

	for {
		items, resp, err := e.client.PullRequests.ListComments(ctx, e.owner, e.repo, nilNumber, opt)
		if err != nil || resp.StatusCode != http.StatusOK {
			net.PrintHTTPResponse(resp.Response)
			return fmt.Errorf("error listing pr comments, rate: %s: %w", rateInfo(&resp.Rate), err)
		}
		slog.Debug("pr review events", "found", len(items), "next_page", resp.NextPage, "last_page", resp.LastPage, "rate", rateInfo(&resp.Rate))

		if len(items) == 0 {
			break
		}

		for i := range items {
			if err := e.add(EventTypePRReview, *items[i].HTMLURL, items[i].User, timestampToTime(items[i].UpdatedAt), parseUsers(items[i].Body), nil); err != nil {
				return fmt.Errorf("error adding PR comment event: %s/%s: %w", e.owner, e.repo, err)
			}
		}

		e.state[EventTypePRReview].Page = opt.ListOptions.Page

		if resp.NextPage == 0 {
			break
		}

		opt.Page = resp.NextPage
	}

	return nil
}

func (e *EventImporter) importForkEvents(ctx context.Context) error {
	slog.Debug("starting fork event import", "page", e.state[EventTypeFork].Page, "since", e.state[EventTypeFork].Since.Format("2006-01-02"))

	opt := &github.RepositoryListForksOptions{
		Sort: sortForkField,
		ListOptions: github.ListOptions{
			PerPage: pageSizeDefault,
			Page:    e.state[EventTypeFork].Page,
		},
	}

	for {
		items, resp, err := e.client.Repositories.ListForks(ctx, e.owner, e.repo, opt)
		if err != nil || resp.StatusCode != http.StatusOK {
			net.PrintHTTPResponse(resp.Response)
			return fmt.Errorf("error listing forks, rate: %s: %w", rateInfo(&resp.Rate), err)
		}
		slog.Debug("fork events", "found", len(items), "next_page", resp.NextPage, "last_page", resp.LastPage, "rate", rateInfo(&resp.Rate))

		if len(items) == 0 {
			break
		}

		for i := range items {
			if err := e.add(EventTypeFork, *items[i].HTMLURL, items[i].Owner, &items[i].UpdatedAt.Time, nil, items[i].Topics); err != nil {
				return fmt.Errorf("error adding fork event: %s/%s: %w", e.owner, e.repo, err)
			}
		}

		e.state[EventTypeFork].Page = opt.ListOptions.Page

		if resp.NextPage == 0 {
			break
		}

		opt.Page = resp.NextPage
	}

	return nil
}

func unique(slice []string) []string {
	keys := make(map[string]bool)
	list := []string{}
	for _, entry := range slice {
		u := strings.ReplaceAll(strings.TrimSpace(entry), "@", "")
		if _, value := keys[u]; !value {
			keys[u] = true
			list = append(list, u)
		}
	}
	return list
}
