package cli

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	urfave "github.com/urfave/cli/v3"
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

func TestCmdSyncOrgRepoValidation(t *testing.T) {
	yaml := `sources:
  - org: NVIDIA
    repos:
      - aicr
score:
  count: 999
`
	f := t.TempDir() + "/sync.yaml"
	require.NoError(t, writeTestFile(f, yaml))

	tests := []struct {
		name    string
		config  string
		org     string
		repo    string
		wantErr string
	}{
		{"org without repo", f, "NVIDIA", "", "--org and --repo must be specified together"},
		{"repo without org", f, "", "aicr", "--org and --repo must be specified together"},
		{"no config no override", "", "", "", "--config is required"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := &urfave.Command{
				Commands: []*urfave.Command{
					{
						Name:   "sync",
						Action: cmdSync,
						Flags: []urfave.Flag{
							dbFilePathFlag,
							syncConfigFlag,
							syncOrgFlag,
							syncRepoFlag,
							debugFlag,
							logJSONFlag,
						},
					},
				},
			}

			args := []string{"test", "sync"}
			if tt.config != "" {
				args = append(args, "--config", tt.config)
			}
			if tt.org != "" {
				args = append(args, "--org", tt.org)
			}
			if tt.repo != "" {
				args = append(args, "--repo", tt.repo)
			}

			err := app.Run(t.Context(), args)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestParseDurationHours(t *testing.T) {
	tests := []struct {
		input   string
		want    int
		wantErr bool
	}{
		{"72h", 72, false},
		{"3d", 72, false},
		{"1w", 168, false},
		{"2w", 336, false},
		{"1d", 24, false},
		{"30m", 0, false},
		{"", 0, false},
		{"bogus", 0, true},
		{"xd", 0, true},
		{"xw", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := parseDurationHours(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func writeTestFile(path, content string) error {
	return os.WriteFile(path, []byte(content), 0o600)
}
