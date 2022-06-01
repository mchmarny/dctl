package data

import (
	"context"
	"database/sql"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/google/go-github/v45/github"
	"github.com/mchmarny/dctl/pkg/net"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
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
		return nil, errors.Wrapf(err, "error getting DB: %s", dbPath)
	}
	defer db.Close()

	list, err := GetAllOrgRepos(db)
	if err != nil {
		return nil, errors.Wrapf(err, "error getting org/repo list")
	}

	results := make(map[string]int)

	for _, r := range list {
		m, err := ImportEvents(dbPath, token, r.Org, r.Repo, EventAgeMonthsDefault)
		if err != nil {
			log.Errorf("error importing events for %s/%s: %v", r.Org, r.Repo, err)
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
		return nil, errors.Wrapf(err, "error loading last page state: %s/%s", owner, repo)
	}

	log.Debugf("importing events for %s/%s", owner, repo)
	var wg sync.WaitGroup

	errCh := make(chan error, len(importers))

	go func() {
		for err := range errCh {
			log.Error(err)
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
		return nil, errors.Wrapf(err, "error flushing final events: %s/%s", imp.owner, imp.repo)
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
			return errors.Wrap(err, "error flushing events")
		}
	}
	return nil
}

func (e *EventImporter) loadState() error {
	db, err := GetDB(e.dbPath)
	if err != nil {
		return errors.Wrapf(err, "error getting DB: %s", e.dbPath)
	}
	defer db.Close()

	for _, t := range EventTypes {
		state, err := GetState(db, t, e.owner, e.repo, e.minEventTime)
		if err != nil {
			return errors.Wrapf(err, "error getting last page: %s/%s - %s", e.owner, e.repo, t)
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

	log.Debugf("flushing %d events and %d developers to db...", len(events), len(devs))

	db, err := GetDB(e.dbPath)
	if err != nil {
		return errors.Wrapf(err, "error getting DB: %s", e.dbPath)
	}
	defer db.Close()

	eventStmt, err := db.Prepare(insertEventSQL)
	if err != nil {
		return errors.Wrap(err, "failed to prepare event insert statement")
	}

	devStmt, err := db.Prepare(insertDeveloperSQL)
	if err != nil {
		return errors.Wrap(err, "failed to prepare developer insert statement")
	}

	stateStmt, err := db.Prepare(insertState)
	if err != nil {
		return errors.Wrapf(err, "failed to prepare state insert statement")
	}

	tx, err := db.Begin()
	if err != nil {
		return errors.Wrap(err, "failed to begin transaction")
	}

	for i, u := range devs {
		if _, err = tx.Stmt(devStmt).Exec(u.Username,
			u.FullName, u.Email, u.AvatarURL, u.ProfileURL, u.Entity,
			u.FullName, u.Email, u.AvatarURL, u.ProfileURL, u.Entity); err != nil {
			rollbackTransaction(tx)
			return errors.Wrapf(err, "error inserting developer[%d]: %s", i, u.Username)
		}
	}

	for i, e := range events {
		_, err = tx.Stmt(eventStmt).Exec(e.Org, e.Repo, e.Username, e.Type, e.Date,
			e.URL, e.Mentions, e.Labels, e.URL, e.Mentions, e.Labels)
		if err != nil {
			rollbackTransaction(tx)
			return errors.Wrapf(err, "error inserting event[%d]: %s/%s", i, e.Org, e.Repo)
		}
	}

	for t, p := range state {
		since := p.Since.Unix()
		_, err = tx.Stmt(stateStmt).Exec(t, e.owner, e.repo, p.Page, since, p.Page, since)
		if err != nil {
			rollbackTransaction(tx)
			return errors.Wrapf(err, "error inserting state[%s]: %s/%s with page:%d and since:%s",
				t, e.owner, e.repo, p.Page, p.Since.Format("2006-01-02"))
		}
	}

	if err = tx.Commit(); err != nil {
		return errors.Wrap(err, "failed to commit transaction")
	}

	log.Debugf("successfully flushed in %s", time.Since(start).String())

	return nil
}

func rollbackTransaction(tx *sql.Tx) {
	if err := tx.Rollback(); err != nil {
		log.Errorf("error rolling back transaction: %s", err)
	}
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
	log.Debugf("starting pr event import on page %d since %s", e.state[EventTypePR].Page, e.state[EventTypePR].Since.Format("2006-01-02"))

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
			return errors.Wrapf(err, "error listing prs, rate: %s", rateInfo(&resp.Rate))
		}
		log.Debugf("pr - found:%d, page:%d/%d, %s", len(items), resp.NextPage, resp.LastPage, rateInfo(&resp.Rate))

		if len(items) == 0 {
			break
		}

		// PR list has no since option so break manually when both 1st and last event are older than the min.
		if !e.isEventBatchValidAge(items[0].CreatedAt, items[len(items)-1].CreatedAt) {
			log.Debugf("pr - all returned events older than %v", e.minEventTime.Format("2006-01-02"))
			break
		}

		for i := range items {
			mentions := parseUsers(items[i].Body)
			mentions = append(mentions, getUsernames(items[i].Assignee)...)
			mentions = append(mentions, getUsernames(items[i].Assignees...)...)
			mentions = append(mentions, getUsernames(items[i].RequestedReviewers...)...)
			if err := e.add(EventTypePR, *items[i].HTMLURL, items[i].User, items[i].UpdatedAt, mentions,
				getLabels(items[i].Labels)); err != nil {
				return errors.Wrapf(err, "error adding pr event: %s/%s", e.owner, e.repo)
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
	log.Debugf("starting issue event import on page %d since %s", e.state[EventTypeIssue].Page, e.state[EventTypeIssue].Since.Format("2006-01-02"))

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
			return errors.Wrapf(err, "error listing issues, rate: %s", rateInfo(&resp.Rate))
		}
		log.Debugf("issue - found:%d, page:%d/%d, %s", len(items), resp.NextPage, resp.LastPage, rateInfo(&resp.Rate))

		if len(items) == 0 {
			break
		}

		for i := range items {
			mentions := parseUsers(items[i].Body)
			mentions = append(mentions, getUsernames(items[i].Assignee)...)
			mentions = append(mentions, getUsernames(items[i].Assignees...)...)
			if err := e.add(EventTypeIssue, *items[i].HTMLURL, items[i].User,
				items[i].UpdatedAt, mentions, getLabels(items[i].Labels)); err != nil {
				return errors.Wrapf(err, "error adding issue event: %s/%s", e.owner, e.repo)
			}
		}

		e.state[EventTypeIssue].Page = opt.ListOptions.Page

		if resp.NextPage == 0 {
			break
		}

		opt.Page = resp.NextPage
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
	log.Debugf("starting issue comment event import on page %d since %s", e.state[EventTypeIssueComment].Page, e.state[EventTypeIssueComment].Since.Format("2006-01-02"))

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
			return errors.Wrapf(err, "error listing issue comments, rate: %s", rateInfo(&resp.Rate))
		}
		log.Debugf("issue comment - found:%d, page:%d/%d, %s", len(items), resp.NextPage, resp.LastPage, rateInfo(&resp.Rate))

		if len(items) == 0 {
			break
		}

		for i := range items {
			if err := e.add(EventTypeIssueComment, *items[i].HTMLURL, items[i].User, items[i].UpdatedAt, parseUsers(items[i].Body), nil); err != nil {
				return errors.Wrapf(err, "error adding issue comment event: %s/%s", e.owner, e.repo)
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
	log.Debugf("starting pr comment event import on page %d since %s", e.state[EventTypePRReview].Page, e.state[EventTypePRReview].Since.Format("2006-01-02"))

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
			return errors.Wrapf(err, "error listing pr comments, rate: %s", rateInfo(&resp.Rate))
		}
		log.Debugf("pr comment - found:%d, page:%d/%d, %s", len(items), resp.NextPage, resp.LastPage, rateInfo(&resp.Rate))

		if len(items) == 0 {
			break
		}

		for i := range items {
			if err := e.add(EventTypePRReview, *items[i].HTMLURL, items[i].User, items[i].UpdatedAt, parseUsers(items[i].Body), nil); err != nil {
				return errors.Wrapf(err, "error adding PR comment event: %s/%s", e.owner, e.repo)
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
	log.Debugf("starting fork event import on page %d since %s", e.state[EventTypeFork].Page, e.state[EventTypeFork].Since.Format("2006-01-02"))

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
			return errors.Wrapf(err, "error listing forks, rate: %s", rateInfo(&resp.Rate))
		}
		log.Debugf("fork - found:%d, page:%d/%d, %s", len(items), resp.NextPage, resp.LastPage, rateInfo(&resp.Rate))

		if len(items) == 0 {
			break
		}

		for i := range items {
			if err := e.add(EventTypeFork, *items[i].HTMLURL, items[i].Owner, &items[i].UpdatedAt.Time, nil, items[i].Topics); err != nil {
				return errors.Wrapf(err, "error adding fork event: %s/%s", e.owner, e.repo)
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
