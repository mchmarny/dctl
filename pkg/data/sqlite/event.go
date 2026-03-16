package sqlite

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"log/slog"

	"github.com/google/go-github/v83/github"
	"github.com/mchmarny/devpulse/pkg/data"
	"github.com/mchmarny/devpulse/pkg/net"
)

const (
	EventTypePR           string = "pr"
	EventTypePRReview     string = "pr_review"
	EventTypeIssue        string = "issue"
	EventTypeIssueComment string = "issue_comment"
	EventTypeFork         string = "fork"

	pageSizeDefault = 100
	importBatchSize = 500
	nilNumber       = 0

	sortField        string = "created"
	sortCommentField string = "updated"
	sortForkField    string = "newest"
	sortDirection    string = "desc"

	selectPRsMissingSizeSQL = `SELECT org, repo, number
		FROM event
		WHERE type = 'pr'
		  AND org = ?
		  AND repo = ?
		  AND number IS NOT NULL
		  AND number > 0
		  AND (additions IS NULL OR changed_files IS NULL)
	`

	updatePRSizeSQL = `UPDATE event
		SET additions = ?, deletions = ?, changed_files = ?, commits = ?
		WHERE type = 'pr' AND org = ? AND repo = ? AND number = ?
	`

	insertEventSQL = `INSERT INTO event (
			org, repo, username, type, date, url, mentions, labels,
			state, number, created_at, closed_at, merged_at, additions, deletions,
			changed_files, commits, title
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(org, repo, username, type, date) DO UPDATE SET
			url = ?, mentions = ?, labels = ?,
			state = COALESCE(?, event.state),
			number = COALESCE(?, event.number),
			created_at = COALESCE(?, event.created_at),
			closed_at = COALESCE(?, event.closed_at),
			merged_at = COALESCE(?, event.merged_at),
			additions = COALESCE(?, event.additions),
			deletions = COALESCE(?, event.deletions),
			changed_files = COALESCE(?, event.changed_files),
			commits = COALESCE(?, event.commits),
			title = ?
	`
)

var EventTypes = []string{
	EventTypePR,
	EventTypeIssue,
	EventTypeIssueComment,
	EventTypePRReview,
	EventTypeFork,
}

type importerFunc func(ctx context.Context) error

func (s *Store) UpdateEvents(ctx context.Context, token string, concurrency int) (map[string]int, error) {
	if token == "" {
		return nil, errors.New("token is required")
	}

	if concurrency < 1 {
		concurrency = 1
	}

	list, err := s.GetAllOrgRepos()
	if err != nil {
		return nil, fmt.Errorf("error getting org/repo list: %w", err)
	}

	results := make(map[string]int)
	var mu sync.Mutex
	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup

	for _, r := range list {
		wg.Add(1)
		go func(org, repo string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			m, _, importErr := s.ImportEvents(ctx, token, org, repo, data.EventAgeMonthsDefault)
			if importErr != nil {
				slog.Error("error importing events", "org", org, "repo", repo, "error", importErr)
			}

			mu.Lock()
			for k, v := range m {
				results[k] += v
			}
			mu.Unlock()
		}(r.Org, r.Repo)
	}

	wg.Wait()

	return results, nil
}

func (s *Store) ImportEvents(ctx context.Context, token, owner, repo string, months int) (map[string]int, *data.ImportSummary, error) {
	if token == "" || owner == "" || repo == "" {
		return nil, nil, errors.New("token, owner, and repo are required")
	}

	if months < 1 {
		months = data.EventAgeMonthsDefault
	}

	client := github.NewClient(net.GetOAuthClient(ctx, token))

	imp := &eventImporter{
		client:       client,
		store:        s,
		owner:        owner,
		repo:         repo,
		list:         make([]*data.Event, 0),
		counts:       make(map[string]int),
		users:        make(map[string]*github.User),
		state:        make(map[string]*data.State),
		minEventTime: time.Now().AddDate(0, -months, 0).UTC(),
	}

	importers := []importerFunc{
		imp.importPREvents,
		imp.importPRReviewEvents,
		imp.importIssueEvents,
		imp.importIssueCommentEvents,
		imp.importForkEvents,
	}

	if err := imp.loadState(); err != nil {
		return nil, nil, fmt.Errorf("error loading last page state: %s/%s: %w", owner, repo, err)
	}

	var earliest time.Time
	for _, t := range EventTypes {
		st := imp.state[t]
		if earliest.IsZero() || st.Since.Before(earliest) {
			earliest = st.Since
		}
		slog.Debug("resume state",
			"repo", owner+"/"+repo,
			"type", t,
			"since", st.Since.Format("2006-01-02"),
			"page", st.Page)
	}
	slog.Info("importing events",
		"repo", owner+"/"+repo,
		"since", earliest.Format("2006-01-02"))
	var wg sync.WaitGroup
	var importErrors []error
	var errMu sync.Mutex

	for i := range importers {
		wg.Add(1)
		go func(fn importerFunc) {
			defer wg.Done()
			if err := fn(ctx); err != nil {
				errMu.Lock()
				importErrors = append(importErrors, err)
				errMu.Unlock()
			}
		}(importers[i])
	}

	wg.Wait()

	for _, err := range importErrors {
		slog.Error("event import failed", "repo", owner+"/"+repo, "error", err)
	}

	if err := imp.flush(); err != nil {
		return nil, nil, fmt.Errorf("error flushing final events: %s/%s: %w", imp.owner, imp.repo, err)
	}

	if err := imp.backfillPRSize(ctx); err != nil {
		slog.Warn("error backfilling PR size data", "repo", owner+"/"+repo, "error", err)
	}

	total := 0
	for _, v := range imp.counts {
		total += v
	}
	slog.Info("events imported",
		"repo", owner+"/"+repo,
		"events", total,
		"developers", len(imp.users),
		"since", earliest.Format("2006-01-02"))

	summary := &data.ImportSummary{
		Repo:       owner + "/" + repo,
		Since:      earliest.Format("2006-01-02"),
		Events:     total,
		Developers: len(imp.users),
	}

	return imp.counts, summary, nil
}

type eventImporter struct {
	mu           sync.Mutex
	client       *github.Client
	store        *Store
	owner        string
	repo         string
	list         []*data.Event
	counts       map[string]int
	users        map[string]*github.User
	state        map[string]*data.State
	minEventTime time.Time
	flushed      int
}

func (e *eventImporter) qualifyTypeKey(t string) string {
	return e.owner + "/" + e.repo + "/" + t
}

type eventExtra struct {
	State        *string
	Number       *int
	CreatedAt    *string
	ClosedAt     *string
	MergedAt     *string
	Additions    *int
	Deletions    *int
	ChangedFiles *int
	Commits      *int
	Title        string
}

func (e *eventImporter) add(eType, url string, usr *github.User, updated *time.Time, mentions []string, labels []string, extra *eventExtra) error {
	item := &data.Event{
		Org:      e.owner,
		Repo:     e.repo,
		Username: usr.GetLogin(),
		Type:     eType,
		Date:     parseDate(updated),
		URL:      url,
		Mentions: strings.Join(unique(mentions), ","),
		Labels:   strings.Join(unique(labels), ","),
	}

	if extra != nil {
		item.State = extra.State
		item.Number = extra.Number
		item.CreatedAt = extra.CreatedAt
		item.ClosedAt = extra.ClosedAt
		item.MergedAt = extra.MergedAt
		item.Additions = extra.Additions
		item.Deletions = extra.Deletions
		item.ChangedFiles = extra.ChangedFiles
		item.Commits = extra.Commits
		item.Title = extra.Title
	}

	e.mu.Lock()
	e.list = append(e.list, item)
	e.counts[e.qualifyTypeKey(eType)]++
	e.users[item.Username] = usr
	shouldFlush := len(e.list) >= importBatchSize
	e.mu.Unlock()

	if shouldFlush {
		if err := e.flush(); err != nil {
			return fmt.Errorf("error flushing events: %w", err)
		}
	}
	return nil
}

func (e *eventImporter) loadState() error {
	for _, t := range EventTypes {
		state, err := e.store.GetState(t, e.owner, e.repo, e.minEventTime)
		if err != nil {
			return fmt.Errorf("error getting last page: %s/%s - %s: %w", e.owner, e.repo, t, err)
		}
		e.state[t] = state
	}

	return nil
}

func (e *eventImporter) flush() error {
	if len(e.list) == 0 {
		return nil
	}

	start := time.Now()

	var events []*data.Event
	var users map[string]*github.User
	var state map[string]*data.State

	e.mu.Lock()
	events = e.list
	e.list = make([]*data.Event, 0)

	users = make(map[string]*github.User, len(e.users))
	for k, v := range e.users {
		users[k] = v
	}

	state = make(map[string]*data.State, len(e.state))
	for k, v := range e.state {
		cp := *v
		state[k] = &cp
	}
	e.mu.Unlock()

	devs := make([]*data.Developer, 0)
	for _, v := range users {
		devs = append(devs, mapUserToDeveloper(v))
	}

	slog.Debug("flushing events and developers to db", "events", len(events), "developers", len(devs))

	db := e.store.db

	eventStmt, err := db.Prepare(insertEventSQL)
	if err != nil {
		return fmt.Errorf("failed to prepare event insert statement: %w", err)
	}
	defer eventStmt.Close()

	devStmt, err := db.Prepare(insertDeveloperSQL)
	if err != nil {
		return fmt.Errorf("failed to prepare developer insert statement: %w", err)
	}
	defer devStmt.Close()

	stateStmt, err := db.Prepare(insertStateSQL)
	if err != nil {
		return fmt.Errorf("failed to prepare state insert statement: %w", err)
	}
	defer stateStmt.Close()

	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	txDevStmt := tx.Stmt(devStmt)
	for i, u := range devs {
		if _, err = txDevStmt.Exec(u.Username,
			u.FullName, u.Email, u.AvatarURL, u.ProfileURL, u.Entity,
			u.FullName, u.Email, u.AvatarURL, u.ProfileURL, u.Entity, u.Entity); err != nil {
			rollbackTransaction(tx)
			return fmt.Errorf("error inserting developer[%d]: %s: %w", i, u.Username, err)
		}
	}

	txEventStmt := tx.Stmt(eventStmt)
	for i, ev := range events {
		_, err = txEventStmt.Exec(
			ev.Org, ev.Repo, ev.Username, ev.Type, ev.Date,
			ev.URL, ev.Mentions, ev.Labels,
			ev.State, ev.Number, ev.CreatedAt, ev.ClosedAt, ev.MergedAt, ev.Additions, ev.Deletions,
			ev.ChangedFiles, ev.Commits, ev.Title,
			ev.URL, ev.Mentions, ev.Labels,
			ev.State, ev.Number, ev.CreatedAt, ev.ClosedAt, ev.MergedAt, ev.Additions, ev.Deletions,
			ev.ChangedFiles, ev.Commits, ev.Title,
		)
		if err != nil {
			rollbackTransaction(tx)
			return fmt.Errorf("error inserting event[%d]: %s/%s: %w", i, ev.Org, ev.Repo, err)
		}
	}

	txStateStmt := tx.Stmt(stateStmt)
	for t, p := range state {
		since := p.Since.Unix()
		_, err = txStateStmt.Exec(t, e.owner, e.repo, p.Page, since, p.Page, since)
		if err != nil {
			rollbackTransaction(tx)
			return fmt.Errorf("error inserting state[%s]: %s/%s with page:%d and since:%s: %w",
				t, e.owner, e.repo, p.Page, p.Since.Format("2006-01-02"), err)
		}
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	e.flushed += len(events)
	slog.Debug("flushed events",
		"repo", e.owner+"/"+e.repo,
		"batch", len(events),
		"total", e.flushed,
		"developers", len(users),
		"duration", time.Since(start).String())

	return nil
}

func timestampToTime(ts *github.Timestamp) *time.Time {
	if ts == nil {
		return nil
	}
	return &ts.Time
}

func (e *eventImporter) isEventBatchValidAge(first *time.Time, last *time.Time) bool {
	if first == nil || last == nil {
		return false
	}

	if first.Before(e.minEventTime) && last.Before(e.minEventTime) {
		return false
	}

	return true
}

func timestampStr(ts *github.Timestamp) *string {
	if ts == nil {
		return nil
	}
	s := ts.Format("2006-01-02T15:04:05Z")
	return &s
}

func intPtr(v int) *int {
	if v == 0 {
		return nil
	}
	return &v
}

func parsePRNumberFromURL(url string) int {
	parts := strings.Split(url, "/")
	if len(parts) == 0 {
		return 0
	}
	n, err := strconv.Atoi(parts[len(parts)-1])
	if err != nil {
		return 0
	}
	return n
}

func (e *eventImporter) importPREvents(ctx context.Context) error {
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
		if err != nil {
			return fmt.Errorf("error listing prs: %w", err)
		}
		if resp.StatusCode != http.StatusOK {
			net.PrintHTTPResponse(resp.Response)
			return fmt.Errorf("error listing prs, rate: %s, status: %d", rateInfo(&resp.Rate), resp.StatusCode)
		}
		checkRateLimit(resp)
		slog.Debug("pr events", "found", len(items), "next_page", resp.NextPage, "last_page", resp.LastPage, "rate", rateInfo(&resp.Rate))

		if len(items) == 0 {
			break
		}

		if !e.isEventBatchValidAge(timestampToTime(items[0].CreatedAt), timestampToTime(items[len(items)-1].CreatedAt)) {
			slog.Debug("pr - all returned events older than min", "min_event_time", e.minEventTime.Format("2006-01-02"))
			break
		}

		for i := range items {
			mentions := parseUsers(items[i].Body)
			mentions = append(mentions, getUsernames(items[i].Assignee)...)
			mentions = append(mentions, getUsernames(items[i].Assignees...)...)
			mentions = append(mentions, getUsernames(items[i].RequestedReviewers...)...)
			extra := &eventExtra{
				State:     items[i].State,
				Number:    items[i].Number,
				CreatedAt: timestampStr(items[i].CreatedAt),
				ClosedAt:  timestampStr(items[i].ClosedAt),
				MergedAt:  timestampStr(items[i].MergedAt),
				Additions: intPtr(items[i].GetAdditions()),
				Deletions: intPtr(items[i].GetDeletions()),
				Title:     items[i].GetTitle(),
			}
			if err := e.add(EventTypePR, *items[i].HTMLURL, items[i].User, timestampToTime(items[i].UpdatedAt), mentions,
				getLabels(items[i].Labels), extra); err != nil {
				return fmt.Errorf("error adding pr event: %s/%s: %w", e.owner, e.repo, err)
			}

			if err := e.importPRReviews(ctx, items[i].GetNumber()); err != nil {
				slog.Warn("error importing PR reviews", "pr", items[i].GetNumber(), "error", err)
			}
		}

		e.state[EventTypePR].Page = opt.ListOptions.Page

		if resp.NextPage == 0 {
			break
		}

		opt.ListOptions.Page = resp.NextPage
	}

	return nil
}

func (e *eventImporter) backfillPRSize(ctx context.Context) error {
	db := e.store.db

	rows, err := db.QueryContext(ctx, selectPRsMissingSizeSQL, e.owner, e.repo)
	if err != nil {
		return fmt.Errorf("error querying PRs missing size: %w", err)
	}
	defer rows.Close()

	type prRef struct {
		org, repo string
		number    int
	}
	var prs []prRef
	for rows.Next() {
		var r prRef
		if err := rows.Scan(&r.org, &r.repo, &r.number); err != nil {
			return fmt.Errorf("error scanning PR ref: %w", err)
		}
		prs = append(prs, r)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("error iterating PR refs: %w", err)
	}

	if len(prs) == 0 {
		return nil
	}

	updated := 0
	for _, p := range prs {
		pr, resp, err := e.client.PullRequests.Get(ctx, e.owner, e.repo, p.number)
		if err != nil {
			if wait := abuseRetryAfter(err); wait > 0 {
				slog.Warn("secondary rate limit hit, waiting", "number", p.number, "wait", wait.String())
				time.Sleep(wait)
				pr, resp, err = e.client.PullRequests.Get(ctx, e.owner, e.repo, p.number)
				if err != nil {
					slog.Warn("error fetching PR details after retry", "number", p.number, "error", err)
					continue
				}
			} else {
				slog.Warn("error fetching PR details", "number", p.number, "error", err)
				continue
			}
		}
		if resp.StatusCode != http.StatusOK {
			continue
		}
		checkRateLimit(resp)

		additions := pr.GetAdditions()
		deletions := pr.GetDeletions()
		changedFiles := pr.GetChangedFiles()
		commits := pr.GetCommits()

		if additions == 0 && deletions == 0 && changedFiles == 0 && commits == 0 {
			continue
		}

		if _, err := db.ExecContext(ctx, updatePRSizeSQL,
			intPtr(additions), intPtr(deletions), intPtr(changedFiles), intPtr(commits),
			p.org, p.repo, p.number); err != nil {
			slog.Warn("error updating PR size", "number", p.number, "error", err)
			continue
		}
		updated++
	}

	if updated > 0 {
		slog.Info("PR sizes backfilled", "repo", e.owner+"/"+e.repo, "updated", updated, "total", len(prs))
	}
	return nil
}

func (e *eventImporter) importPRReviews(ctx context.Context, prNumber int) error {
	if prNumber == 0 {
		return nil
	}

	opts := &github.ListOptions{PerPage: pageSizeDefault}

	for {
		reviews, resp, err := e.client.PullRequests.ListReviews(ctx, e.owner, e.repo, prNumber, opts)
		if err != nil {
			return fmt.Errorf("error listing reviews for PR #%d: %w", prNumber, err)
		}
		checkRateLimit(resp)

		for i := range reviews {
			if reviews[i].User == nil || reviews[i].HTMLURL == nil {
				continue
			}
			n := prNumber
			extra := &eventExtra{
				Number:    &n,
				CreatedAt: timestampStr(reviews[i].SubmittedAt),
			}
			if err := e.add(EventTypePRReview, *reviews[i].HTMLURL, reviews[i].User,
				timestampToTime(reviews[i].SubmittedAt), nil, nil, extra); err != nil {
				return fmt.Errorf("error adding PR review event: %w", err)
			}
		}

		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	return nil
}

func (e *eventImporter) importIssueEvents(ctx context.Context) error {
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
		if err != nil {
			return fmt.Errorf("error listing issues: %w", err)
		}
		if resp.StatusCode != http.StatusOK {
			net.PrintHTTPResponse(resp.Response)
			return fmt.Errorf("error listing issues, rate: %s, status: %d", rateInfo(&resp.Rate), resp.StatusCode)
		}
		checkRateLimit(resp)
		slog.Debug("issue events", "found", len(items), "next_page", resp.NextPage, "last_page", resp.LastPage, "rate", rateInfo(&resp.Rate))

		if len(items) == 0 {
			break
		}

		for i := range items {
			mentions := parseUsers(items[i].Body)
			mentions = append(mentions, getUsernames(items[i].Assignee)...)
			mentions = append(mentions, getUsernames(items[i].Assignees...)...)
			extra := &eventExtra{
				State:     items[i].State,
				Number:    items[i].Number,
				CreatedAt: timestampStr(items[i].CreatedAt),
				ClosedAt:  timestampStr(items[i].ClosedAt),
				Title:     items[i].GetTitle(),
			}
			if err := e.add(EventTypeIssue, *items[i].HTMLURL, items[i].User,
				timestampToTime(items[i].UpdatedAt), mentions, getLabels(items[i].Labels), extra); err != nil {
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

func (e *eventImporter) importIssueCommentEvents(ctx context.Context) error {
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
		if err != nil {
			return fmt.Errorf("error listing issue comments: %w", err)
		}
		if resp.StatusCode != http.StatusOK {
			net.PrintHTTPResponse(resp.Response)
			return fmt.Errorf("error listing issue comments, rate: %s, status: %d", rateInfo(&resp.Rate), resp.StatusCode)
		}
		checkRateLimit(resp)
		slog.Debug("issue comment events", "found", len(items), "next_page", resp.NextPage, "last_page", resp.LastPage, "rate", rateInfo(&resp.Rate))

		if len(items) == 0 {
			break
		}

		for i := range items {
			if err := e.add(EventTypeIssueComment, *items[i].HTMLURL, items[i].User, timestampToTime(items[i].UpdatedAt), parseUsers(items[i].Body), nil, nil); err != nil {
				return fmt.Errorf("error adding issue comment event: %s/%s: %w", e.owner, e.repo, err)
			}
		}

		e.state[EventTypeIssueComment].Page = opt.ListOptions.Page

		if resp.NextPage == 0 {
			break
		}

		opt.ListOptions.Page = resp.NextPage
	}

	return nil
}

func (e *eventImporter) importPRReviewEvents(ctx context.Context) error {
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
		if err != nil {
			return fmt.Errorf("error listing pr comments: %w", err)
		}
		if resp.StatusCode != http.StatusOK {
			net.PrintHTTPResponse(resp.Response)
			return fmt.Errorf("error listing pr comments, rate: %s, status: %d", rateInfo(&resp.Rate), resp.StatusCode)
		}
		checkRateLimit(resp)
		slog.Debug("pr review events", "found", len(items), "next_page", resp.NextPage, "last_page", resp.LastPage, "rate", rateInfo(&resp.Rate))

		if len(items) == 0 {
			break
		}

		for i := range items {
			extra := &eventExtra{
				CreatedAt: timestampStr(items[i].CreatedAt),
			}
			if items[i].PullRequestURL != nil {
				if n := parsePRNumberFromURL(*items[i].PullRequestURL); n > 0 {
					extra.Number = &n
				}
			}
			if err := e.add(EventTypePRReview, *items[i].HTMLURL, items[i].User, timestampToTime(items[i].UpdatedAt), parseUsers(items[i].Body), nil, extra); err != nil {
				return fmt.Errorf("error adding PR comment event: %s/%s: %w", e.owner, e.repo, err)
			}
		}

		e.state[EventTypePRReview].Page = opt.ListOptions.Page

		if resp.NextPage == 0 {
			break
		}

		opt.ListOptions.Page = resp.NextPage
	}

	return nil
}

func (e *eventImporter) importForkEvents(ctx context.Context) error {
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
		if err != nil {
			return fmt.Errorf("error listing forks: %w", err)
		}
		if resp.StatusCode != http.StatusOK {
			net.PrintHTTPResponse(resp.Response)
			return fmt.Errorf("error listing forks, rate: %s, status: %d", rateInfo(&resp.Rate), resp.StatusCode)
		}
		checkRateLimit(resp)
		slog.Debug("fork events", "found", len(items), "next_page", resp.NextPage, "last_page", resp.LastPage, "rate", rateInfo(&resp.Rate))

		if len(items) == 0 {
			break
		}

		for i := range items {
			if err := e.add(EventTypeFork, *items[i].HTMLURL, items[i].Owner, &items[i].UpdatedAt.Time, nil, items[i].Topics, nil); err != nil {
				return fmt.Errorf("error adding fork event: %s/%s: %w", e.owner, e.repo, err)
			}
		}

		e.state[EventTypeFork].Page = opt.ListOptions.Page

		if resp.NextPage == 0 {
			break
		}

		opt.ListOptions.Page = resp.NextPage
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
