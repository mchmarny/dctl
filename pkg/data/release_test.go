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

func TestGetReleaseDownloadsByTag_NilDB(t *testing.T) {
	_, err := GetReleaseDownloadsByTag(nil, nil, nil, 6)
	assert.Error(t, err)
}

func TestGetReleaseDownloadsByTag_EmptyDB(t *testing.T) {
	db := setupTestDB(t)
	series, err := GetReleaseDownloadsByTag(db, nil, nil, 6)
	require.NoError(t, err)
	assert.Empty(t, series.Tags)
}

func TestGetReleaseDownloadsByTag_WithData(t *testing.T) {
	db := setupTestDB(t)

	// Insert 11 releases spanning recent months
	_, err := db.Exec(`INSERT INTO release (org, repo, tag, name, published_at, prerelease) VALUES
		('org1', 'repo1', 'v0.1.0', 'R0.1', '2025-01-01T00:00:00Z', 0),
		('org1', 'repo1', 'v0.2.0', 'R0.2', '2025-02-01T00:00:00Z', 0),
		('org1', 'repo1', 'v0.3.0', 'R0.3', '2025-03-01T00:00:00Z', 0),
		('org1', 'repo1', 'v0.4.0', 'R0.4', '2025-04-01T00:00:00Z', 0),
		('org1', 'repo1', 'v0.5.0', 'R0.5', '2025-05-01T00:00:00Z', 0),
		('org1', 'repo1', 'v0.6.0', 'R0.6', '2025-06-01T00:00:00Z', 0),
		('org1', 'repo1', 'v0.7.0', 'R0.7', '2025-07-01T00:00:00Z', 0),
		('org1', 'repo1', 'v0.8.0', 'R0.8', '2025-08-01T00:00:00Z', 0),
		('org1', 'repo1', 'v0.9.0', 'R0.9', '2025-09-01T00:00:00Z', 0),
		('org1', 'repo1', 'v1.0.0', 'R1.0', '2025-10-01T00:00:00Z', 0),
		('org1', 'repo1', 'v1.1.0', 'R1.1', '2025-11-01T00:00:00Z', 0)`)
	require.NoError(t, err)

	// v0.1.0 is the all-time top by downloads (5000), but NOT in the last 9 by date
	_, err = db.Exec(`INSERT INTO release_asset (org, repo, tag, name, content_type, size, download_count) VALUES
		('org1', 'repo1', 'v0.1.0', 'app.tar.gz', 'application/gzip', 100, 5000),
		('org1', 'repo1', 'v0.2.0', 'app.tar.gz', 'application/gzip', 100, 10),
		('org1', 'repo1', 'v0.3.0', 'app.tar.gz', 'application/gzip', 100, 20),
		('org1', 'repo1', 'v0.4.0', 'app.tar.gz', 'application/gzip', 100, 30),
		('org1', 'repo1', 'v0.5.0', 'app.tar.gz', 'application/gzip', 100, 40),
		('org1', 'repo1', 'v0.6.0', 'app.tar.gz', 'application/gzip', 100, 50),
		('org1', 'repo1', 'v0.7.0', 'app.tar.gz', 'application/gzip', 100, 60),
		('org1', 'repo1', 'v0.8.0', 'app.tar.gz', 'application/gzip', 100, 70),
		('org1', 'repo1', 'v0.9.0', 'app.tar.gz', 'application/gzip', 100, 80),
		('org1', 'repo1', 'v1.0.0', 'app.tar.gz', 'application/gzip', 100, 90),
		('org1', 'repo1', 'v1.1.0', 'app.tar.gz', 'application/gzip', 100, 100)`)
	require.NoError(t, err)

	series, err := GetReleaseDownloadsByTag(db, nil, nil, 24)
	require.NoError(t, err)

	// 9 recent (v0.3.0..v1.1.0) + top (v0.1.0) = 10
	require.Len(t, series.Tags, 10)

	// First entry should be v0.1.0 (oldest by published_at, pulled in as top)
	assert.Equal(t, "v0.1.0", series.Tags[0])
	assert.Equal(t, 5000, series.Downloads[0])

	// Last entry should be v1.1.0 (most recent)
	assert.Equal(t, "v1.1.0", series.Tags[9])
	assert.Equal(t, 100, series.Downloads[9])
}

func TestGetReleaseDownloadsByTag_Dedup(t *testing.T) {
	db := setupTestDB(t)

	// Only 3 releases — the top downloaded IS in the recent 9
	_, err := db.Exec(`INSERT INTO release (org, repo, tag, name, published_at, prerelease) VALUES
		('org1', 'repo1', 'v1.0.0', 'R1', '2025-10-01T00:00:00Z', 0),
		('org1', 'repo1', 'v1.1.0', 'R2', '2025-11-01T00:00:00Z', 0),
		('org1', 'repo1', 'v1.2.0', 'R3', '2025-12-01T00:00:00Z', 0)`)
	require.NoError(t, err)

	_, err = db.Exec(`INSERT INTO release_asset (org, repo, tag, name, content_type, size, download_count) VALUES
		('org1', 'repo1', 'v1.0.0', 'app.tar.gz', 'application/gzip', 100, 10),
		('org1', 'repo1', 'v1.1.0', 'app.tar.gz', 'application/gzip', 100, 500),
		('org1', 'repo1', 'v1.2.0', 'app.tar.gz', 'application/gzip', 100, 20)`)
	require.NoError(t, err)

	series, err := GetReleaseDownloadsByTag(db, nil, nil, 24)
	require.NoError(t, err)

	// Only 3 entries — no duplicate for v1.1.0
	require.Len(t, series.Tags, 3)
}
