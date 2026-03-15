package data

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/go-github/v83/github"
	"github.com/mchmarny/devpulse/pkg/net"
	"github.com/mchmarny/reputer/pkg/score"
)

const (
	reputationStaleHours = 24

	selectStaleReputationUsernamesSQL = `SELECT DISTINCT d.username
		FROM developer d
		JOIN event e ON d.username = e.username
		WHERE d.username NOT LIKE '%[bot]'
		  AND LOWER(d.username) NOT IN ('copilot','github-copilot','claude','anthropic-claude')
		  AND e.org = COALESCE(?, e.org)
		  AND e.repo = COALESCE(?, e.repo)
		  AND (d.reputation IS NULL
		   OR d.reputation_updated_at IS NULL
		   OR d.reputation_updated_at < ?)
	`

	updateReputationSQL = `UPDATE developer
		SET reputation = ?, reputation_updated_at = ?, reputation_deep = ?,
		    reputation_signals = ?
		WHERE username = ?
	`

	selectUserReputationSQL = `SELECT reputation, reputation_signals
		FROM developer
		WHERE username = ?
		  AND reputation IS NOT NULL
		  AND reputation_deep = 1
		  AND reputation_updated_at IS NOT NULL
		  AND reputation_updated_at >= ?
	`

	selectReputationSQL = `SELECT d.username, d.reputation
		FROM developer d
		JOIN event e ON d.username = e.username
		WHERE e.org = COALESCE(?, e.org)
		  AND e.repo = COALESCE(?, e.repo)
		  AND IFNULL(d.entity, '') = COALESCE(?, IFNULL(d.entity, ''))
		  AND e.date >= ?
		  AND d.reputation IS NOT NULL
		  AND d.username NOT LIKE '%[bot]'
		  AND LOWER(d.username) NOT IN ('copilot','github-copilot','claude','anthropic-claude')
		GROUP BY d.username
		ORDER BY d.reputation ASC
		LIMIT 20
	`

	selectDistinctOrgsSQL = `SELECT DISTINCT org FROM event`

	selectLowestReputationUsernamesSQL = `SELECT d.username
		FROM developer d
		JOIN event e ON d.username = e.username
		WHERE d.reputation IS NOT NULL
		  AND d.username NOT LIKE '%[bot]'
		  AND LOWER(d.username) NOT IN ('copilot','github-copilot','claude','anthropic-claude')
		  AND e.org = COALESCE(?, e.org)
		  AND e.repo = COALESCE(?, e.repo)
		  AND (d.reputation_deep IS NULL OR d.reputation_deep = 0
		   OR d.reputation_updated_at IS NULL
		   OR d.reputation_updated_at < ?)
		GROUP BY d.username
		ORDER BY d.reputation ASC
		LIMIT ?
	`

	selectUserCommitCountSQL = `SELECT COUNT(*) FROM event
		WHERE username = ? AND date >= ?
	`

	selectTotalCommitCountSQL = `SELECT COUNT(*) FROM event
		WHERE date >= ?
	`

	selectTotalContributorCountSQL = `SELECT COUNT(DISTINCT username) FROM event
		WHERE date >= ?
	`

	selectLastCommitDateSQL = `SELECT MAX(date) FROM event
		WHERE username = ?
	`
)

// ReputationResult is returned by the shallow bulk import.
type ReputationResult struct {
	Updated int `json:"updated" yaml:"updated"`
	Skipped int `json:"skipped" yaml:"skipped"`
	Errors  int `json:"errors" yaml:"errors"`
}

// DeepReputationResult is returned by the bulk deep scoring step.
type DeepReputationResult struct {
	Scored  int `json:"scored" yaml:"scored"`
	Skipped int `json:"skipped" yaml:"skipped"`
	Errors  int `json:"errors" yaml:"errors"`
}

// ReputationDistribution is the dashboard chart data.
type ReputationDistribution struct {
	Labels []string  `json:"labels" yaml:"labels"`
	Data   []float64 `json:"data" yaml:"data"`
}

// UserReputation is returned by the on-demand deep score endpoint.
type UserReputation struct {
	Username   string         `json:"username" yaml:"username"`
	Reputation float64        `json:"reputation" yaml:"reputation"`
	Deep       bool           `json:"deep" yaml:"deep"`
	Signals    *SignalSummary `json:"signals,omitempty" yaml:"signals,omitempty"`
}

// SignalSummary exposes gathered signals to the UI.
type SignalSummary struct {
	AgeDays           int64  `json:"age_days" yaml:"ageDays"`
	Followers         int64  `json:"followers" yaml:"followers"`
	Following         int64  `json:"following" yaml:"following"`
	PublicRepos       int64  `json:"public_repos" yaml:"publicRepos"`
	Suspended         bool   `json:"suspended" yaml:"suspended"`
	OrgMember         bool   `json:"org_member" yaml:"orgMember"`
	Commits           int64  `json:"commits" yaml:"commits"`
	TotalCommits      int64  `json:"total_commits" yaml:"totalCommits"`
	TotalContributors int    `json:"total_contributors" yaml:"totalContributors"`
	LastCommitDays    int64  `json:"last_commit_days" yaml:"lastCommitDays"`
	AuthorAssociation string `json:"author_association" yaml:"authorAssociation"`
	HasBio            bool   `json:"has_bio" yaml:"hasBio"`
	HasCompany        bool   `json:"has_company" yaml:"hasCompany"`
	HasLocation       bool   `json:"has_location" yaml:"hasLocation"`
	HasWebsite        bool   `json:"has_website" yaml:"hasWebsite"`
	PRsMerged         int64  `json:"prs_merged" yaml:"prsMerged"`
	PRsClosed         int64  `json:"prs_closed" yaml:"prsClosed"`
	RecentPRRepoCount int64  `json:"recent_pr_repo_count" yaml:"recentPRRepoCount"`
	ForkedRepos       int64  `json:"forked_repos" yaml:"forkedRepos"`
	TrustedOrgMember  bool   `json:"trusted_org_member" yaml:"trustedOrgMember"`
}

// globalStats holds DB-wide statistics computed once per import run.
type globalStats struct {
	totalCommits      int64
	totalContributors int
}

// ImportReputation computes shallow (local-only) reputation scores for all
// contributors with stale or missing scores. No GitHub API calls.
func ImportReputation(db *sql.DB, org, repo *string) (*ReputationResult, error) {
	if db == nil {
		return nil, errDBNotInitialized
	}

	threshold := time.Now().UTC().Add(-reputationStaleHours * time.Hour).Format("2006-01-02T15:04:05Z")

	usernames, err := getStaleReputationUsernames(db, org, repo, threshold)
	if err != nil {
		return nil, fmt.Errorf("error getting stale usernames: %w", err)
	}

	if len(usernames) == 0 {
		slog.Info("reputation up to date")
		return &ReputationResult{}, nil
	}

	slog.Info("scoring reputation", "users", len(usernames))

	since := time.Now().UTC().AddDate(0, -EventAgeMonthsDefault, 0).Format("2006-01-02")

	stats, err := computeGlobalStats(db, since)
	if err != nil {
		return nil, fmt.Errorf("error computing global stats: %w", err)
	}

	now := time.Now().UTC().Format("2006-01-02T15:04:05Z")
	res := &ReputationResult{}

	tx, err := db.Begin()
	if err != nil {
		return nil, fmt.Errorf("error starting reputation tx: %w", err)
	}

	stmt, err := tx.Prepare(updateReputationSQL)
	if err != nil {
		rollbackTransaction(tx)
		return nil, fmt.Errorf("error preparing reputation update: %w", err)
	}

	total := len(usernames)
	logEvery := total / 10
	if logEvery < 1 {
		logEvery = 1
	}

	for i, username := range usernames {
		signals := gatherLocalSignals(db, username, since, stats)
		rep := score.Compute(signals)

		if _, execErr := stmt.Exec(rep, now, 0, nil, username); execErr != nil {
			rollbackTransaction(tx)
			return nil, fmt.Errorf("error updating reputation for %s: %w", username, execErr)
		}
		res.Updated++

		if (i+1)%logEvery == 0 {
			slog.Info("reputation progress", "scored", i+1, "total", total)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("error committing reputation tx: %w", err)
	}

	slog.Info("reputation done", "updated", res.Updated)

	return res, nil
}

// ImportDeepReputation deep-scores the N lowest-reputation contributors
// using GitHub API signals. Runs after shallow scoring; errors are
// per-user and never fatal to the overall import.
func ImportDeepReputation(db *sql.DB, token string, limit int, org, repo *string) (*DeepReputationResult, error) {
	if db == nil {
		return nil, errDBNotInitialized
	}

	if token == "" {
		return nil, errors.New("token is required for deep reputation scoring")
	}

	if limit <= 0 {
		return &DeepReputationResult{}, nil
	}

	threshold := time.Now().UTC().Add(-reputationStaleHours * time.Hour).Format("2006-01-02T15:04:05Z")

	usernames, err := getLowestReputationUsernames(db, org, repo, threshold, limit)
	if err != nil {
		return nil, fmt.Errorf("error getting lowest reputation usernames: %w", err)
	}

	if len(usernames) == 0 {
		slog.Info("deep reputation: no candidates")
		return &DeepReputationResult{}, nil
	}

	slog.Info("deep reputation scoring", "candidates", len(usernames))

	res := &DeepReputationResult{}

	for i, username := range usernames {
		slog.Info("reputation", "user", username, "progress", fmt.Sprintf("%d/%d", i+1, len(usernames)))

		if _, deepErr := ComputeDeepReputation(db, token, username); deepErr != nil {
			slog.Error("deep reputation failed", "username", username, "error", deepErr)
			res.Errors++
			continue
		}

		res.Scored++
	}

	slog.Info("deep reputation done", "scored", res.Scored, "errors", res.Errors)

	return res, nil
}

// GetOrComputeDeepReputation returns a cached deep score if fresh (<24h),
// otherwise computes a new one via GitHub API.
func GetOrComputeDeepReputation(db *sql.DB, token, username string) (*UserReputation, error) {
	if db == nil {
		return nil, errDBNotInitialized
	}

	threshold := time.Now().UTC().Add(-reputationStaleHours * time.Hour).Format("2006-01-02T15:04:05Z")

	var rep float64
	var signalsJSON sql.NullString
	err := db.QueryRow(selectUserReputationSQL, username, threshold).Scan(&rep, &signalsJSON)
	if err == nil {
		result := &UserReputation{
			Username:   username,
			Reputation: rep,
			Deep:       false,
		}
		if signalsJSON.Valid && signalsJSON.String != "" {
			var ss SignalSummary
			if jsonErr := json.Unmarshal([]byte(signalsJSON.String), &ss); jsonErr == nil {
				result.Signals = &ss
			}
		}
		return result, nil
	}

	return ComputeDeepReputation(db, token, username)
}

// ComputeDeepReputation scores a single user using GitHub API signals
// and stores the result. Called on-demand from the UI.
func ComputeDeepReputation(db *sql.DB, token, username string) (*UserReputation, error) {
	if db == nil {
		return nil, errDBNotInitialized
	}

	if token == "" || username == "" {
		return nil, errors.New("token and username are required")
	}

	since := time.Now().UTC().AddDate(0, -EventAgeMonthsDefault, 0).Format("2006-01-02")

	stats, err := computeGlobalStats(db, since)
	if err != nil {
		return nil, fmt.Errorf("error computing global stats: %w", err)
	}

	orgs, err := getDistinctOrgs(db)
	if err != nil {
		return nil, fmt.Errorf("error getting distinct orgs: %w", err)
	}

	orgSet := make(map[string]bool, len(orgs))
	for _, o := range orgs {
		orgSet[strings.ToLower(o)] = true
	}

	ctx := context.Background()
	client := github.NewClient(net.GetOAuthClient(ctx, token))

	signals, err := gatherFullSignals(ctx, client, db, username, orgs, orgSet, since, stats)
	if err != nil {
		return nil, fmt.Errorf("error gathering signals for %s: %w", username, err)
	}

	rep := score.Compute(signals)
	now := time.Now().UTC().Format("2006-01-02T15:04:05Z")

	ss := &SignalSummary{
		AgeDays:           signals.AgeDays,
		Followers:         signals.Followers,
		Following:         signals.Following,
		PublicRepos:       signals.PublicRepos,
		Suspended:         signals.Suspended,
		OrgMember:         signals.OrgMember,
		Commits:           signals.Commits,
		TotalCommits:      signals.TotalCommits,
		TotalContributors: signals.TotalContributors,
		LastCommitDays:    signals.LastCommitDays,
		AuthorAssociation: signals.AuthorAssociation,
		HasBio:            signals.HasBio,
		HasCompany:        signals.HasCompany,
		HasLocation:       signals.HasLocation,
		HasWebsite:        signals.HasWebsite,
		PRsMerged:         signals.PRsMerged,
		PRsClosed:         signals.PRsClosed,
		RecentPRRepoCount: signals.RecentPRRepoCount,
		ForkedRepos:       signals.ForkedRepos,
		TrustedOrgMember:  signals.TrustedOrgMember,
	}

	if updateErr := updateReputation(db, username, rep, now, true, ss); updateErr != nil {
		return nil, fmt.Errorf("error storing reputation for %s: %w", username, updateErr)
	}

	return &UserReputation{
		Username:   username,
		Reputation: rep,
		Deep:       true,
		Signals:    ss,
	}, nil
}

func GetReputationDistribution(db *sql.DB, org, repo, entity *string, months int) (*ReputationDistribution, error) {
	if db == nil {
		return nil, errDBNotInitialized
	}

	since := time.Now().UTC().AddDate(0, -months, 0).Format("2006-01-02")

	rows, err := db.Query(selectReputationSQL, org, repo, entity, since)
	if err != nil {
		return nil, fmt.Errorf("failed to query reputation distribution: %w", err)
	}
	defer rows.Close()

	d := &ReputationDistribution{
		Labels: make([]string, 0),
		Data:   make([]float64, 0),
	}

	for rows.Next() {
		var username string
		var rep float64
		if err := rows.Scan(&username, &rep); err != nil {
			return nil, fmt.Errorf("failed to scan reputation row: %w", err)
		}
		d.Labels = append(d.Labels, username)
		d.Data = append(d.Data, rep)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	return d, nil
}

// gatherLocalSignals collects only DB-available signals (no API calls).
func gatherLocalSignals(db *sql.DB, username, since string, stats *globalStats) score.Signals {
	var s score.Signals

	var commits int64
	if err := db.QueryRow(selectUserCommitCountSQL, username, since).Scan(&commits); err != nil && !errors.Is(err, sql.ErrNoRows) {
		slog.Debug("error counting user commits", "username", username, "error", err)
	}
	s.Commits = commits

	s.TotalCommits = stats.totalCommits
	s.TotalContributors = stats.totalContributors

	var lastDate sql.NullString
	if err := db.QueryRow(selectLastCommitDateSQL, username).Scan(&lastDate); err != nil && !errors.Is(err, sql.ErrNoRows) {
		slog.Debug("error getting last commit date", "username", username, "error", err)
	}
	if lastDate.Valid && lastDate.String != "" {
		if t, parseErr := time.Parse("2006-01-02", lastDate.String); parseErr == nil {
			s.LastCommitDays = int64(time.Since(t).Hours() / 24)
		}
	}

	return s
}

// gatherFullSignals collects all signals including GitHub API data.
func gatherFullSignals(ctx context.Context, client *github.Client, db *sql.DB, username string, orgs []string, orgSet map[string]bool, since string, stats *globalStats) (score.Signals, error) {
	s := gatherLocalSignals(db, username, since, stats)

	usr, resp, err := client.Users.Get(ctx, username)
	if err != nil {
		return s, fmt.Errorf("error getting user %s: %w", username, err)
	}
	checkRateLimit(resp)

	if usr.CreatedAt != nil {
		s.AgeDays = int64(time.Since(usr.CreatedAt.Time).Hours() / 24)
	}
	s.Followers = int64(usr.GetFollowers())
	s.Following = int64(usr.GetFollowing())
	s.PublicRepos = int64(usr.GetPublicRepos())
	s.Suspended = usr.SuspendedAt != nil
	s.HasBio = usr.GetBio() != ""
	s.HasCompany = usr.GetCompany() != ""
	s.HasLocation = usr.GetLocation() != ""
	s.HasWebsite = usr.GetBlog() != ""

	// Check org membership: first try profile company field, then API.
	company := strings.ToLower(strings.TrimSpace(strings.ReplaceAll(usr.GetCompany(), "@", "")))
	if company != "" && orgSet[company] {
		s.OrgMember = true
	} else {
		for _, org := range orgs {
			isMember, memberResp, memberErr := client.Organizations.IsMember(ctx, org, username)
			if memberErr != nil {
				slog.Debug("error checking org membership", "org", org, "username", username, "error", memberErr)
				continue
			}
			checkRateLimit(memberResp)
			if isMember {
				s.OrgMember = true
				break
			}
		}
	}

	s.TrustedOrgMember = s.OrgMember

	// Count forked repos.
	var forkedCount int64
	repoOpts := &github.RepositoryListByUserOptions{
		Type:        "owner",
		ListOptions: github.ListOptions{PerPage: 100},
	}
	for {
		repos, repoResp, repoErr := client.Repositories.ListByUser(ctx, username, repoOpts)
		if repoErr != nil {
			slog.Debug("error listing repos for fork count", "username", username, "error", repoErr)
			break
		}
		checkRateLimit(repoResp)
		for _, r := range repos {
			if r.GetFork() {
				forkedCount++
			}
		}
		if repoResp.NextPage == 0 {
			break
		}
		repoOpts.Page = repoResp.NextPage
	}
	s.ForkedRepos = forkedCount

	// PR signals via search API.
	mergedQuery := fmt.Sprintf("type:pr author:%s is:merged", username)
	mergedResult, mergedResp, mergedErr := client.Search.Issues(ctx, mergedQuery, &github.SearchOptions{
		ListOptions: github.ListOptions{PerPage: 1},
	})
	if mergedErr != nil {
		slog.Debug("error searching merged PRs", "username", username, "error", mergedErr)
	} else {
		checkRateLimit(mergedResp)
		s.PRsMerged = int64(mergedResult.GetTotal())
	}

	closedQuery := fmt.Sprintf("type:pr author:%s is:unmerged is:closed", username)
	closedResult, closedResp, closedErr := client.Search.Issues(ctx, closedQuery, &github.SearchOptions{
		ListOptions: github.ListOptions{PerPage: 1},
	})
	if closedErr != nil {
		slog.Debug("error searching closed PRs", "username", username, "error", closedErr)
	} else {
		checkRateLimit(closedResp)
		s.PRsClosed = int64(closedResult.GetTotal())
	}

	cutoff := time.Now().AddDate(0, 0, -90).Format("2006-01-02")
	recentQuery := fmt.Sprintf("type:pr author:%s created:>=%s", username, cutoff)
	recentRepoSet := make(map[string]bool)
	recentOpts := &github.SearchOptions{
		ListOptions: github.ListOptions{PerPage: 100},
	}
	for {
		recentResult, recentResp, recentErr := client.Search.Issues(ctx, recentQuery, recentOpts)
		if recentErr != nil {
			slog.Debug("error searching recent PRs", "username", username, "error", recentErr)
			break
		}
		checkRateLimit(recentResp)
		for _, issue := range recentResult.Issues {
			if repoURL := issue.GetRepositoryURL(); repoURL != "" {
				recentRepoSet[repoURL] = true
			}
		}
		if recentResp.NextPage == 0 {
			break
		}
		recentOpts.Page = recentResp.NextPage
	}
	s.RecentPRRepoCount = int64(len(recentRepoSet))

	return s, nil
}

func computeGlobalStats(db *sql.DB, since string) (*globalStats, error) {
	var s globalStats

	if err := db.QueryRow(selectTotalCommitCountSQL, since).Scan(&s.totalCommits); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("error counting total commits: %w", err)
	}

	if err := db.QueryRow(selectTotalContributorCountSQL, since).Scan(&s.totalContributors); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("error counting total contributors: %w", err)
	}

	return &s, nil
}

func getStaleReputationUsernames(db *sql.DB, org, repo *string, threshold string) ([]string, error) {
	if db == nil {
		return nil, errDBNotInitialized
	}

	rows, err := db.Query(selectStaleReputationUsernamesSQL, org, repo, threshold)
	if err != nil {
		return nil, fmt.Errorf("failed to query stale reputation usernames: %w", err)
	}
	defer rows.Close()

	list := make([]string, 0)
	for rows.Next() {
		var username string
		if err := rows.Scan(&username); err != nil {
			return nil, fmt.Errorf("failed to scan username: %w", err)
		}
		list = append(list, username)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	return list, nil
}

func getLowestReputationUsernames(db *sql.DB, org, repo *string, threshold string, limit int) ([]string, error) {
	if db == nil {
		return nil, errDBNotInitialized
	}

	rows, err := db.Query(selectLowestReputationUsernamesSQL, org, repo, threshold, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query lowest reputation usernames: %w", err)
	}
	defer rows.Close()

	list := make([]string, 0, limit)
	for rows.Next() {
		var username string
		if err := rows.Scan(&username); err != nil {
			return nil, fmt.Errorf("failed to scan username: %w", err)
		}
		list = append(list, username)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	return list, nil
}

func updateReputation(db *sql.DB, username string, reputation float64, updatedAt string, deep bool, signals *SignalSummary) error {
	if db == nil {
		return errDBNotInitialized
	}

	deepVal := 0
	if deep {
		deepVal = 1
	}

	var signalsJSON *string
	if signals != nil {
		b, err := json.Marshal(signals)
		if err != nil {
			return fmt.Errorf("failed to marshal signals for %s: %w", username, err)
		}
		s := string(b)
		signalsJSON = &s
	}

	_, err := db.Exec(updateReputationSQL, reputation, updatedAt, deepVal, signalsJSON, username)
	if err != nil {
		return fmt.Errorf("failed to update reputation for %s: %w", username, err)
	}

	return nil
}

func getDistinctOrgs(db *sql.DB) ([]string, error) {
	if db == nil {
		return nil, errDBNotInitialized
	}

	rows, err := db.Query(selectDistinctOrgsSQL)
	if err != nil {
		return nil, fmt.Errorf("failed to query distinct orgs: %w", err)
	}
	defer rows.Close()

	list := make([]string, 0)
	for rows.Next() {
		var org string
		if err := rows.Scan(&org); err != nil {
			return nil, fmt.Errorf("failed to scan org: %w", err)
		}
		list = append(list, org)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	return list, nil
}
