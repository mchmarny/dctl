package sqlite

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mchmarny/devpulse/pkg/data"
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

	devs := make(map[string]*data.CNCFDeveloper)
	err := extractAffiliations(path, devs)
	require.NoError(t, err)
	assert.Contains(t, devs, "jdoe")

	jdoe := devs["jdoe"]
	assert.Len(t, jdoe.Affiliations, 2)
	assert.Equal(t, "Google", jdoe.Affiliations[0].Entity)
}

func TestExtractAffiliations_EmptyPath(t *testing.T) {
	devs := make(map[string]*data.CNCFDeveloper)
	err := extractAffiliations("", devs)
	assert.Error(t, err)
}

func TestExtractAffiliations_NonExistentFile(t *testing.T) {
	devs := make(map[string]*data.CNCFDeveloper)
	err := extractAffiliations("/nonexistent/path/file.txt", devs)
	assert.Error(t, err)
}
