package cli

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/mchmarny/devpulse/pkg/data"
	"github.com/mchmarny/devpulse/pkg/data/sqlite"
	"github.com/mchmarny/devpulse/pkg/net"
	"github.com/urfave/cli/v3"
)

var (
	orgNameFlag = &cli.StringFlag{
		Name:    "org",
		Usage:   "Name of the GitHub organization or user",
		Sources: cli.EnvVars("DEVPULSE_ORG"),
	}

	repoNameFlag = &cli.StringSliceFlag{
		Name:    "repo",
		Usage:   "Name of the GitHub repository (can be specified multiple times)",
		Sources: cli.EnvVars("DEVPULSE_REPO"),
	}

	monthsFlag = &cli.IntFlag{
		Name:    "months",
		Usage:   "Number of months to import",
		Value:   data.EventAgeMonthsDefault,
		Sources: cli.EnvVars("DEVPULSE_MONTHS"),
	}

	freshFlag = &cli.BoolFlag{
		Name:    "fresh",
		Usage:   "Clear pagination state and re-import from scratch",
		Sources: cli.EnvVars("DEVPULSE_FRESH"),
	}

	concurrencyFlag = &cli.IntFlag{
		Name:    "concurrency",
		Usage:   "Number of repos to import concurrently",
		Value:   3,
		Sources: cli.EnvVars("DEVPULSE_CONCURRENCY"),
	}

	importCmd = &cli.Command{
		Name:            "import",
		Aliases:         []string{"imp"},
		HideHelpCommand: true,
		Usage:           "Import GitHub data (events, affiliations, metadata, releases, reputation)",
		UsageText: `devpulse import --org <ORG> --repo <REPO> [--months <N>] [--fresh]

Examples:
  devpulse import --org <ORG> --repo <REPO1> --repo <REPO2>    # import specific repos
  devpulse import --org <ORG> --repo <REPO1> --months 24       # import last 24 months for specific repo
  devpulse import --org <ORG> --repo <REPO1> --fresh           # re-import from scratch
  devpulse import                                              # update all previously imported data`,
		Action: cmdImport,
		Flags: []cli.Flag{
			orgNameFlag,
			repoNameFlag,
			monthsFlag,
			freshFlag,
			concurrencyFlag,
			formatFlag,
			debugFlag,
		},
	}
)

type ImportResult struct {
	Org          string                        `json:"org,omitempty" yaml:"org,omitempty"`
	Repos        []*data.ImportSummary         `json:"repos,omitempty" yaml:"repos,omitempty"`
	Duration     string                        `json:"duration" yaml:"duration,omitempty"`
	Events       map[string]int                `json:"events,omitempty" yaml:"events,omitempty"`
	Affiliations *data.AffiliationImportResult `json:"affiliations,omitempty" yaml:"affiliations,omitempty"`
	Substituted  []*data.Substitution          `json:"substituted,omitempty" yaml:"substituted,omitempty"`
	Reputation   *data.ReputationResult        `json:"reputation,omitempty" yaml:"reputation,omitempty"`
}

func cmdImport(ctx context.Context, cmd *cli.Command) error {
	start := time.Now()
	applyFlags(cmd)

	org := cmd.String(orgNameFlag.Name)
	repos := cmd.StringSlice(repoNameFlag.Name)
	months := cmd.Int(monthsFlag.Name)

	token, err := requireGitHubToken()
	if err != nil {
		return err
	}

	cfg := getConfig(cmd)

	concurrency := cmd.Int(concurrencyFlag.Name)
	if concurrency > concurrencyFlag.Value {
		slog.Warn("high concurrency may trigger GitHub API rate limits", "concurrency", concurrency)
	}

	// If no org specified, update all previously imported data.
	if org == "" {
		return cmdUpdate(ctx, cfg, token, concurrency, start)
	}

	// At least one repo is required when org is specified
	if len(repos) == 0 {
		return fmt.Errorf("--repo is required when --org is specified (e.g. --org %s --repo <REPO>)", org)
	}

	res := &ImportResult{
		Org:    org,
		Repos:  make([]*data.ImportSummary, 0, len(repos)),
		Events: make(map[string]int),
	}

	// 0. clear state if fresh
	if cmd.Bool(freshFlag.Name) {
		for _, r := range repos {
			if clearErr := cfg.Store.ClearState(org, r); clearErr != nil {
				slog.Error("failed to clear state", "org", org, "repo", r, "error", clearErr)
			}
		}
		slog.Info("cleared pagination state", "org", org, "repos", len(repos))
	}

	// 1. events
	for _, r := range repos {
		m, summary, importErr := cfg.Store.ImportEvents(ctx, token, org, r, months)
		if importErr != nil {
			slog.Error("failed to import events", "org", org, "repo", r, "error", importErr)
			continue
		}
		if summary != nil {
			res.Repos = append(res.Repos, summary)
		}
		for k, v := range m {
			res.Events[k] += v
		}
	}

	// 2. affiliations
	slog.Info("updating affiliations")
	a, err := importAffiliations(ctx, cfg.Store)
	if err != nil {
		slog.Error("affiliations failed", "error", err)
	} else {
		res.Affiliations = a
	}

	// 3. substitutions
	slog.Info("applying substitutions")
	sub, err := cfg.Store.ApplySubstitutions()
	if err != nil {
		slog.Error("substitutions failed", "error", err)
	} else {
		res.Substituted = sub
	}

	// 4. metadata + releases
	importRepoExtras(ctx, cfg.Store, token, org, repos)

	// 5. reputation (shallow — local DB only, no API calls)
	orgPtr := &org
	slog.Info("computing reputation")
	repResult, repErr := cfg.Store.ImportReputation(orgPtr, nil)
	if repErr != nil {
		slog.Error("reputation failed", "error", repErr)
	} else {
		res.Reputation = repResult
	}

	res.Duration = time.Since(start).String()

	if err := encode(res); err != nil {
		return fmt.Errorf("error encoding result: %w", err)
	}

	return nil
}

func cmdUpdate(ctx context.Context, cfg *appConfig, token string, concurrency int, start time.Time) error {
	slog.Info("updating all previously imported data", "concurrency", concurrency)

	m, err := cfg.Store.UpdateEvents(ctx, token, concurrency)
	if err != nil {
		return fmt.Errorf("failed to import events: %w", err)
	}

	slog.Info("updating affiliations")
	a, err := importAffiliations(ctx, cfg.Store)
	if err != nil {
		slog.Error("affiliations failed", "error", err)
	}

	slog.Info("applying substitutions")
	sub, err := cfg.Store.ApplySubstitutions()
	if err != nil {
		slog.Error("substitutions failed", "error", err)
	}

	slog.Info("updating metadata")
	if metaErr := cfg.Store.ImportAllRepoMeta(ctx, token); metaErr != nil {
		slog.Error("metadata failed", "error", metaErr)
	}

	slog.Info("updating releases")
	if relErr := cfg.Store.ImportAllReleases(ctx, token); relErr != nil {
		slog.Error("releases failed", "error", relErr)
	}

	slog.Info("updating metric history")
	if histErr := cfg.Store.ImportAllRepoMetricHistory(ctx, token); histErr != nil {
		slog.Error("metric history failed", "error", histErr)
	}

	slog.Info("updating container versions")
	if cvErr := cfg.Store.ImportAllContainerVersions(ctx, token); cvErr != nil {
		slog.Error("container versions failed", "error", cvErr)
	}

	slog.Info("computing reputation")
	repResult, repErr := cfg.Store.ImportReputation(nil, nil)
	if repErr != nil {
		slog.Error("reputation failed", "error", repErr)
	}

	res := &ImportResult{
		Events:       m,
		Affiliations: a,
		Substituted:  sub,
		Reputation:   repResult,
		Duration:     time.Since(start).String(),
	}

	if err := encode(res); err != nil {
		return fmt.Errorf("error encoding result: %w", err)
	}

	return nil
}

func importRepoExtras(ctx context.Context, store data.Store, token, org string, repos []string) {
	for _, r := range repos {
		slog.Info("updating extras", "repo", org+"/"+r)

		if err := store.ImportRepoMeta(ctx, token, org, r); err != nil {
			slog.Error("failed to import repo metadata", "org", org, "repo", r, "error", err)
		}
		if err := store.ImportReleases(ctx, token, org, r); err != nil {
			slog.Error("failed to import releases", "org", org, "repo", r, "error", err)
		}
		if err := store.ImportRepoMetricHistory(ctx, token, org, r); err != nil {
			slog.Error("failed to import metric history", "org", org, "repo", r, "error", err)
		}
		if err := store.ImportContainerVersions(ctx, token, org, r); err != nil {
			slog.Error("failed to import container versions", "org", org, "repo", r, "error", err)
		}
	}
}

func importAffiliations(ctx context.Context, store data.Store) (*data.AffiliationImportResult, error) {
	token, err := requireGitHubToken()
	if err != nil {
		return nil, err
	}

	client := net.GetOAuthClient(ctx, token)

	res, err := sqlite.UpdateDevelopersWithCNCFEntityAffiliations(ctx, store, store, client)
	if err != nil {
		return nil, fmt.Errorf("failed to import affiliations: %w", err)
	}

	return res, nil
}
