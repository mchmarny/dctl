package data

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtractAffiliations(t *testing.T) {
	content := `jdoe: jdoe!gmail.com, jdoe!users.noreply.github.com
Google from 2020-01-01 until 2022-12-31
Microsoft from 2023-01-01
asmith: asmith!corp.com
Independent`

	dir := t.TempDir()
	path := filepath.Join(dir, "test_affiliations.txt")
	require.NoError(t, os.WriteFile(path, []byte(content), 0644))

	devs := make(map[string]*CNCFDeveloper)
	err := extractAffiliations(path, devs)
	require.NoError(t, err)
	assert.Contains(t, devs, "jdoe")

	jdoe := devs["jdoe"]
	assert.Len(t, jdoe.Affiliations, 2)
	assert.Equal(t, "Google", jdoe.Affiliations[0].Entity)
}

func TestExtractAffiliations_EmptyPath(t *testing.T) {
	devs := make(map[string]*CNCFDeveloper)
	err := extractAffiliations("", devs)
	assert.Error(t, err)
}

func TestCNCFDeveloper_GetBestIdentity(t *testing.T) {
	dev := &CNCFDeveloper{Identities: []string{"first@test.com", "second@test.com"}}
	assert.Equal(t, "first@test.com", dev.GetBestIdentity())

	empty := &CNCFDeveloper{}
	assert.Equal(t, "", empty.GetBestIdentity())
}

func TestCNCFDeveloper_GetLatestAffiliation(t *testing.T) {
	dev := &CNCFDeveloper{
		Affiliations: []*CNCFAffiliation{
			{Entity: "OldCorp", From: "2020-01-01"},
			{Entity: "NewCorp", From: "2023-06-01"},
		},
	}
	assert.Equal(t, "NewCorp", dev.GetLatestAffiliation())

	empty := &CNCFDeveloper{}
	assert.Equal(t, "", empty.GetLatestAffiliation())
}

func TestExtractAffiliations_NonExistentFile(t *testing.T) {
	devs := make(map[string]*CNCFDeveloper)
	err := extractAffiliations("/nonexistent/path/file.txt", devs)
	assert.Error(t, err)
}

func TestCNCFDeveloper_GetLatestAffiliation_Single(t *testing.T) {
	dev := &CNCFDeveloper{
		Affiliations: []*CNCFAffiliation{
			{Entity: "OnlyCorp", From: "2023-01-01"},
		},
	}
	assert.Equal(t, "OnlyCorp", dev.GetLatestAffiliation())
}
