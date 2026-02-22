package cli

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"errors"

	"github.com/mchmarny/dctl/pkg/data"
	"github.com/mchmarny/dctl/pkg/net"
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
		Usage: fmt.Sprintf("Number of months to import (default: %d)", data.EventAgeMonthsDefault),
		Value: data.EventAgeMonthsDefault,
	}

	freshFlag = &cli.BoolFlag{
		Name:  "fresh",
		Usage: "Clear pagination state and re-import from scratch",
	}

	importCmd = &cli.Command{
		Name:    "import",
		Aliases: []string{"i"},
		Usage:   "Import GitHub data (events, affiliations, metadata, releases, reputation)",
		UsageText: `dctl import --org NVIDIA --repo NVSentinel --repo skyhook   # import specific repos
   dctl import --org NVIDIA                                     # import all org repos
   dctl import                                                  # update all previously imported data
   dctl import --org NVIDIA --fresh                             # re-import from scratch`,
		Action: cmdImport,
		Flags: []cli.Flag{
			orgNameFlag,
			repoNameFlag,
			monthsFlag,
			freshFlag,
		},
	}

	subTypeFlag = &cli.StringFlag{
		Name:  "type",
		Usage: fmt.Sprintf("Substitution type [%s]", strings.Join(data.UpdatableProperties, ",")),
	}

	oldValFlag = &cli.StringFlag{
		Name:     "old",
		Usage:    "Old value",
		Required: true,
	}

	newValFlag = &cli.StringFlag{
		Name:     "new",
		Usage:    "New value",
		Required: true,
	}

	substituteCmd = &cli.Command{
		Name:    "substitute",
		Aliases: []string{"sub"},
		Usage:   "Create a global data substitution (e.g. standardize entity name)",
		Action:  cmdSubstitutes,
		Flags: []cli.Flag{
			subTypeFlag,
			oldValFlag,
			newValFlag,
		},
	}
)

type ImportResult struct {
	Org          string                        `json:"org,omitempty"`
	Repos        []*data.ImportSummary         `json:"repos,omitempty"`
	Duration     string                        `json:"duration"`
	Events       map[string]int                `json:"events,omitempty"`
	Affiliations *data.AffiliationImportResult `json:"affiliations,omitempty"`
	Substituted  []*data.Substitution          `json:"substituted,omitempty"`
	Reputation   *data.ReputationResult        `json:"reputation,omitempty"`
}

func cmdImport(c *cli.Context) error {
	start := time.Now()
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
		slog.Info("importing events", "org", org, "repo", r)
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
	slog.Info("importing affiliations")
	a, err := importAffiliations(cfg.DB)
	if err != nil {
		slog.Error("failed to import affiliations", "error", err)
	} else {
		res.Affiliations = a
	}

	// 3. substitutions
	slog.Info("applying substitutions")
	sub, err := data.ApplySubstitutions(cfg.DB)
	if err != nil {
		slog.Error("failed to apply substitutions", "error", err)
	} else {
		res.Substituted = sub
	}

	// 4. metadata + releases
	importRepoExtras(cfg.DBPath, token, org, repos)

	// 5. reputation (shallow — local DB only)
	slog.Info("computing reputation scores")
	repResult, err := data.ImportReputation(cfg.DB)
	if err != nil {
		slog.Error("failed to compute reputation scores", "error", err)
	} else {
		res.Reputation = repResult
	}

	res.Duration = time.Since(start).String()

	if err := getEncoder().Encode(res); err != nil {
		return fmt.Errorf("error encoding result: %w", err)
	}

	return nil
}

func cmdUpdate(_ *cli.Context, cfg *appConfig, token string, start time.Time) error {
	slog.Info("no org specified, updating all previously imported data")

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

	if err := getEncoder().Encode(res); err != nil {
		return fmt.Errorf("error encoding result: %w", err)
	}

	return nil
}

func importRepoExtras(dbPath, token, org string, repos []string) {
	for _, r := range repos {
		slog.Info("importing metadata", "org", org, "repo", r)
		if err := data.ImportRepoMeta(dbPath, token, org, r); err != nil {
			slog.Error("failed to import repo metadata", "org", org, "repo", r, "error", err)
		}
	}
	for _, r := range repos {
		slog.Info("importing releases", "org", org, "repo", r)
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

func cmdSubstitutes(c *cli.Context) error {
	sub := c.String(subTypeFlag.Name)
	old := c.String(oldValFlag.Name)
	new := c.String(newValFlag.Name)

	if sub == "" || old == "" || new == "" {
		return cli.ShowSubcommandHelp(c)
	}

	cfg := getConfig(c)

	res, err := data.SaveAndApplyDeveloperSub(cfg.DB, sub, old, new)
	if err != nil {
		return fmt.Errorf("failed to apply substitution: %w", err)
	}

	if err := getEncoder().Encode(res); err != nil {
		return fmt.Errorf("error encoding result: %w", err)
	}

	return nil
}
