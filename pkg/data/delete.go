package data

import (
	"database/sql"
	"fmt"
)

const (
	deleteReleaseAssets = `DELETE FROM release_asset WHERE org = ? AND repo = ?`
	deleteReleases      = `DELETE FROM release WHERE org = ? AND repo = ?`
	deleteEvents        = `DELETE FROM event WHERE org = ? AND repo = ?`
	deleteRepoMeta      = `DELETE FROM repo_meta WHERE org = ? AND repo = ?`
	deleteState         = `DELETE FROM state WHERE org = ? AND repo = ?`
)

func DeleteRepoData(db *sql.DB, org, repo string) (*DeleteResult, error) {
	if db == nil {
		return nil, errDBNotInitialized
	}

	if org == "" || repo == "" {
		return nil, fmt.Errorf("org and repo are required (got org=%q, repo=%q)", org, repo)
	}

	tx, err := db.Begin()
	if err != nil {
		return nil, fmt.Errorf("beginning delete transaction: %w", err)
	}

	result := &DeleteResult{Org: org, Repo: repo}

	deletes := []struct {
		sql   string
		field *int64
	}{
		{deleteReleaseAssets, &result.ReleaseAssets},
		{deleteReleases, &result.Releases},
		{deleteEvents, &result.Events},
		{deleteRepoMeta, &result.RepoMeta},
		{deleteState, &result.State},
	}

	for _, d := range deletes {
		res, execErr := tx.Exec(d.sql, org, repo)
		if execErr != nil {
			_ = tx.Rollback()
			return nil, fmt.Errorf("deleting from %s/%s: %w", org, repo, execErr)
		}
		n, _ := res.RowsAffected()
		*d.field = n
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("committing delete transaction: %w", err)
	}

	return result, nil
}
