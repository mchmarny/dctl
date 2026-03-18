package cli

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/mchmarny/devpulse/pkg/data"
	"github.com/urfave/cli/v3"
)

var (
	countFlag = &cli.IntFlag{
		Name:    "count",
		Usage:   "Number of lowest-reputation contributors to deep-score via GitHub API",
		Value:   5,
		Sources: cli.EnvVars("DEVPULSE_COUNT"),
	}

	scoreCmd = &cli.Command{
		Name:            "score",
		HideHelpCommand: true,
		Usage:           "Deep-score contributor reputation via GitHub API",
		UsageText: `devpulse score --org <ORG> [--repo <REPO>] [--count <N>]

Examples:
  devpulse score --org myorg                          # deep-score 5 lowest in org
  devpulse score --org myorg --repo repo1             # deep-score 5 lowest in repo
  devpulse score --org myorg --count 20               # deep-score 20 lowest
  devpulse score --org myorg --repo repo1 --count 10  # scoped deep scoring`,
		Action: cmdScore,
		Flags: []cli.Flag{
			orgNameFlag,
			repoNameFlag,
			countFlag,
			formatFlag,
			debugFlag,
			logJSONFlag,
		},
	}
)

// ScoreResult holds the output of the score command.
type ScoreResult struct {
	Org      string                     `json:"org" yaml:"org"`
	Repo     string                     `json:"repo,omitempty" yaml:"repo,omitempty"`
	Result   *data.DeepReputationResult `json:"result" yaml:"result"`
	Duration string                     `json:"duration" yaml:"duration"`
}

func cmdScore(ctx context.Context, cmd *cli.Command) error {
	start := time.Now()
	applyFlags(cmd)

	org := cmd.String(orgNameFlag.Name)
	if org == "" {
		return cli.ShowSubcommandHelp(cmd)
	}

	token, err := requireGitHubToken()
	if err != nil {
		return err
	}

	cfg := getConfig(cmd)

	var repo string
	if repos := cmd.StringSlice(repoNameFlag.Name); len(repos) > 0 {
		repo = repos[0]
	}

	orgPtr := &org
	var repoPtr *string
	if repo != "" {
		repoPtr = &repo
	}

	count := cmd.Int(countFlag.Name)

	slog.Info("deep scoring", "org", org, "repo", repo, "count", count)
	result, err := cfg.Store.ImportDeepReputation(ctx, func() string { return token }, count, 0, orgPtr, repoPtr)
	if err != nil {
		return fmt.Errorf("failed to compute deep reputation scores: %w", err)
	}

	res := &ScoreResult{
		Org:      org,
		Repo:     repo,
		Result:   result,
		Duration: time.Since(start).String(),
	}

	if err := encode(res); err != nil {
		return fmt.Errorf("encoding result: %w", err)
	}

	return nil
}
