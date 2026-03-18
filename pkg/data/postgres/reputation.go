package postgres

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
	"github.com/mchmarny/devpulse/pkg/data"
	"github.com/mchmarny/devpulse/pkg/data/ghutil"
	"github.com/mchmarny/devpulse/pkg/net"
	"github.com/mchmarny/reputer/pkg/score"
)

const (
	reputationStaleHours = 24

	// selectStaleReputationUsernamesSQL: $1=org, $2=repo, $3=threshold
	selectStaleReputationUsernamesSQL = `SELECT DISTINCT d.username
		FROM developer d
		JOIN event e ON d.username = e.username
		WHERE 1=1
		  ` + botExcludeDSQL + `
		  AND e.org = COALESCE($1, e.org)
		  AND e.repo = COALESCE($2, e.repo)
		  AND (d.reputation IS NULL
		   OR d.reputation_updated_at IS NULL
		   OR d.reputation_updated_at < $3)
	`

	// updateReputationSQL: $1=reputation, $2=updated_at, $3=deep, $4=signals, $5=username
	updateReputationSQL = `UPDATE developer
		SET reputation = $1, reputation_updated_at = $2, reputation_deep = $3,
		    reputation_signals = $4
		WHERE username = $5
	`

	// selectUserReputationSQL: $1=username, $2=threshold
	selectUserReputationSQL = `SELECT reputation, reputation_signals
		FROM developer
		WHERE username = $1
		  AND reputation IS NOT NULL
		  AND reputation_deep = 1
		  AND reputation_updated_at IS NOT NULL
		  AND reputation_updated_at >= $2
	`

	// selectReputationSQL: $1=org, $2=repo, $3=entity, $4=since
	selectReputationSQL = `SELECT d.username, d.reputation
		FROM developer d
		JOIN event e ON d.username = e.username
		WHERE e.org = COALESCE($1, e.org)
		  AND e.repo = COALESCE($2, e.repo)
		  AND COALESCE(d.entity, '') = COALESCE($3, COALESCE(d.entity, ''))
		  AND e.date >= $4
		  AND d.reputation IS NOT NULL
		  ` + botExcludeDSQL + `
		GROUP BY d.username, d.reputation
		ORDER BY d.reputation ASC
		LIMIT 10
	`

	// selectReputationCountSQL: $1=org, $2=repo, $3=entity, $4=since
	selectReputationCountSQL = `SELECT
		COUNT(DISTINCT e.username) AS total,
		COUNT(DISTINCT CASE WHEN d.reputation IS NOT NULL THEN e.username END) AS scored
		FROM event e
		JOIN developer d ON e.username = d.username
		WHERE e.org = COALESCE($1, e.org)
		  AND e.repo = COALESCE($2, e.repo)
		  AND COALESCE(d.entity, '') = COALESCE($3, COALESCE(d.entity, ''))
		  AND e.date >= $4
		  ` + botExcludeDSQL + `
	`

	selectDistinctOrgsSQL = `SELECT DISTINCT org FROM event`

	// selectLowestReputationUsernamesSQL: $1=org, $2=repo, $3=threshold, $4=limit
	selectLowestReputationUsernamesSQL = `SELECT d.username
		FROM developer d
		JOIN event e ON d.username = e.username
		WHERE d.reputation IS NOT NULL
		  ` + botExcludeDSQL + `
		  AND e.org = COALESCE($1, e.org)
		  AND e.repo = COALESCE($2, e.repo)
		  AND (d.reputation_deep IS NULL OR d.reputation_deep = 0
		   OR d.reputation_updated_at IS NULL
		   OR d.reputation_updated_at < $3)
		GROUP BY d.username, d.reputation
		ORDER BY d.reputation ASC
		LIMIT $4
	`

	// selectUserCommitCountSQL: $1=username, $2=since
	selectUserCommitCountSQL = `SELECT COUNT(*) FROM event
		WHERE username = $1 AND date >= $2
	`

	// selectTotalCommitCountSQL: $1=since
	selectTotalCommitCountSQL = `SELECT COUNT(*) FROM event
		WHERE date >= $1
	`

	// selectTotalContributorCountSQL: $1=since
	selectTotalContributorCountSQL = `SELECT COUNT(DISTINCT username) FROM event
		WHERE date >= $1
	`

	// selectLastCommitDateSQL: $1=username
	selectLastCommitDateSQL = `SELECT MAX(date) FROM event
		WHERE username = $1
	`
)

type globalStats struct {
	totalCommits      int64
	totalContributors int
}

func (s *Store) ImportReputation(org, repo *string) (*data.ReputationResult, error) {
	if s.db == nil {
		return nil, data.ErrDBNotInitialized
	}

	threshold := time.Now().UTC().Add(-reputationStaleHours * time.Hour).Format("2006-01-02T15:04:05Z")

	usernames, err := s.getStaleReputationUsernames(org, repo, threshold)
	if err != nil {
		return nil, fmt.Errorf("error getting stale usernames: %w", err)
	}

	if len(usernames) == 0 {
		slog.Debug("reputation up to date")
		return &data.ReputationResult{}, nil
	}

	slog.Info("scoring reputation", "users", len(usernames))

	since := sinceDate(data.EventAgeMonthsDefault)

	stats, err := s.computeGlobalStats(since)
	if err != nil {
		return nil, fmt.Errorf("error computing global stats: %w", err)
	}

	now := time.Now().UTC().Format("2006-01-02T15:04:05Z")
	res := &data.ReputationResult{}

	tx, err := s.db.Begin()
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
		signals := s.gatherLocalSignals(username, since, stats)
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

func (s *Store) ImportDeepReputation(ctx context.Context, tokenFn data.TokenFunc, limit, staleHours int, org, repo *string) (*data.DeepReputationResult, error) {
	if s.db == nil {
		return nil, data.ErrDBNotInitialized
	}

	if tokenFn == nil || tokenFn() == "" {
		return nil, errors.New("token is required for deep reputation scoring")
	}

	if limit <= 0 {
		return &data.DeepReputationResult{}, nil
	}

	if staleHours <= 0 {
		staleHours = reputationStaleHours
	}

	threshold := time.Now().UTC().Add(-time.Duration(staleHours) * time.Hour).Format("2006-01-02T15:04:05Z")

	usernames, err := s.getLowestReputationUsernames(org, repo, threshold, limit)
	if err != nil {
		return nil, fmt.Errorf("error getting lowest reputation usernames: %w", err)
	}

	if len(usernames) == 0 {
		slog.Info("deep reputation: no candidates")
		return &data.DeepReputationResult{}, nil
	}

	slog.Info("deep reputation scoring", "candidates", len(usernames))

	res := &data.DeepReputationResult{}

	for i, username := range usernames {
		slog.Info("reputation", "user", username, "progress", fmt.Sprintf("%d/%d", i+1, len(usernames)))

		if _, deepErr := s.ComputeDeepReputation(ctx, tokenFn(), username); deepErr != nil {
			slog.Error("deep reputation failed", "username", username, "error", deepErr)
			res.Errors++
			continue
		}

		res.Scored++
	}

	slog.Info("deep reputation done", "scored", res.Scored, "errors", res.Errors)

	return res, nil
}

func (s *Store) GetOrComputeDeepReputation(ctx context.Context, token, username string) (*data.UserReputation, error) {
	if s.db == nil {
		return nil, data.ErrDBNotInitialized
	}

	threshold := time.Now().UTC().Add(-reputationStaleHours * time.Hour).Format("2006-01-02T15:04:05Z")

	var rep float64
	var signalsJSON sql.NullString
	err := s.db.QueryRow(selectUserReputationSQL, username, threshold).Scan(&rep, &signalsJSON)
	if err == nil {
		result := &data.UserReputation{
			Username:   username,
			Reputation: rep,
			Deep:       false,
		}
		if signalsJSON.Valid && signalsJSON.String != "" {
			var ss data.SignalSummary
			if jsonErr := json.Unmarshal([]byte(signalsJSON.String), &ss); jsonErr == nil {
				result.Signals = &ss
			}
		}
		return result, nil
	}

	return s.ComputeDeepReputation(ctx, token, username)
}

func (s *Store) ComputeDeepReputation(ctx context.Context, token, username string) (*data.UserReputation, error) {
	if s.db == nil {
		return nil, data.ErrDBNotInitialized
	}

	if token == "" || username == "" {
		return nil, errors.New("token and username are required")
	}

	since := sinceDate(data.EventAgeMonthsDefault)

	stats, err := s.computeGlobalStats(since)
	if err != nil {
		return nil, fmt.Errorf("error computing global stats: %w", err)
	}

	orgs, err := s.getDistinctOrgs()
	if err != nil {
		return nil, fmt.Errorf("error getting distinct orgs: %w", err)
	}

	orgSet := make(map[string]bool, len(orgs))
	for _, o := range orgs {
		orgSet[strings.ToLower(o)] = true
	}

	client := github.NewClient(net.GetOAuthClient(ctx, token))

	signals, err := s.gatherFullSignals(ctx, client, username, orgs, orgSet, since, stats)
	if err != nil {
		return nil, fmt.Errorf("error gathering signals for %s: %w", username, err)
	}

	rep := score.Compute(signals)
	now := time.Now().UTC().Format("2006-01-02T15:04:05Z")

	ss := &data.SignalSummary{
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

	if updateErr := s.updateReputation(username, rep, now, true, ss); updateErr != nil {
		return nil, fmt.Errorf("error storing reputation for %s: %w", username, updateErr)
	}

	return &data.UserReputation{
		Username:   username,
		Reputation: rep,
		Deep:       true,
		Signals:    ss,
	}, nil
}

func (s *Store) GetReputationDistribution(org, repo, entity *string, months int) (*data.ReputationDistribution, error) {
	if s.db == nil {
		return nil, data.ErrDBNotInitialized
	}

	since := sinceDate(months)

	rows, err := s.db.Query(selectReputationSQL, org, repo, entity, since)
	if err != nil {
		return nil, fmt.Errorf("failed to query reputation distribution: %w", err)
	}
	defer rows.Close()

	d := &data.ReputationDistribution{
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

	if err := s.db.QueryRow(selectReputationCountSQL, org, repo, entity, since).Scan(&d.Total, &d.Scored); err != nil {
		return nil, fmt.Errorf("failed to query reputation counts: %w", err)
	}

	return d, nil
}

func (s *Store) gatherLocalSignals(username, since string, stats *globalStats) score.Signals {
	var sig score.Signals

	var commits int64
	if err := s.db.QueryRow(selectUserCommitCountSQL, username, since).Scan(&commits); err != nil && !errors.Is(err, sql.ErrNoRows) {
		slog.Debug("error counting user commits", "username", username, "error", err)
	}
	sig.Commits = commits

	sig.TotalCommits = stats.totalCommits
	sig.TotalContributors = stats.totalContributors

	var lastDate sql.NullString
	if err := s.db.QueryRow(selectLastCommitDateSQL, username).Scan(&lastDate); err != nil && !errors.Is(err, sql.ErrNoRows) {
		slog.Debug("error getting last commit date", "username", username, "error", err)
	}
	if lastDate.Valid && lastDate.String != "" {
		if t, parseErr := time.Parse("2006-01-02", lastDate.String); parseErr == nil {
			sig.LastCommitDays = int64(time.Since(t).Hours() / 24)
		}
	}

	return sig
}

func (s *Store) gatherFullSignals(ctx context.Context, client *github.Client, username string, orgs []string, orgSet map[string]bool, since string, stats *globalStats) (score.Signals, error) {
	sig := s.gatherLocalSignals(username, since, stats)

	usr, resp, err := client.Users.Get(ctx, username)
	if err != nil {
		return sig, fmt.Errorf("error getting user %s: %w", username, err)
	}
	ghutil.CheckRateLimit(resp)

	if usr.CreatedAt != nil {
		sig.AgeDays = int64(time.Since(usr.CreatedAt.Time).Hours() / 24)
	}
	sig.Followers = int64(usr.GetFollowers())
	sig.Following = int64(usr.GetFollowing())
	sig.PublicRepos = int64(usr.GetPublicRepos())
	sig.Suspended = usr.SuspendedAt != nil
	sig.HasBio = usr.GetBio() != ""
	sig.HasCompany = usr.GetCompany() != ""
	sig.HasLocation = usr.GetLocation() != ""
	sig.HasWebsite = usr.GetBlog() != ""

	company := strings.ToLower(strings.TrimSpace(strings.ReplaceAll(usr.GetCompany(), "@", "")))
	if company != "" && orgSet[company] {
		sig.OrgMember = true
	} else {
		for _, org := range orgs {
			isMember, memberResp, memberErr := client.Organizations.IsMember(ctx, org, username)
			if memberErr != nil {
				slog.Debug("error checking org membership", "org", org, "username", username, "error", memberErr)
				continue
			}
			ghutil.CheckRateLimit(memberResp)
			if isMember {
				sig.OrgMember = true
				break
			}
		}
	}

	sig.TrustedOrgMember = sig.OrgMember

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
		ghutil.CheckRateLimit(repoResp)
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
	sig.ForkedRepos = forkedCount

	mergedQuery := fmt.Sprintf("type:pr author:%s is:merged", username)
	mergedResult, mergedResp, mergedErr := client.Search.Issues(ctx, mergedQuery, &github.SearchOptions{
		ListOptions: github.ListOptions{PerPage: 1},
	})
	if mergedErr != nil {
		slog.Debug("error searching merged PRs", "username", username, "error", mergedErr)
	} else {
		ghutil.CheckRateLimit(mergedResp)
		sig.PRsMerged = int64(mergedResult.GetTotal())
	}

	closedQuery := fmt.Sprintf("type:pr author:%s is:unmerged is:closed", username)
	closedResult, closedResp, closedErr := client.Search.Issues(ctx, closedQuery, &github.SearchOptions{
		ListOptions: github.ListOptions{PerPage: 1},
	})
	if closedErr != nil {
		slog.Debug("error searching closed PRs", "username", username, "error", closedErr)
	} else {
		ghutil.CheckRateLimit(closedResp)
		sig.PRsClosed = int64(closedResult.GetTotal())
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
		ghutil.CheckRateLimit(recentResp)
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
	sig.RecentPRRepoCount = int64(len(recentRepoSet))

	return sig, nil
}

func (s *Store) computeGlobalStats(since string) (*globalStats, error) {
	var gs globalStats

	if err := s.db.QueryRow(selectTotalCommitCountSQL, since).Scan(&gs.totalCommits); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("error counting total commits: %w", err)
	}

	if err := s.db.QueryRow(selectTotalContributorCountSQL, since).Scan(&gs.totalContributors); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("error counting total contributors: %w", err)
	}

	return &gs, nil
}

func (s *Store) getStaleReputationUsernames(org, repo *string, threshold string) ([]string, error) {
	if s.db == nil {
		return nil, data.ErrDBNotInitialized
	}

	rows, err := s.db.Query(selectStaleReputationUsernamesSQL, org, repo, threshold)
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

func (s *Store) getLowestReputationUsernames(org, repo *string, threshold string, limit int) ([]string, error) {
	if s.db == nil {
		return nil, data.ErrDBNotInitialized
	}

	rows, err := s.db.Query(selectLowestReputationUsernamesSQL, org, repo, threshold, limit)
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

func (s *Store) updateReputation(username string, reputation float64, updatedAt string, deep bool, signals *data.SignalSummary) error {
	if s.db == nil {
		return data.ErrDBNotInitialized
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
		str := string(b)
		signalsJSON = &str
	}

	_, err := s.db.Exec(updateReputationSQL, reputation, updatedAt, deepVal, signalsJSON, username)
	if err != nil {
		return fmt.Errorf("failed to update reputation for %s: %w", username, err)
	}

	return nil
}

func (s *Store) getDistinctOrgs() ([]string, error) {
	if s.db == nil {
		return nil, data.ErrDBNotInitialized
	}

	rows, err := s.db.Query(selectDistinctOrgsSQL)
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
