package cli

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/mchmarny/devpulse/pkg/data"
	"github.com/mchmarny/devpulse/pkg/data/ghutil"
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

	syncOrgFlag = &urfave.StringFlag{
		Name:  "org",
		Usage: "Override target org (requires --repo)",
	}

	syncRepoFlag = &urfave.StringFlag{
		Name:  "repo",
		Usage: "Override target repo (requires --org)",
	}

	syncStaleFlag = &urfave.StringFlag{
		Name:    "stale",
		Usage:   "Duration before a reputation score is considered stale (e.g. 72h, 3d, 1w)",
		Value:   "3d",
		Sources: urfave.EnvVars("DEVPULSE_STALE"),
	}

	syncCmd = &urfave.Command{
		Name:            "sync",
		HideHelpCommand: true,
		Usage:           "Import and score one repo from a config file (round-robin by hour)",
		UsageText: `devpulse sync --config <path-or-url> [--org <org> --repo <repo>]
  devpulse sync --org <org> --repo <repo>

Examples:
  devpulse sync --config sync.yaml
  devpulse sync --config https://raw.githubusercontent.com/org/repo/main/sync.yaml
  devpulse sync --config sync.yaml --org NVIDIA --repo DCGM
  devpulse sync --org NVIDIA --repo DCGM`,
		Action: cmdSync,
		Flags: []urfave.Flag{
			syncConfigFlag,
			syncOrgFlag,
			syncRepoFlag,
			syncStaleFlag,
			debugFlag,
			logJSONFlag,
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
	orgOverride := cmd.String(syncOrgFlag.Name)
	repoOverride := cmd.String(syncRepoFlag.Name)

	if (orgOverride == "") != (repoOverride == "") {
		return fmt.Errorf("--org and --repo must be specified together")
	}

	var (
		target syncTarget
		sc     *syncConfig
	)

	if orgOverride != "" {
		target = syncTarget{Org: orgOverride, Repo: repoOverride}
		if configPath != "" {
			var err error
			sc, err = loadSyncConfig(ctx, configPath)
			if err != nil {
				return fmt.Errorf("loading sync config: %w", err)
			}
		}
		slog.Info("sync target override", "org", target.Org, "repo", target.Repo)
	} else {
		if configPath == "" {
			return fmt.Errorf("--config is required when --org/--repo are not set")
		}
		var err error
		sc, err = loadSyncConfig(ctx, configPath)
		if err != nil {
			return fmt.Errorf("loading sync config: %w", err)
		}
		targets := flattenTargets(sc.Sources)
		if len(targets) == 0 {
			return fmt.Errorf("no repos found in sync config")
		}
		target = pickTarget(targets, time.Now())
		slog.Info("sync target selected",
			"org", target.Org,
			"repo", target.Repo,
			"index", time.Now().UTC().Hour()%len(targets),
			"total", len(targets),
		)
	}

	tokenStr, err := requireGitHubToken()
	if err != nil {
		return err
	}

	pool := ghutil.NewTokenPool(tokenStr)
	slog.Info("token pool initialized", "tokens", pool.Size())

	cfg := getConfig(cmd)

	var (
		errors     int
		eventCount int
		devCount   int
		scored     int
	)

	// Import
	phaseStart := time.Now()
	slog.Info("importing events", "org", target.Org, "repo", target.Repo)
	_, summary, importErr := cfg.Store.ImportEvents(ctx, pool.Token(), target.Org, target.Repo, data.EventAgeMonthsDefault)
	importSec := time.Since(phaseStart).Seconds()
	if importErr != nil {
		errors++
		slog.Error("failed to import events", "org", target.Org, "repo", target.Repo, "error", importErr)
	} else if summary != nil {
		eventCount = summary.Events
		devCount = summary.Developers
		slog.Info("import complete", "events", eventCount, "developers", devCount, "duration_sec", importSec)
	}

	// Affiliations
	phaseStart = time.Now()
	slog.Info("updating affiliations")
	if _, affErr := importAffiliations(ctx, cfg.Store); affErr != nil {
		errors++
		slog.Error("affiliations failed", "error", affErr)
	}
	affiliationsSec := time.Since(phaseStart).Seconds()

	// Substitutions
	phaseStart = time.Now()
	slog.Info("applying substitutions")
	if _, subErr := cfg.Store.ApplySubstitutions(); subErr != nil {
		errors++
		slog.Error("substitutions failed", "error", subErr)
	}
	substitutionsSec := time.Since(phaseStart).Seconds()

	// Extras (metadata, releases, etc.)
	phaseStart = time.Now()
	importRepoExtras(ctx, cfg.Store, pool.Token(), target.Org, []string{target.Repo})
	extrasSec := time.Since(phaseStart).Seconds()

	// Reputation (local, no API calls)
	phaseStart = time.Now()
	org := target.Org
	slog.Info("computing reputation")
	if _, repErr := cfg.Store.ImportReputation(&org, nil); repErr != nil {
		errors++
		slog.Error("reputation failed", "error", repErr)
	}
	reputationSec := time.Since(phaseStart).Seconds()

	// Score
	phaseStart = time.Now()
	count := 999
	if sc != nil && sc.Score.Count > 0 {
		count = sc.Score.Count
	}
	repo := target.Repo
	slog.Info("deep scoring", "org", target.Org, "repo", target.Repo, "count", count)
	staleHours, parseErr := parseDurationHours(cmd.String(syncStaleFlag.Name))
	if parseErr != nil {
		return fmt.Errorf("invalid --stale value: %w", parseErr)
	}
	deepResult, scoreErr := cfg.Store.ImportDeepReputation(ctx, pool.Token, count, staleHours, &org, &repo)
	if scoreErr != nil {
		errors++
		slog.Error("deep scoring failed", "error", scoreErr)
	} else if deepResult != nil {
		scored = deepResult.Scored
		errors += deepResult.Errors
	}
	scoringSec := time.Since(phaseStart).Seconds()

	totalSec := time.Since(start).Seconds()

	slog.Info("sync_summary",
		"org", target.Org,
		"repo", target.Repo,
		"events", eventCount,
		"developers", devCount,
		"scored", scored,
		"errors", errors,
		"total_sec", totalSec,
		"import_sec", importSec,
		"affiliations_sec", affiliationsSec,
		"substitutions_sec", substitutionsSec,
		"extras_sec", extrasSec,
		"reputation_sec", reputationSec,
		"scoring_sec", scoringSec,
	)

	return nil
}

func loadSyncConfig(ctx context.Context, path string) (*syncConfig, error) {
	var r io.ReadCloser

	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		parsed, err := url.Parse(path)
		if err != nil {
			return nil, fmt.Errorf("parsing config URL: %w", err)
		}
		client, err := pnet.GetHTTPClient()
		if err != nil {
			return nil, fmt.Errorf("creating HTTP client: %w", err)
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, parsed.String(), nil)
		if err != nil {
			return nil, fmt.Errorf("creating request: %w", err)
		}
		resp, err := client.Do(req) //nolint:gosec,nolintlint // G704: URL from trusted --config flag
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

// parseDurationHours parses a duration string into whole hours.
// Supports Go duration syntax (e.g. "72h") plus shorthand "d" (days) and "w" (weeks).
func parseDurationHours(s string) (int, error) {
	if s == "" {
		return 0, nil
	}

	// Handle shorthand suffixes not supported by time.ParseDuration.
	switch {
	case strings.HasSuffix(s, "d"):
		s = strings.TrimSuffix(s, "d")
		var days int
		if _, err := fmt.Sscanf(s, "%d", &days); err != nil {
			return 0, fmt.Errorf("invalid day duration %q: %w", s+"d", err)
		}
		return days * 24, nil
	case strings.HasSuffix(s, "w"):
		s = strings.TrimSuffix(s, "w")
		var weeks int
		if _, err := fmt.Sscanf(s, "%d", &weeks); err != nil {
			return 0, fmt.Errorf("invalid week duration %q: %w", s+"w", err)
		}
		return weeks * 7 * 24, nil
	}

	d, err := time.ParseDuration(s)
	if err != nil {
		return 0, err
	}
	return int(d.Hours()), nil
}
