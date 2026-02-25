package cli

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"time"

	"errors"

	"github.com/mchmarny/devpulse/pkg/data"
	"github.com/mchmarny/devpulse/pkg/net"
	"github.com/urfave/cli/v2"
)

var (
	orgNameFlag = &cli.StringFlag{
		Name:  "org",
		Usage: "Name of the GitHub organization or user",
	}

	repoNameFlag = &cli.StringSliceFlag{
		Name:  "repo",
		Usage: "Name of the GitHub repository (can be specified multiple times)",
	}

	monthsFlag = &cli.IntFlag{
		Name:  "months",
		Usage: "Number of months to import",
		Value: data.EventAgeMonthsDefault,
	}

	freshFlag = &cli.BoolFlag{
		Name:  "fresh",
		Usage: "Clear pagination state and re-import from scratch",
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
  devpulse import --org <ORG>                                  # import all org repos
  devpulse import --org <ORG> --fresh                          # re-import from scratch
  devpulse import                                              # update all previously imported data`,
		Action: cmdImport,
		Flags: []cli.Flag{
			orgNameFlag,
			repoNameFlag,
			monthsFlag,
			freshFlag,
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

func cmdImport(c *cli.Context) error {
	start := time.Now()
	applyFlags(c)

	org := c.String(orgNameFlag.Name)
	repoSlice := c.StringSlice(repoNameFlag.Name)
	months := c.Int(monthsFlag.Name)

	token, err := getGitHubToken()
	if err != nil {
		return fmt.Errorf("failed to get GitHub token: %w", err)
	}

	if token == "" {
		return cli.ShowSubcommandHelp(c)
	}

	cfg := getConfig(c)

	// If no org specified, update all previously imported data.
	if org == "" {
		return cmdUpdate(c, cfg, token, start)
	}

	// Resolve repos
	var repos []string
	if len(repoSlice) == 0 {
		ctx := context.Background()
		client := net.GetOAuthClient(ctx, token)
		repos, err = data.GetOrgRepoNames(ctx, client, org)
		if err != nil {
			return fmt.Errorf("failed to get org %s repos: %w", org, err)
		}
	} else {
		repos = repoSlice
	}

	res := &ImportResult{
		Org:    org,
		Repos:  make([]*data.ImportSummary, 0, len(repos)),
		Events: make(map[string]int),
	}

	// 0. clear state if fresh
	if c.Bool(freshFlag.Name) {
		for _, r := range repos {
			if clearErr := data.ClearState(cfg.DB, org, r); clearErr != nil {
				slog.Error("failed to clear state", "org", org, "repo", r, "error", clearErr)
			}
		}
		slog.Info("cleared pagination state", "org", org, "repos", len(repos))
	}

	// 1. events
	for _, r := range repos {
		slog.Info("events", "org", org, "repo", r)
		m, summary, importErr := data.ImportEvents(cfg.DBPath, token, org, r, months)
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
	slog.Info("affiliations")
	a, err := importAffiliations(cfg.DB)
	if err != nil {
		slog.Error("failed to import affiliations", "error", err)
	} else {
		res.Affiliations = a
	}

	// 3. substitutions
	slog.Info("substitutions")
	sub, err := data.ApplySubstitutions(cfg.DB)
	if err != nil {
		slog.Error("failed to apply substitutions", "error", err)
	} else {
		res.Substituted = sub
	}

	// 4. metadata + releases
	importRepoExtras(cfg.DBPath, token, org, repos)

	// 5. reputation (shallow — local DB only)
	slog.Info("reputation")
	repResult, err := data.ImportReputation(cfg.DB)
	if err != nil {
		slog.Error("failed to compute reputation scores", "error", err)
	} else {
		res.Reputation = repResult
	}

	res.Duration = time.Since(start).String()

	if err := encode(res); err != nil {
		return fmt.Errorf("error encoding result: %w", err)
	}

	return nil
}

func cmdUpdate(_ *cli.Context, cfg *appConfig, token string, start time.Time) error {
	slog.Info("updating all previously imported data")

	m, err := data.UpdateEvents(cfg.DBPath, token)
	if err != nil {
		return fmt.Errorf("failed to import events: %w", err)
	}

	a, err := importAffiliations(cfg.DB)
	if err != nil {
		slog.Error("failed to import affiliations", "error", err)
	}

	sub, err := data.ApplySubstitutions(cfg.DB)
	if err != nil {
		slog.Error("failed to apply substitutions", "error", err)
	}

	if metaErr := data.ImportAllRepoMeta(cfg.DBPath, token); metaErr != nil {
		slog.Error("failed to import repo metadata", "error", metaErr)
	}

	if relErr := data.ImportAllReleases(cfg.DBPath, token); relErr != nil {
		slog.Error("failed to import releases", "error", relErr)
	}

	repResult, repErr := data.ImportReputation(cfg.DB)
	if repErr != nil {
		slog.Error("failed to compute reputation scores", "error", repErr)
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

func importRepoExtras(dbPath, token, org string, repos []string) {
	for _, r := range repos {
		slog.Info("metadata", "org", org, "repo", r)
		if err := data.ImportRepoMeta(dbPath, token, org, r); err != nil {
			slog.Error("failed to import repo metadata", "org", org, "repo", r, "error", err)
		}
	}
	for _, r := range repos {
		slog.Info("releases", "org", org, "repo", r)
		if err := data.ImportReleases(dbPath, token, org, r); err != nil {
			slog.Error("failed to import releases", "org", org, "repo", r, "error", err)
		}
	}
}

func importAffiliations(db *sql.DB) (*data.AffiliationImportResult, error) {
	token, err := getGitHubToken()
	if err != nil {
		return nil, fmt.Errorf("failed to get GitHub token: %w", err)
	}

	if token == "" {
		return nil, errors.New("no GitHub token")
	}

	ctx := context.Background()
	client := net.GetOAuthClient(ctx, token)

	res, err := data.UpdateDevelopersWithCNCFEntityAffiliations(ctx, db, client)
	if err != nil {
		return nil, fmt.Errorf("failed to import affiliations: %w", err)
	}

	return res, nil
}
