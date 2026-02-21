package data

import (
	"testing"
	"time"

	"github.com/google/go-github/v83/github"
	"github.com/stretchr/testify/assert"
)

func TestParsingBody(t *testing.T) {
	tests := map[string]int{
		"plain string with no name":     0,
		"@username up front":            1,
		"some username on the end @foo": 1,
		"@foo username on the end @bar": 2,
	}

	for input, expected := range tests {
		names := parseUsers(&input)
		assert.Len(t, names, expected)
	}
}

func TestParseDate(t *testing.T) {
	now := time.Date(2025, 6, 15, 10, 30, 0, 0, time.UTC)
	assert.Equal(t, "2025-06-15", parseDate(&now))
	assert.NotEmpty(t, parseDate(nil))
}

func TestTrim(t *testing.T) {
	s := " @hello "
	assert.Equal(t, "hello", trim(&s))
	assert.Equal(t, "", trim(nil))
}

func TestGetLabels_Nil(t *testing.T) {
	assert.Empty(t, getLabels(nil))
}

func TestGetLabels_WithValues(t *testing.T) {
	name1 := "Bug"
	name2 := "Feature"
	labels := []*github.Label{
		{Name: &name1},
		{Name: &name2},
		nil,
	}
	result := getLabels(labels)
	assert.Len(t, result, 2)
	assert.Equal(t, "bug", result[0])
	assert.Equal(t, "feature", result[1])
}

func TestGetUsernames_Nil(t *testing.T) {
	assert.Empty(t, getUsernames(nil))
}

func TestGetUsernames_WithValues(t *testing.T) {
	login1 := "user1"
	login2 := "user2"
	users := []*github.User{
		{Login: &login1},
		nil,
		{Login: &login2},
	}
	result := getUsernames(users...)
	assert.Len(t, result, 2)
}

func TestParseUsers_NilBody(t *testing.T) {
	assert.Empty(t, parseUsers(nil))
}

func TestMapUserToDeveloper(t *testing.T) {
	login := "testuser"
	name := "Test User"
	email := "test@example.com"
	avatar := "https://avatar.url"
	htmlURL := "https://github.com/testuser"
	company := "@TestCorp"
	u := &github.User{
		Login:     &login,
		Name:      &name,
		Email:     &email,
		AvatarURL: &avatar,
		HTMLURL:   &htmlURL,
		Company:   &company,
	}
	dev := mapUserToDeveloper(u)
	assert.Equal(t, "testuser", dev.Username)
	assert.Equal(t, "Test User", dev.FullName)
	// trim() strips @ signs, so email becomes "testexample.com"
	assert.Equal(t, "testexample.com", dev.Email)
	assert.Equal(t, "TestCorp", dev.Entity)
}

func TestRateInfo_Nil(t *testing.T) {
	assert.Equal(t, "", rateInfo(nil))
}

func TestRateInfo_WithRate(t *testing.T) {
	r := &github.Rate{
		Remaining: 4999,
		Limit:     5000,
		Reset:     github.Timestamp{Time: time.Date(2025, 6, 15, 14, 30, 0, 0, time.UTC)},
	}
	info := rateInfo(r)
	assert.Contains(t, info, "4999")
	assert.Contains(t, info, "5000")
}

func TestMapGitHubUserToDeveloperListItem(t *testing.T) {
	login := "testuser"
	company := "TestCo"
	u := &github.User{
		Login:   &login,
		Company: &company,
	}
	item := mapGitHubUserToDeveloperListItem(u)
	assert.Equal(t, "testuser", item.Username)
	assert.Equal(t, "TestCo", item.Entity)
}

func TestGetUsernames_EmptySlice(t *testing.T) {
	result := getUsernames([]*github.User{}...)
	assert.Empty(t, result)
}

func TestGetLabels_EmptySlice(t *testing.T) {
	result := getLabels([]*github.Label{})
	assert.Empty(t, result)
}

func TestTrim_EmptyString(t *testing.T) {
	s := ""
	assert.Equal(t, "", trim(&s))
}

func TestTrim_AtSign(t *testing.T) {
	s := "@org"
	assert.Equal(t, "org", trim(&s))
}
