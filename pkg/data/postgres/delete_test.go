package postgres

import (
	"testing"

	"github.com/mchmarny/devpulse/pkg/data"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDeleteRepoData_NilDB(t *testing.T) {
	s := &Store{db: nil}
	_, err := s.DeleteRepoData("org", "repo")
	assert.ErrorIs(t, err, data.ErrDBNotInitialized)
}

func TestDeleteRepoData_EmptyDB(t *testing.T) {
	store := setupTestDB(t)
	result, err := store.DeleteRepoData("org", "repo")
	require.NoError(t, err)
	assert.Equal(t, "org", result.Org)
	assert.Equal(t, "repo", result.Repo)
	assert.Equal(t, int64(0), result.Events)
	assert.Equal(t, int64(0), result.RepoMeta)
	assert.Equal(t, int64(0), result.Releases)
	assert.Equal(t, int64(0), result.ReleaseAssets)
	assert.Equal(t, int64(0), result.State)
}

func TestDeleteRepoData_WithData(t *testing.T) {
	store := setupTestDB(t)

	// Insert a developer (required by event FK)
	_, err := store.db.Exec(`INSERT INTO developer (username, full_name) VALUES ('testuser', 'Test User')`)
	require.NoError(t, err)

	// Insert events
	_, err = store.db.Exec(`INSERT INTO event (org, repo, username, type, date, url, mentions, labels) VALUES
		('myorg', 'myrepo', 'testuser', 'pr', '2025-01-01', 'http://example.com/1', '', ''),
		('myorg', 'myrepo', 'testuser', 'issue', '2025-01-02', 'http://example.com/2', '', ''),
		('myorg', 'otherrepo', 'testuser', 'pr', '2025-01-03', 'http://example.com/3', '', '')`)
	require.NoError(t, err)

	// Insert repo_meta
	_, err = store.db.Exec(`INSERT INTO repo_meta (org, repo, stars, forks, open_issues) VALUES
		('myorg', 'myrepo', 10, 5, 2),
		('myorg', 'otherrepo', 20, 10, 4)`)
	require.NoError(t, err)

	// Insert releases and assets
	_, err = store.db.Exec(`INSERT INTO release (org, repo, tag, name, published_at, prerelease) VALUES
		('myorg', 'myrepo', 'v1.0', 'Release 1', '2025-01-01', 0)`)
	require.NoError(t, err)
	_, err = store.db.Exec(`INSERT INTO release_asset (org, repo, tag, name, content_type, size, download_count) VALUES
		('myorg', 'myrepo', 'v1.0', 'binary.tar.gz', 'application/gzip', 1024, 50),
		('myorg', 'myrepo', 'v1.0', 'checksums.txt', 'text/plain', 256, 30)`)
	require.NoError(t, err)

	// Insert state
	_, err = store.db.Exec(`INSERT INTO state (query, org, repo, page, since) VALUES
		('pr', 'myorg', 'myrepo', 5, 1700000000)`)
	require.NoError(t, err)

	// Delete myorg/myrepo
	result, err := store.DeleteRepoData("myorg", "myrepo")
	require.NoError(t, err)

	assert.Equal(t, int64(2), result.Events)
	assert.Equal(t, int64(1), result.RepoMeta)
	assert.Equal(t, int64(1), result.Releases)
	assert.Equal(t, int64(2), result.ReleaseAssets)
	assert.Equal(t, int64(1), result.State)

	// Verify otherrepo data is untouched
	var count int
	err = store.db.QueryRow(`SELECT COUNT(*) FROM event WHERE org = 'myorg' AND repo = 'otherrepo'`).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 1, count)

	// Verify developer is NOT deleted
	err = store.db.QueryRow(`SELECT COUNT(*) FROM developer WHERE username = 'testuser'`).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 1, count)
}

func TestDeleteRepoData_EmptyParams(t *testing.T) {
	store := setupTestDB(t)

	_, err := store.DeleteRepoData("", "repo")
	assert.Error(t, err)

	_, err = store.DeleteRepoData("org", "")
	assert.Error(t, err)
}
