package cli

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/mchmarny/devpulse/pkg/data"
	pnet "github.com/mchmarny/devpulse/pkg/net"
	urfave "github.com/urfave/cli/v3"
	"gopkg.in/yaml.v3"
)

var (
	syncConfigFlag = &urfave.StringFlag{
		Name:    "config",
		Usage:   "Path or URL to sync config file",
		Sources: urfave.EnvVars("DEVPULSE_SYNC_CONFIG"),
	}

	syncCmd = &urfave.Command{
		Name:            "sync",
		HideHelpCommand: true,
		Usage:           "Import and score one repo from a config file (round-robin by hour)",
		UsageText: `devpulse sync --config <path-or-url>

Examples:
  devpulse sync --config sync.yaml
  devpulse sync --config https://raw.githubusercontent.com/org/repo/main/sync.yaml`,
		Action: cmdSync,
		Flags: []urfave.Flag{
			syncConfigFlag,
			debugFlag,
		},
	}
)

// syncConfig represents the sync configuration file.
type syncConfig struct {
	Sources []syncSource `yaml:"sources"`
	Score   syncScore    `yaml:"score"`
}

type syncSource struct {
	Org   string   `yaml:"org"`
	Repos []string `yaml:"repos"`
}

type syncScore struct {
	Count int `yaml:"count"`
}

// syncTarget is a flattened org/repo pair.
type syncTarget struct {
	Org  string
	Repo string
}

func cmdSync(ctx context.Context, cmd *urfave.Command) error {
	start := time.Now()
	applyFlags(cmd)

	configPath := cmd.String(syncConfigFlag.Name)
	if configPath == "" {
		return fmt.Errorf("--config is required")
	}

	sc, err := loadSyncConfig(ctx, configPath)
	if err != nil {
		return fmt.Errorf("loading sync config: %w", err)
	}

	targets := flattenTargets(sc.Sources)
	if len(targets) == 0 {
		return fmt.Errorf("no repos found in sync config")
	}

	target := pickTarget(targets, time.Now())
	slog.Info("sync target selected",
		"org", target.Org,
		"repo", target.Repo,
		"index", time.Now().UTC().Hour()%len(targets),
		"total", len(targets),
	)

	token, err := getGitHubToken()
	if err != nil {
		return fmt.Errorf("no GitHub token found, run 'devpulse auth' first: %w", err)
	}
	if token == "" {
		return fmt.Errorf("no GitHub token found, run 'devpulse auth' or set GITHUB_TOKEN")
	}

	cfg := getConfig(cmd)

	// Import
	slog.Info("importing events", "org", target.Org, "repo", target.Repo)
	events, summary, importErr := cfg.Store.ImportEvents(ctx, token, target.Org, target.Repo, data.EventAgeMonthsDefault)
	if importErr != nil {
		slog.Error("failed to import events", "org", target.Org, "repo", target.Repo, "error", importErr)
	} else {
		slog.Info("import complete", "events", events, "summary", summary)
	}

	// Affiliations
	slog.Info("updating affiliations")
	if _, affErr := importAffiliations(ctx, cfg.Store); affErr != nil {
		slog.Error("affiliations failed", "error", affErr)
	}

	// Substitutions
	slog.Info("applying substitutions")
	if _, subErr := cfg.Store.ApplySubstitutions(); subErr != nil {
		slog.Error("substitutions failed", "error", subErr)
	}

	// Extras (metadata, releases, etc.)
	importRepoExtras(ctx, cfg.Store, token, target.Org, []string{target.Repo})

	// Reputation (local, no API calls)
	org := target.Org
	slog.Info("computing reputation")
	if _, repErr := cfg.Store.ImportReputation(&org, nil); repErr != nil {
		slog.Error("reputation failed", "error", repErr)
	}

	// Score
	count := sc.Score.Count
	if count <= 0 {
		count = 999
	}
	repo := target.Repo
	slog.Info("deep scoring", "org", target.Org, "repo", target.Repo, "count", count)
	if _, scoreErr := cfg.Store.ImportDeepReputation(ctx, token, count, &org, &repo); scoreErr != nil {
		slog.Error("deep scoring failed", "error", scoreErr)
	}

	slog.Info("sync complete", "org", target.Org, "repo", target.Repo, "duration", time.Since(start).String())

	return nil
}

func loadSyncConfig(ctx context.Context, path string) (*syncConfig, error) {
	var r io.ReadCloser

	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		client, err := pnet.GetHTTPClient()
		if err != nil {
			return nil, fmt.Errorf("creating HTTP client: %w", err)
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, path, nil)
		if err != nil {
			return nil, fmt.Errorf("creating request: %w", err)
		}
		resp, err := client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("fetching config from %s: %w", path, err)
		}
		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			return nil, fmt.Errorf("fetching config from %s: status %d", path, resp.StatusCode)
		}
		r = resp.Body
	} else {
		f, err := os.Open(path)
		if err != nil {
			return nil, fmt.Errorf("opening config file %s: %w", path, err)
		}
		r = f
	}
	defer r.Close()

	var sc syncConfig
	if err := yaml.NewDecoder(r).Decode(&sc); err != nil {
		return nil, fmt.Errorf("parsing sync config: %w", err)
	}

	return &sc, nil
}

func flattenTargets(sources []syncSource) []syncTarget {
	var targets []syncTarget
	for _, s := range sources {
		for _, r := range s.Repos {
			targets = append(targets, syncTarget{Org: s.Org, Repo: r})
		}
	}
	return targets
}

func pickTarget(targets []syncTarget, now time.Time) syncTarget {
	idx := now.UTC().Hour() % len(targets)
	return targets[idx]
}
