package data

import (
	"context"
	"database/sql"
	"net/http"
	"sync"
	"time"

	"github.com/google/go-github/v44/github"
	"github.com/mchmarny/dctl/pkg/net"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
)

const (
	// EventTypes is a list of event types to import
	EventTypePR           string = "pr_request"
	EventTypePRComment    string = "pr_comment"
	EventTypeIssue        string = "issue_request"
	EventTypeIssueComment string = "issue_comment"

	pageSizeDefault = 100
	importBatchSize = 500
	nilNumber       = 0

	EventAgeMonthsDefault = 6

	sortField        string = "created"
	sortCommentField string = "updated"
	sortDirection    string = "desc"

	insertEventSQL = `INSERT INTO event (
			id, org, repo, username, event_type, event_date
		) 
		VALUES (?, ?,?, ?, ?, ?) 
		ON CONFLICT(id, org, repo, username, event_type, event_date) DO NOTHING
	`
)

var (
	event_types = []string{
		EventTypePR,
		EventTypeIssue,
		EventTypeIssueComment,
		EventTypePRComment,
	}
)

type Event struct {
	ID       int64  `json:"id,omitempty"`
	Org      string `json:"org,omitempty"`
	Repo     string `json:"repo,omitempty"`
	Username string `json:"username,omitempty"`
	Type     string `json:"type,omitempty"`
	Date     string `json:"date,omitempty"`
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
		state:        make(map[string]int),
		minEventTime: time.Now().AddDate(0, -months, 0),
	}

	importers := []importer{
		imp.importPREvents,
		imp.importIssueEvents,
		imp.importIssueCommentEvents,
		imp.importPRCommentEvents,
	}

	if err := imp.loadLastPageState(); err != nil {
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
	state        map[string]int
	minEventTime time.Time
}

func (e *EventImporter) add(id int64, eType string, usr *github.User, updated *time.Time) error {
	item := &Event{
		ID:       id,
		Org:      e.owner,
		Repo:     e.repo,
		Username: trim(usr.Login),
		Type:     eType,
		Date:     parseDate(updated),
	}

	e.mu.Lock()
	e.list = append(e.list, item)
	e.counts[eType]++
	e.users[item.Username] = usr
	e.mu.Unlock()

	if len(e.list) >= importBatchSize {
		if err := e.flush(); err != nil {
			return errors.Wrap(err, "error flushing events")
		}
	}
	return nil
}

func (e *EventImporter) loadLastPageState() error {
	db, err := GetDB(e.dbPath)
	if err != nil {
		return errors.Wrapf(err, "error getting DB: %s", e.dbPath)
	}
	defer db.Close()

	for _, t := range event_types {
		lastPage, err := GetState(db, t, e.owner, e.repo)
		if err != nil {
			return errors.Wrapf(err, "error getting last page: %s/%s - %s", e.owner, e.repo, t)
		}
		e.state[t] = lastPage
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
	var state map[string]int

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
			u.Updated, u.ID, u.FullName, u.Email, u.AvatarURL, u.ProfileURL, u.Entity, u.Location,
			u.Updated, u.ID, u.FullName, u.Email, u.AvatarURL, u.ProfileURL, u.Entity, u.Location); err != nil {
			rollbackTransaction(tx)
			return errors.Wrapf(err, "error inserting developer[%d]: %s", i, u.Username)
		}
	}

	for i, e := range events {
		_, err = tx.Stmt(eventStmt).Exec(e.ID, e.Org, e.Repo, e.Username, e.Type, e.Date)
		if err != nil {
			rollbackTransaction(tx)
			return errors.Wrapf(err, "error inserting event[%d]: %s/%s#%d", i, e.Org, e.Repo, e.ID)
		}
	}

	for t, p := range state {
		_, err = tx.Stmt(stateStmt).Exec(t, e.owner, e.repo, p, p)
		if err != nil {
			rollbackTransaction(tx)
			return errors.Wrapf(err, "error inserting state[%s]: %s/%s", t, e.owner, e.repo)
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
	log.Debugf("starting pr event import on page %d", e.state[EventTypePR])

	opt := &github.PullRequestListOptions{
		State:     "all",
		Sort:      sortField,
		Direction: sortDirection,
		ListOptions: github.ListOptions{
			PerPage: pageSizeDefault,
			Page:    e.state[EventTypePR],
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
			log.Debugf("pr - all returned events older than %d months", EventAgeMonthsDefault)
			break
		}

		for i := range items {
			if err := e.add(*items[i].ID, EventTypePR, items[i].User, items[i].CreatedAt); err != nil {
				return errors.Wrapf(err, "error adding pr event: %s/%s", e.owner, e.repo)
			}
		}

		e.state[EventTypePR] = opt.ListOptions.Page

		if resp.NextPage == 0 {
			break
		}

		opt.Page = resp.NextPage
	}

	return nil
}

func (e *EventImporter) importIssueEvents(ctx context.Context) error {
	log.Debugf("starting issue event import on page %d", e.state[EventTypeIssue])

	opt := &github.IssueListByRepoOptions{
		State:     "all",
		Sort:      sortField,
		Direction: sortDirection,
		Since:     e.minEventTime,
		ListOptions: github.ListOptions{
			PerPage: pageSizeDefault,
			Page:    e.state[EventTypeIssue],
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
			if err := e.add(*items[i].ID, EventTypeIssue, items[i].User, items[i].CreatedAt); err != nil {
				return errors.Wrapf(err, "error adding issue event: %s/%s", e.owner, e.repo)
			}
		}

		e.state[EventTypeIssue] = opt.ListOptions.Page

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
	log.Debugf("starting issue comment event import on page %d", e.state[EventTypeIssueComment])

	opt := &github.IssueListCommentsOptions{
		Sort:      getStrPtr(sortField),
		Direction: getStrPtr(sortCommentField),
		Since:     &e.minEventTime,
		ListOptions: github.ListOptions{
			PerPage: pageSizeDefault,
			Page:    e.state[EventTypeIssueComment],
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
			if err := e.add(*items[i].ID, EventTypeIssueComment, items[i].User, items[i].UpdatedAt); err != nil {
				return errors.Wrapf(err, "error adding issue comment event: %s/%s", e.owner, e.repo)
			}
		}

		e.state[EventTypeIssueComment] = opt.ListOptions.Page

		if resp.NextPage == 0 {
			break
		}

		opt.Page = resp.NextPage
	}

	return nil
}

func (e *EventImporter) importPRCommentEvents(ctx context.Context) error {
	log.Debugf("starting pr comment event import on page %d", e.state[EventTypePRComment])

	opt := &github.PullRequestListCommentsOptions{
		Sort:      sortField,
		Direction: sortCommentField,
		Since:     e.minEventTime,
		ListOptions: github.ListOptions{
			PerPage: pageSizeDefault,
			Page:    e.state[EventTypePRComment],
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
			if err := e.add(*items[i].ID, EventTypePRComment, items[i].User, items[i].UpdatedAt); err != nil {
				return errors.Wrapf(err, "error adding PR comment event: %s/%s", e.owner, e.repo)
			}
		}

		e.state[EventTypePRComment] = opt.ListOptions.Page

		if resp.NextPage == 0 {
			break
		}

		opt.Page = resp.NextPage
	}

	return nil
}
