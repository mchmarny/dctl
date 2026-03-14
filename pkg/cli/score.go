package cli

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/mchmarny/devpulse/pkg/data"
	"github.com/urfave/cli/v2"
)

var (
	countFlag = &cli.IntFlag{
		Name:  "count",
		Usage: "Number of lowest-reputation contributors to deep-score via GitHub API",
		Value: 5,
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

func cmdScore(c *cli.Context) error {
	start := time.Now()
	applyFlags(c)

	org := c.String(orgNameFlag.Name)
	if org == "" {
		return cli.ShowSubcommandHelp(c)
	}

	token, err := getGitHubToken()
	if err != nil {
		return fmt.Errorf("failed to get GitHub token: %w", err)
	}
	if token == "" {
		return fmt.Errorf("GitHub token required for deep scoring")
	}

	cfg := getConfig(c)

	var repo string
	if repos := c.StringSlice(repoNameFlag.Name); len(repos) > 0 {
		repo = repos[0]
	}

	orgPtr := &org
	var repoPtr *string
	if repo != "" {
		repoPtr = &repo
	}

	count := c.Int(countFlag.Name)

	slog.Info("deep scoring", "org", org, "repo", repo, "count", count)
	result, err := data.ImportDeepReputation(cfg.DB, token, count, orgPtr, repoPtr)
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
