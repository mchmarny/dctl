package data

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetContainerActivity_NilDB(t *testing.T) {
	_, err := GetContainerActivity(nil, nil, nil, 6)
	assert.Error(t, err)
}

func TestGetContainerActivity_EmptyDB(t *testing.T) {
	db := setupTestDB(t)
	series, err := GetContainerActivity(db, nil, nil, 6)
	require.NoError(t, err)
	assert.Empty(t, series.Months)
	assert.Empty(t, series.Versions)
}

func TestGetContainerActivity_WithData(t *testing.T) {
	db := setupTestDB(t)

	_, err := db.Exec(`INSERT INTO container_version (org, repo, package, version_id, tag, created_at)
		VALUES
		('org1', 'repo1', 'pkg1', 1, 'v1.0.0', '2025-01-15T10:00:00Z'),
		('org1', 'repo1', 'pkg1', 2, 'v1.1.0', '2025-01-20T10:00:00Z'),
		('org1', 'repo1', 'pkg1', 3, 'v2.0.0', '2025-02-10T10:00:00Z')`)
	require.NoError(t, err)

	series, err := GetContainerActivity(db, nil, nil, 24)
	require.NoError(t, err)
	require.Len(t, series.Months, 2)
	assert.Equal(t, "2025-01", series.Months[0])
	assert.Equal(t, 2, series.Versions[0])
	assert.Equal(t, "2025-02", series.Months[1])
	assert.Equal(t, 1, series.Versions[1])
}

func TestGetContainerActivity_FilterByOrg(t *testing.T) {
	db := setupTestDB(t)

	_, err := db.Exec(`INSERT INTO container_version (org, repo, package, version_id, tag, created_at)
		VALUES
		('org1', 'repo1', 'pkg1', 1, 'v1.0.0', '2025-01-15T10:00:00Z'),
		('org2', 'repo2', 'pkg2', 2, 'v1.0.0', '2025-01-20T10:00:00Z')`)
	require.NoError(t, err)

	org := "org1"
	series, err := GetContainerActivity(db, &org, nil, 24)
	require.NoError(t, err)
	require.Len(t, series.Months, 1)
	assert.Equal(t, 1, series.Versions[0])
}
