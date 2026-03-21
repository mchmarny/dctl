package cli

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	urfave "github.com/urfave/cli/v3"
)

func TestResolveTargets(t *testing.T) {
	repos := []syncRepo{
		{Name: "aicr", Org: "NVIDIA"},
		{Name: "skyhook", Org: "NVIDIA", Reputation: &syncReputation{ScoreCount: 200}},
		{Name: "devpulse", Org: "mchmarny", Insight: &syncInsight{PeriodMonths: 6, StaleAfter: "14d"}},
	}

	targets, err := resolveTargets(repos)
	require.NoError(t, err)
	require.Len(t, targets, 3)

	assert.Equal(t, "NVIDIA", targets[0].Org)
	assert.Equal(t, "aicr", targets[0].Repo)
	assert.Equal(t, defaultScoreCount, targets[0].ScoreCount)
	assert.Equal(t, 72, targets[0].ReputationStale) // 3d default
	assert.Equal(t, defaultInsightPeriod, targets[0].InsightPeriod)
	assert.Equal(t, 72, targets[0].InsightStale) // 3d default

	assert.Equal(t, 200, targets[1].ScoreCount)

	assert.Equal(t, 6, targets[2].InsightPeriod)
	assert.Equal(t, 336, targets[2].InsightStale) // 14d
}

func TestResolveTargetsEmpty(t *testing.T) {
	targets, err := resolveTargets(nil)
	require.NoError(t, err)
	assert.Empty(t, targets)
}

func TestResolveTargetDefaults(t *testing.T) {
	target, err := resolveTarget("NVIDIA", "aicr", nil, nil)
	require.NoError(t, err)
	assert.Equal(t, defaultScoreCount, target.ScoreCount)
	assert.Equal(t, 72, target.ReputationStale)
	assert.Equal(t, 72, target.InsightStale)
	assert.Equal(t, defaultInsightPeriod, target.InsightPeriod)
}

func TestResolveTargetOverrides(t *testing.T) {
	target, err := resolveTarget("NVIDIA", "aicr",
		&syncReputation{ScoreCount: 50, StaleAfter: "1d"},
		&syncInsight{PeriodMonths: 6, StaleAfter: "2w"},
	)
	require.NoError(t, err)
	assert.Equal(t, 50, target.ScoreCount)
	assert.Equal(t, 24, target.ReputationStale)
	assert.Equal(t, 6, target.InsightPeriod)
	assert.Equal(t, 336, target.InsightStale)
}

func TestResolveTargetInvalidStale(t *testing.T) {
	_, err := resolveTarget("NVIDIA", "aicr",
		&syncReputation{StaleAfter: "bogus"}, nil)
	require.Error(t, err)

	_, err = resolveTarget("NVIDIA", "aicr",
		nil, &syncInsight{StaleAfter: "bogus"})
	require.Error(t, err)
}

func TestFindRepo(t *testing.T) {
	sc := &syncConfig{
		Repos: []syncRepo{
			{Name: "aicr", Org: "NVIDIA", Reputation: &syncReputation{ScoreCount: 100}},
			{Name: "skyhook", Org: "NVIDIA"},
		},
	}

	found := findRepo(sc, "NVIDIA", "aicr")
	require.NotNil(t, found)
	assert.Equal(t, 100, found.Reputation.ScoreCount)

	found = findRepo(sc, "nvidia", "AICR") // case insensitive
	require.NotNil(t, found)

	found = findRepo(sc, "NVIDIA", "nonexistent")
	assert.Nil(t, found)
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
	yaml := `repos:
  - name: aicr
    org: NVIDIA
    reputation:
      scoreCount: 100
      staleAfter: "48h"
  - name: skyhook
    org: NVIDIA
  - name: devpulse
    org: mchmarny
    insight:
      periodMonths: 6
`
	f := t.TempDir() + "/sync.yaml"
	require.NoError(t, writeTestFile(f, yaml))

	sc, err := loadSyncConfig(t.Context(), f)
	require.NoError(t, err)
	require.Len(t, sc.Repos, 3)
	assert.Equal(t, "NVIDIA", sc.Repos[0].Org)
	assert.Equal(t, "aicr", sc.Repos[0].Name)
	require.NotNil(t, sc.Repos[0].Reputation)
	assert.Equal(t, 100, sc.Repos[0].Reputation.ScoreCount)
	assert.Equal(t, "48h", sc.Repos[0].Reputation.StaleAfter)
	assert.Nil(t, sc.Repos[1].Reputation)
	assert.Equal(t, "mchmarny", sc.Repos[2].Org)
	require.NotNil(t, sc.Repos[2].Insight)
	assert.Equal(t, 6, sc.Repos[2].Insight.PeriodMonths)
}

func TestLoadSyncConfigFileNotFound(t *testing.T) {
	_, err := loadSyncConfig(t.Context(), "/nonexistent/sync.yaml")
	require.Error(t, err)
}

func TestCmdSyncOrgRepoValidation(t *testing.T) {
	yaml := `repos:
  - name: aicr
    org: NVIDIA
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
