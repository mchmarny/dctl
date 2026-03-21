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
			dbFilePathFlag,
			syncConfigFlag,
			syncOrgFlag,
			syncRepoFlag,
			debugFlag,
			logJSONFlag,
		},
	}
)

const (
	defaultScoreCount      = 50
	defaultReputationStale = "3d"
	defaultInsightStale    = "3d"
	defaultInsightPeriod   = 3
)

// syncConfig represents the sync configuration file.
type syncConfig struct {
	Repos []syncRepo `yaml:"repos"`
}

type syncRepo struct {
	Name       string          `yaml:"name"`
	Org        string          `yaml:"org"`
	Reputation *syncReputation `yaml:"reputation,omitempty"`
	Insight    *syncInsight    `yaml:"insight,omitempty"`
}

type syncReputation struct {
	ScoreCount int    `yaml:"scoreCount,omitempty"`
	StaleAfter string `yaml:"staleAfter,omitempty"`
}

type syncInsight struct {
	PeriodMonths int    `yaml:"periodMonths,omitempty"`
	StaleAfter   string `yaml:"staleAfter,omitempty"`
}

// syncTarget is a resolved org/repo with numeric parameters ready for use.
type syncTarget struct {
	Org             string
	Repo            string
	ScoreCount      int
	ReputationStale int // hours
	InsightStale    int // hours
	InsightPeriod   int // months
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

	target, err := selectTarget(ctx, configPath, orgOverride, repoOverride)
	if err != nil {
		return err
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
	if _, affErr := importAffiliations(ctx, cfg.Store, pool.Token()); affErr != nil {
		errors++
		slog.Error("affiliations failed", "error", affErr)
	}
	affiliationsSec := time.Since(phaseStart).Seconds()

	// Substitutions
	phaseStart = time.Now()
	if _, subErr := cfg.Store.ApplySubstitutions(); subErr != nil {
		errors++
		slog.Error("substitutions failed", "error", subErr)
	}
	substitutionsSec := time.Since(phaseStart).Seconds()

	// Extras
	phaseStart = time.Now()
	importRepoExtras(ctx, cfg.Store, pool.Token(), target.Org, []string{target.Repo})
	extrasSec := time.Since(phaseStart).Seconds()

	// Reputation
	phaseStart = time.Now()
	org := target.Org
	if _, repErr := cfg.Store.ImportReputation(&org, nil); repErr != nil {
		errors++
		slog.Error("reputation failed", "error", repErr)
	}
	reputationSec := time.Since(phaseStart).Seconds()

	// Score
	phaseStart = time.Now()
	repo := target.Repo
	slog.Info("deep scoring", "org", target.Org, "repo", target.Repo, "count", target.ScoreCount)
	deepResult, scoreErr := cfg.Store.ImportDeepReputation(ctx, pool.Token, target.ScoreCount, target.ReputationStale, &org, &repo)
	if scoreErr != nil {
		errors++
		slog.Error("deep scoring failed", "error", scoreErr)
	} else if deepResult != nil {
		scored = deepResult.Scored
		errors += deepResult.Errors
	}
	scoringSec := time.Since(phaseStart).Seconds()

	// Insights
	phaseStart = time.Now()
	insightsGenerated, insightsErrors := runInsightsGeneration(ctx, cfg.Store, target)
	errors += insightsErrors
	insightsSec := time.Since(phaseStart).Seconds()

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
		"insights_sec", insightsSec,
		"insights_generated", insightsGenerated,
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

func resolveTargets(repos []syncRepo) ([]syncTarget, error) {
	targets := make([]syncTarget, 0, len(repos))
	for _, r := range repos {
		t, err := resolveTarget(r.Org, r.Name, r.Reputation, r.Insight)
		if err != nil {
			return nil, fmt.Errorf("repo %s/%s: %w", r.Org, r.Name, err)
		}
		targets = append(targets, t)
	}
	return targets, nil
}

func resolveTarget(org, repo string, rep *syncReputation, ins *syncInsight) (syncTarget, error) {
	t := syncTarget{
		Org:           org,
		Repo:          repo,
		ScoreCount:    defaultScoreCount,
		InsightPeriod: defaultInsightPeriod,
	}

	repStale := defaultReputationStale
	if rep != nil {
		if rep.ScoreCount > 0 {
			t.ScoreCount = rep.ScoreCount
		}
		if rep.StaleAfter != "" {
			repStale = rep.StaleAfter
		}
	}
	var err error
	t.ReputationStale, err = parseDurationHours(repStale)
	if err != nil {
		return syncTarget{}, fmt.Errorf("invalid reputation.staleAfter %q: %w", repStale, err)
	}

	insStale := defaultInsightStale
	if ins != nil {
		if ins.PeriodMonths > 0 {
			t.InsightPeriod = ins.PeriodMonths
		}
		if ins.StaleAfter != "" {
			insStale = ins.StaleAfter
		}
	}
	t.InsightStale, err = parseDurationHours(insStale)
	if err != nil {
		return syncTarget{}, fmt.Errorf("invalid insight.staleAfter %q: %w", insStale, err)
	}

	return t, nil
}

func selectTarget(ctx context.Context, configPath, org, repo string) (syncTarget, error) {
	if org != "" {
		if configPath != "" {
			sc, err := loadSyncConfig(ctx, configPath)
			if err != nil {
				return syncTarget{}, fmt.Errorf("loading sync config: %w", err)
			}
			if found := findRepo(sc, org, repo); found != nil {
				return resolveTarget(found.Org, found.Name, found.Reputation, found.Insight)
			}
		}
		t, err := resolveTarget(org, repo, nil, nil)
		if err != nil {
			return syncTarget{}, err
		}
		slog.Info("sync target override", "org", t.Org, "repo", t.Repo)
		return t, nil
	}

	if configPath == "" {
		return syncTarget{}, fmt.Errorf("--config is required when --org/--repo are not set")
	}
	sc, err := loadSyncConfig(ctx, configPath)
	if err != nil {
		return syncTarget{}, fmt.Errorf("loading sync config: %w", err)
	}
	targets, rErr := resolveTargets(sc.Repos)
	if rErr != nil {
		return syncTarget{}, fmt.Errorf("resolving targets: %w", rErr)
	}
	if len(targets) == 0 {
		return syncTarget{}, fmt.Errorf("no repos found in sync config")
	}
	target := pickTarget(targets, time.Now())
	slog.Info("sync target selected",
		"org", target.Org,
		"repo", target.Repo,
		"index", time.Now().UTC().Hour()%len(targets),
		"total", len(targets),
	)
	return target, nil
}

func findRepo(sc *syncConfig, org, repo string) *syncRepo {
	for i := range sc.Repos {
		if strings.EqualFold(sc.Repos[i].Org, org) && strings.EqualFold(sc.Repos[i].Name, repo) {
			return &sc.Repos[i]
		}
	}
	return nil
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

func runInsightsGeneration(ctx context.Context, store data.Store, target syncTarget) (bool, int) {
	llmCfg := data.NewLLMConfigFromEnv()
	if llmCfg == nil {
		slog.Debug("ANTHROPIC_API_KEY not set, skipping insights generation")
		return false, 0
	}

	genAt, _ := store.GetRepoInsightsGeneratedAt(target.Org, target.Repo)
	if genAt != "" {
		if t, pErr := time.Parse("2006-01-02T15:04:05Z", genAt); pErr == nil {
			if time.Since(t) <= time.Duration(target.InsightStale)*time.Hour {
				slog.Debug("insights fresh, skipping", "org", target.Org, "repo", target.Repo, "generated_at", genAt)
				return false, 0
			}
		}
	}

	period := target.InsightPeriod
	model := llmCfg.Model
	if model == "" {
		model = data.DefaultInsightsModel
	}
	slog.Info("generating insights", "org", target.Org, "repo", target.Repo, "period_months", period, "model", model, "base_url", llmCfg.BaseURL)

	metrics, gatherErr := data.GatherInsightsMetrics(store, target.Org, target.Repo, period)
	if gatherErr != nil {
		slog.Error("failed to gather metrics", "error", gatherErr)
		return false, 1
	}

	gi, usedModel, genErr := data.GenerateInsights(ctx, llmCfg, metrics, period)
	if genErr != nil {
		slog.Error("insights generation failed", "error", genErr)
		return false, 1
	}

	now := time.Now().UTC().Format("2006-01-02T15:04:05Z")
	ri := &data.RepoInsights{
		Insights:     gi,
		PeriodMonths: period,
		Model:        usedModel,
		GeneratedAt:  now,
	}
	if saveErr := store.SaveRepoInsights(target.Org, target.Repo, ri); saveErr != nil {
		slog.Error("failed to save insights", "error", saveErr)
		return false, 1
	}

	return true, 0
}
