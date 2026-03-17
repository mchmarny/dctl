package postgres

import (
	"fmt"

	"github.com/mchmarny/devpulse/pkg/data"
)

const (
	deleteReleaseAssetsSQL = `DELETE FROM release_asset WHERE org = $1 AND repo = $2`
	deleteReleasesSQL      = `DELETE FROM release WHERE org = $1 AND repo = $2`
	deleteEventsSQL        = `DELETE FROM event WHERE org = $1 AND repo = $2`
	deleteRepoMetaSQL      = `DELETE FROM repo_meta WHERE org = $1 AND repo = $2`
	deleteStateSQL         = `DELETE FROM state WHERE org = $1 AND repo = $2`
)

func (s *Store) DeleteRepoData(org, repo string) (*data.DeleteResult, error) {
	if s.db == nil {
		return nil, data.ErrDBNotInitialized
	}

	if org == "" || repo == "" {
		return nil, fmt.Errorf("org and repo are required (got org=%q, repo=%q)", org, repo)
	}

	tx, err := s.db.Begin()
	if err != nil {
		return nil, fmt.Errorf("beginning delete transaction: %w", err)
	}

	result := &data.DeleteResult{Org: org, Repo: repo}

	deletes := []struct {
		sql   string
		field *int64
	}{
		{deleteReleaseAssetsSQL, &result.ReleaseAssets},
		{deleteReleasesSQL, &result.Releases},
		{deleteEventsSQL, &result.Events},
		{deleteRepoMetaSQL, &result.RepoMeta},
		{deleteStateSQL, &result.State},
	}

	for _, d := range deletes {
		res, execErr := tx.Exec(d.sql, org, repo)
		if execErr != nil {
			_ = tx.Rollback()
			return nil, fmt.Errorf("deleting from %s/%s: %w", org, repo, execErr)
		}
		n, raErr := res.RowsAffected()
		if raErr != nil {
			_ = tx.Rollback()
			return nil, fmt.Errorf("getting rows affected: %w", raErr)
		}
		*d.field = n
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("committing delete transaction: %w", err)
	}

	return result, nil
}
