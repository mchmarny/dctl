package data

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetReleaseCadence_EmptyDB(t *testing.T) {
	db := setupTestDB(t)
	series, err := GetReleaseCadence(db, nil, nil, 6)
	require.NoError(t, err)
	assert.Empty(t, series.Months)
}

func TestGetReleaseCadence_NilDB(t *testing.T) {
	_, err := GetReleaseCadence(nil, nil, nil, 6)
	assert.Error(t, err)
}

func TestGetReleaseCadence_WithData(t *testing.T) {
	db := setupTestDB(t)

	_, err := db.Exec(`INSERT INTO release (org, repo, tag, name, published_at, prerelease)
		VALUES
		('org1', 'repo1', 'v1.0.0', 'Release 1', '2025-01-15T00:00:00Z', 0),
		('org1', 'repo1', 'v1.1.0-rc1', 'RC1', '2025-01-20T00:00:00Z', 1),
		('org1', 'repo1', 'v1.1.0', 'Release 1.1', '2025-02-10T00:00:00Z', 0)`)
	require.NoError(t, err)

	series, err := GetReleaseCadence(db, nil, nil, 24)
	require.NoError(t, err)
	require.Len(t, series.Months, 2)

	// January: 2 total (1 stable + 1 prerelease)
	assert.Equal(t, "2025-01", series.Months[0])
	assert.Equal(t, 2, series.Total[0])
	assert.Equal(t, 1, series.Stable[0])

	// February: 1 total, 1 stable
	assert.Equal(t, "2025-02", series.Months[1])
	assert.Equal(t, 1, series.Total[1])
	assert.Equal(t, 1, series.Stable[1])
}

func TestGetReleaseCadence_WithFilter(t *testing.T) {
	db := setupTestDB(t)

	_, err := db.Exec(`INSERT INTO release (org, repo, tag, name, published_at, prerelease)
		VALUES
		('org1', 'repo1', 'v1.0.0', 'Release 1', '2025-01-15T00:00:00Z', 0),
		('org2', 'repo2', 'v2.0.0', 'Release 2', '2025-01-20T00:00:00Z', 0)`)
	require.NoError(t, err)

	org := "org1"
	series, err := GetReleaseCadence(db, &org, nil, 24)
	require.NoError(t, err)
	require.Len(t, series.Months, 1)
	assert.Equal(t, 1, series.Total[0])
}

func TestGetReleaseDownloads_NilDB(t *testing.T) {
	_, err := GetReleaseDownloads(nil, nil, nil, 6)
	assert.Error(t, err)
}

func TestGetReleaseDownloads_EmptyDB(t *testing.T) {
	db := setupTestDB(t)
	series, err := GetReleaseDownloads(db, nil, nil, 6)
	require.NoError(t, err)
	assert.Empty(t, series.Months)
}

func TestGetReleaseDownloads_WithData(t *testing.T) {
	db := setupTestDB(t)

	_, err := db.Exec(`INSERT INTO release (org, repo, tag, name, published_at, prerelease)
		VALUES
		('org1', 'repo1', 'v1.0.0', 'Release 1', '2025-01-15T00:00:00Z', 0),
		('org1', 'repo1', 'v1.1.0', 'Release 1.1', '2025-02-10T00:00:00Z', 0)`)
	require.NoError(t, err)

	_, err = db.Exec(`INSERT INTO release_asset (org, repo, tag, name, content_type, size, download_count)
		VALUES
		('org1', 'repo1', 'v1.0.0', 'app_linux_amd64.tar.gz', 'application/gzip', 1000, 150),
		('org1', 'repo1', 'v1.0.0', 'app_darwin_arm64.tar.gz', 'application/gzip', 1000, 50),
		('org1', 'repo1', 'v1.1.0', 'app_linux_amd64.tar.gz', 'application/gzip', 1000, 30)`)
	require.NoError(t, err)

	series, err := GetReleaseDownloads(db, nil, nil, 24)
	require.NoError(t, err)
	require.Len(t, series.Months, 2)

	assert.Equal(t, "2025-01", series.Months[0])
	assert.Equal(t, 200, series.Downloads[0])

	assert.Equal(t, "2025-02", series.Months[1])
	assert.Equal(t, 30, series.Downloads[1])
}

func TestGetReleaseDownloads_WithFilter(t *testing.T) {
	db := setupTestDB(t)

	_, err := db.Exec(`INSERT INTO release (org, repo, tag, name, published_at, prerelease)
		VALUES
		('org1', 'repo1', 'v1.0.0', 'Release 1', '2025-01-15T00:00:00Z', 0),
		('org2', 'repo2', 'v2.0.0', 'Release 2', '2025-01-20T00:00:00Z', 0)`)
	require.NoError(t, err)

	_, err = db.Exec(`INSERT INTO release_asset (org, repo, tag, name, content_type, size, download_count)
		VALUES
		('org1', 'repo1', 'v1.0.0', 'app.tar.gz', 'application/gzip', 1000, 100),
		('org2', 'repo2', 'v2.0.0', 'app.tar.gz', 'application/gzip', 1000, 200)`)
	require.NoError(t, err)

	org := "org1"
	series, err := GetReleaseDownloads(db, &org, nil, 24)
	require.NoError(t, err)
	require.Len(t, series.Months, 1)
	assert.Equal(t, 100, series.Downloads[0])
}
