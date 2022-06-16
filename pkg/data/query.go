package data

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/pkg/errors"
)

const (
	selectEventTypesSinceSQL = `SELECT
			date, 
			SUM(prs) as prs,
			SUM(pr_review) as pr_review,
			SUM(issues) as issues, 
			SUM(issue_comments) as issue_comments,
			SUM(forks) as forks
		FROM (
			WITH RECURSIVE dates(date) AS (
				VALUES(?)
				UNION ALL
				SELECT date(date, '+1 day')
				FROM dates
				WHERE date < ?
			)
			SELECT 
				substr(dates.date, 0, 8) as date,
				CASE WHEN e.type = ? THEN 1 ELSE 0 END as prs,
				CASE WHEN e.type = ? THEN 1 ELSE 0 END as pr_review,
				CASE WHEN e.type = ? THEN 1 ELSE 0 END as issues,
				CASE WHEN e.type = ? THEN 1 ELSE 0 END as issue_comments,
				CASE WHEN e.type = ? THEN 1 ELSE 0 END as forks
			FROM dates 
			LEFT JOIN event e ON dates.date = e.date
			JOIN developer d ON e.username = d.username
			AND e.org = COALESCE(?, e.org)
			AND e.repo = COALESCE(?, e.repo)
			AND d.entity = COALESCE(?, d.entity)
		) dt
		GROUP BY date
		ORDER BY 1
	`

	selectEventSQL = `SELECT
			e.org,
			e.repo,
			e.date,
			e.type,
			e.url,
			e.mentions,
			e.labels,
			d.username,
			d.email,
			d.full_name,
			d.avatar,
			d.url,
			d.entity
		FROM event e
		JOIN developer d ON e.username = d.username
		WHERE e.date >= COALESCE(?, e.date)
		AND e.date <= COALESCE(?, e.date)
		AND e.type = COALESCE(?, e.type)
		AND e.org = COALESCE(?, e.org)
		AND e.repo = COALESCE(?, e.repo)
		AND e.username = COALESCE(?, e.username)
		AND e.mentions LIKE COALESCE(?, e.mentions)
		AND e.labels LIKE COALESCE(?, e.labels)
		AND d.entity = COALESCE(?, d.entity)
		ORDER BY 1 DESC, 2, 3
		LIMIT ? OFFSET ?
	`
)

type EventTypeSeries struct {
	Dates         []string  `json:"dates"`
	PRs           []int     `json:"pr"`
	PRReviews     []int     `json:"pr_review"`
	Issues        []int     `json:"issue"`
	IssueComments []int     `json:"issue_comment"`
	Forks         []int     `json:"fork"`
	Avg           []float32 `json:"avg"`
}

type EventDetails struct {
	Event     *Event     `json:"event,omitempty"`
	Developer *Developer `json:"developer,omitempty"`
}

type EventSearchCriteria struct {
	FromDate *string `json:"from,omitempty"`
	ToDate   *string `json:"to,omitempty"`
	Type     *string `json:"type,omitempty"`
	Org      *string `json:"org,omitempty"`
	Repo     *string `json:"repo,omitempty"`
	Username *string `json:"user,omitempty"`
	Entity   *string `json:"entity,omitempty"`
	Mention  *string `json:"mention,omitempty"`
	Label    *string `json:"label,omitempty"`
	Page     int     `json:"page,omitempty"`
	PageSize int     `json:"page_size,omitempty"`
}

func (c EventSearchCriteria) String() string {
	b, err := json.Marshal(c)
	if err != nil {
		return ""
	}
	return string(b)
}

func optionalLike(s *string) *string {
	if s == nil || *s == "" {
		return nil
	}
	v := fmt.Sprintf("%%%s%%", *s)
	return &v
}

func SearchEvents(db *sql.DB, q *EventSearchCriteria) ([]*EventDetails, error) {
	if db == nil {
		return nil, errDBNotInitialized
	}

	stmt, err := db.Prepare(selectEventSQL)
	if err != nil {
		return nil, errors.Wrap(err, "failed to prepare event search statement")
	}

	offset := (q.Page - 1) * q.PageSize
	rows, err := stmt.Query(q.FromDate, q.ToDate, q.Type, q.Org, q.Repo, q.Username, optionalLike(q.Mention), optionalLike(q.Label), q.Entity, q.PageSize, offset)
	if err != nil && err != sql.ErrNoRows {
		return nil, errors.Wrap(err, "failed to execute event search statement")
	}
	defer rows.Close()

	list := make([]*EventDetails, 0)

	for rows.Next() {
		e := &EventDetails{
			Event:     &Event{},
			Developer: &Developer{},
		}
		var mentions, labels string
		if err := rows.Scan(&e.Event.Org, &e.Event.Repo, &e.Event.Date, &e.Event.Type, &e.Event.URL,
			&mentions, &labels, &e.Developer.Username, &e.Developer.Email, &e.Developer.FullName,
			&e.Developer.AvatarURL, &e.Developer.ProfileURL, &e.Developer.Entity); err != nil {
			return nil, errors.Wrapf(err, "failed to scan row")
		}
		e.Event.Mentions = slice(mentions)
		e.Event.Labels = slice(labels)
		list = append(list, e)
	}

	return list, nil
}

func GetEventTypeSeries(db *sql.DB, org, repo, entity *string, months int) (*EventTypeSeries, error) {
	if db == nil {
		return nil, errDBNotInitialized
	}

	stmt, err := db.Prepare(selectEventTypesSinceSQL)
	if err != nil {
		return nil, errors.Wrap(err, "failed to prepare repo events statement")
	}

	since := time.Now().UTC().AddDate(0, -months, 0).Format("2006-01-02")
	to := time.Now().UTC().Format("2006-01-02")

	rows, err := stmt.Query(since, to,
		EventTypePR, EventTypePRReview, EventTypeIssue, EventTypeIssueComment, EventTypeFork,
		org, repo, entity)
	if err != nil && err != sql.ErrNoRows {
		return nil, errors.Wrap(err, "failed to execute series select statement")
	}
	defer rows.Close()

	series := &EventTypeSeries{
		Dates:         make([]string, 0),
		PRs:           make([]int, 0),
		PRReviews:     make([]int, 0),
		Issues:        make([]int, 0),
		IssueComments: make([]int, 0),
		Forks:         make([]int, 0),
		Avg:           make([]float32, 0),
	}

	var runSum float32 = 0
	var runCount int = 0
	for rows.Next() {
		var date string
		var prs, prComments, issues, issueComments, forks int
		if err := rows.Scan(&date, &prs, &prComments, &issues, &issueComments, &forks); err != nil {
			return nil, errors.Wrapf(err, "failed to scan row")
		}
		series.Dates = append(series.Dates, date)
		series.PRs = append(series.PRs, prs)
		series.PRReviews = append(series.PRReviews, prComments)
		series.Issues = append(series.Issues, issues)
		series.IssueComments = append(series.IssueComments, issueComments)
		series.Forks = append(series.Forks, forks)

		// avg
		runCount++
		runSum += float32(prs + prComments + issues + issueComments + forks)
		series.Avg = append(series.Avg, runSum/float32(len(EventTypes)*runCount))
	}

	return series, nil
}
