package cli

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFlattenTargets(t *testing.T) {
	sources := []syncSource{
		{Org: "NVIDIA", Repos: []string{"aicr", "skyhook"}},
		{Org: "mchmarny", Repos: []string{"devpulse"}},
	}

	targets := flattenTargets(sources)

	require.Len(t, targets, 3)
	assert.Equal(t, syncTarget{Org: "NVIDIA", Repo: "aicr"}, targets[0])
	assert.Equal(t, syncTarget{Org: "NVIDIA", Repo: "skyhook"}, targets[1])
	assert.Equal(t, syncTarget{Org: "mchmarny", Repo: "devpulse"}, targets[2])
}

func TestFlattenTargetsEmpty(t *testing.T) {
	targets := flattenTargets(nil)
	assert.Empty(t, targets)

	targets = flattenTargets([]syncSource{{Org: "org", Repos: nil}})
	assert.Empty(t, targets)
}

func TestPickTarget(t *testing.T) {
	targets := []syncTarget{
		{Org: "NVIDIA", Repo: "aicr"},
		{Org: "NVIDIA", Repo: "skyhook"},
		{Org: "mchmarny", Repo: "devpulse"},
	}

	tests := []struct {
		name     string
		hour     int
		expected syncTarget
	}{
		{"hour 0 picks index 0", 0, targets[0]},
		{"hour 1 picks index 1", 1, targets[1]},
		{"hour 2 picks index 2", 2, targets[2]},
		{"hour 3 wraps to index 0", 3, targets[0]},
		{"hour 23 wraps", 23, targets[23%3]},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			now := time.Date(2026, 3, 17, tt.hour, 0, 0, 0, time.UTC)
			got := pickTarget(targets, now)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestLoadSyncConfigFromFile(t *testing.T) {
	yaml := `sources:
  - org: NVIDIA
    repos:
      - aicr
      - skyhook
  - org: mchmarny
    repos:
      - devpulse
score:
  count: 999
`
	f := t.TempDir() + "/sync.yaml"
	require.NoError(t, writeTestFile(f, yaml))

	sc, err := loadSyncConfig(t.Context(), f)
	require.NoError(t, err)
	require.Len(t, sc.Sources, 2)
	assert.Equal(t, "NVIDIA", sc.Sources[0].Org)
	assert.Equal(t, []string{"aicr", "skyhook"}, sc.Sources[0].Repos)
	assert.Equal(t, "mchmarny", sc.Sources[1].Org)
	assert.Equal(t, []string{"devpulse"}, sc.Sources[1].Repos)
	assert.Equal(t, 999, sc.Score.Count)
}

func TestLoadSyncConfigFileNotFound(t *testing.T) {
	_, err := loadSyncConfig(t.Context(), "/nonexistent/sync.yaml")
	require.Error(t, err)
}

func writeTestFile(path, content string) error {
	return os.WriteFile(path, []byte(content), 0o600)
}
