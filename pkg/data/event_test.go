package data

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestUnique(t *testing.T) {
	tests := []struct {
		input    []string
		expected int
	}{
		{[]string{"@a", "@b", "@a"}, 2},
		{[]string{" x ", "@y"}, 2},
		{[]string{}, 0},
		{nil, 0},
	}
	for _, tc := range tests {
		result := unique(tc.input)
		assert.Len(t, result, tc.expected)
	}
}

func TestIsEventBatchValidAge(t *testing.T) {
	minTime := time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)
	imp := &EventImporter{minEventTime: minTime}

	recent := time.Date(2025, 7, 1, 0, 0, 0, 0, time.UTC)
	old := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)

	assert.True(t, imp.isEventBatchValidAge(&recent, &recent))
	assert.True(t, imp.isEventBatchValidAge(&recent, &old))
	assert.False(t, imp.isEventBatchValidAge(&old, &old))
	assert.False(t, imp.isEventBatchValidAge(nil, nil))
}

func TestQualifyTypeKey(t *testing.T) {
	imp := &EventImporter{owner: "org", repo: "repo"}
	assert.Equal(t, "org/repo/pr", imp.qualifyTypeKey("pr"))
}

func TestTimestampToTime_Nil(t *testing.T) {
	assert.Nil(t, timestampToTime(nil))
}

func TestGetStrPtr(t *testing.T) {
	assert.Nil(t, getStrPtr(""))
	ptr := getStrPtr("hello")
	assert.NotNil(t, ptr)
	assert.Equal(t, "hello", *ptr)
}
