package postgres

import (
	"testing"

	"github.com/mchmarny/devpulse/pkg/data"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSearchEvents(t *testing.T) {
	store := setupTestDB(t)
	seedTestData(t, store)
	q := &data.EventSearchCriteria{PageSize: 10, Page: 1}
	results, err := store.SearchEvents(q)
	require.NoError(t, err)
	assert.Len(t, results, 3)
}

func TestSearchEvents_ByType(t *testing.T) {
	store := setupTestDB(t)
	seedTestData(t, store)
	prType := data.EventTypePR
	q := &data.EventSearchCriteria{Type: &prType, PageSize: 10, Page: 1}
	results, err := store.SearchEvents(q)
	require.NoError(t, err)
	assert.Len(t, results, 2)
}

func TestSearchEvents_NilDB(t *testing.T) {
	s := &Store{db: nil}
	q := &data.EventSearchCriteria{PageSize: 10, Page: 1}
	_, err := s.SearchEvents(q)
	assert.Error(t, err)
}

func TestSearchEvents_ByOrg(t *testing.T) {
	store := setupTestDB(t)
	seedTestData(t, store)
	org := "testorg"
	q := &data.EventSearchCriteria{Org: &org, PageSize: 10, Page: 1}
	results, err := store.SearchEvents(q)
	require.NoError(t, err)
	assert.Len(t, results, 3)
}

func TestSearchEvents_ByUsername(t *testing.T) {
	store := setupTestDB(t)
	seedTestData(t, store)
	user := "dev1"
	q := &data.EventSearchCriteria{Username: &user, PageSize: 10, Page: 1}
	results, err := store.SearchEvents(q)
	require.NoError(t, err)
	assert.Len(t, results, 1)
}

func TestSearchEvents_ByEntity(t *testing.T) {
	store := setupTestDB(t)
	seedTestData(t, store)
	entity := "GOOGLE"
	q := &data.EventSearchCriteria{Entity: &entity, PageSize: 10, Page: 1}
	results, err := store.SearchEvents(q)
	require.NoError(t, err)
	assert.Len(t, results, 2)
}

func TestSearchEvents_ByMention(t *testing.T) {
	store := setupTestDB(t)
	seedTestData(t, store)
	mention := "dev1"
	q := &data.EventSearchCriteria{Mention: &mention, PageSize: 10, Page: 1}
	results, err := store.SearchEvents(q)
	require.NoError(t, err)
	assert.Len(t, results, 1)
}

func TestSearchEvents_ByLabel(t *testing.T) {
	store := setupTestDB(t)
	seedTestData(t, store)
	label := "bug"
	q := &data.EventSearchCriteria{Label: &label, PageSize: 10, Page: 1}
	results, err := store.SearchEvents(q)
	require.NoError(t, err)
	assert.Len(t, results, 1)
}

func TestSearchEvents_Pagination(t *testing.T) {
	store := setupTestDB(t)
	seedTestData(t, store)
	q := &data.EventSearchCriteria{PageSize: 2, Page: 1}
	results, err := store.SearchEvents(q)
	require.NoError(t, err)
	assert.Len(t, results, 2)

	q.Page = 2
	results, err = store.SearchEvents(q)
	require.NoError(t, err)
	assert.Len(t, results, 1)
}

func TestGetEventTypeSeries_NilDB(t *testing.T) {
	s := &Store{db: nil}
	_, err := s.GetEventTypeSeries(nil, nil, nil, 6)
	assert.Error(t, err)
}

func TestGetEventTypeSeries(t *testing.T) {
	store := setupTestDB(t)
	seedTestData(t, store)
	org := "testorg"
	repo := "testrepo"
	series, err := store.GetEventTypeSeries(&org, &repo, nil, 24)
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
