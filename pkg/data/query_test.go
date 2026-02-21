package data

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSearchEvents(t *testing.T) {
	db := setupTestDB(t)
	seedTestData(t, db)
	q := &EventSearchCriteria{PageSize: 10, Page: 1}
	results, err := SearchEvents(db, q)
	require.NoError(t, err)
	assert.Len(t, results, 3)
}

func TestSearchEvents_ByType(t *testing.T) {
	db := setupTestDB(t)
	seedTestData(t, db)
	prType := EventTypePR
	q := &EventSearchCriteria{Type: &prType, PageSize: 10, Page: 1}
	results, err := SearchEvents(db, q)
	require.NoError(t, err)
	assert.Len(t, results, 2)
}

func TestSearchEvents_NilDB(t *testing.T) {
	q := &EventSearchCriteria{PageSize: 10, Page: 1}
	_, err := SearchEvents(nil, q)
	assert.Error(t, err)
}

func TestEventSearchCriteria_String(t *testing.T) {
	q := EventSearchCriteria{PageSize: 10, Page: 1}
	s := q.String()
	assert.Contains(t, s, "page_size")
}

func TestSearchEvents_ByOrg(t *testing.T) {
	db := setupTestDB(t)
	seedTestData(t, db)
	org := "testorg"
	q := &EventSearchCriteria{Org: &org, PageSize: 10, Page: 1}
	results, err := SearchEvents(db, q)
	require.NoError(t, err)
	assert.Len(t, results, 3)
}

func TestSearchEvents_ByUsername(t *testing.T) {
	db := setupTestDB(t)
	seedTestData(t, db)
	user := "dev1"
	q := &EventSearchCriteria{Username: &user, PageSize: 10, Page: 1}
	results, err := SearchEvents(db, q)
	require.NoError(t, err)
	assert.Len(t, results, 1)
}

func TestSearchEvents_ByEntity(t *testing.T) {
	db := setupTestDB(t)
	seedTestData(t, db)
	entity := "GOOGLE"
	q := &EventSearchCriteria{Entity: &entity, PageSize: 10, Page: 1}
	results, err := SearchEvents(db, q)
	require.NoError(t, err)
	assert.Len(t, results, 2)
}

func TestSearchEvents_ByMention(t *testing.T) {
	db := setupTestDB(t)
	seedTestData(t, db)
	mention := "dev1"
	q := &EventSearchCriteria{Mention: &mention, PageSize: 10, Page: 1}
	results, err := SearchEvents(db, q)
	require.NoError(t, err)
	assert.Len(t, results, 1)
}

func TestSearchEvents_ByLabel(t *testing.T) {
	db := setupTestDB(t)
	seedTestData(t, db)
	label := "bug"
	q := &EventSearchCriteria{Label: &label, PageSize: 10, Page: 1}
	results, err := SearchEvents(db, q)
	require.NoError(t, err)
	assert.Len(t, results, 1)
}

func TestSearchEvents_Pagination(t *testing.T) {
	db := setupTestDB(t)
	seedTestData(t, db)
	q := &EventSearchCriteria{PageSize: 2, Page: 1}
	results, err := SearchEvents(db, q)
	require.NoError(t, err)
	assert.Len(t, results, 2)

	q.Page = 2
	results, err = SearchEvents(db, q)
	require.NoError(t, err)
	assert.Len(t, results, 1)
}

func TestGetEventTypeSeries_NilDB(t *testing.T) {
	_, err := GetEventTypeSeries(nil, nil, nil, nil, 6)
	assert.Error(t, err)
}

func TestGetEventTypeSeries(t *testing.T) {
	db := setupTestDB(t)
	seedTestData(t, db)
	org := "testorg"
	repo := "testrepo"
	series, err := GetEventTypeSeries(db, &org, &repo, nil, 24)
	require.NoError(t, err)
	assert.NotNil(t, series)
	assert.NotEmpty(t, series.Dates)
}

func TestOptionalLike(t *testing.T) {
	s := "test"
	result := optionalLike(&s)
	assert.NotNil(t, result)
	assert.Equal(t, "%test%", *result)

	assert.Nil(t, optionalLike(nil))

	empty := ""
	assert.Nil(t, optionalLike(&empty))
}
