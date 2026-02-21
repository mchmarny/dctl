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

	repoNameFlag = &cli.StringFlag{
		Name:  "repo",
		Usage: "Name of the GitHub repository",
	}

	monthsFlag = &cli.IntFlag{
		Name:  "months",
		Usage: fmt.Sprintf("Number of months to import (default: %d)", data.EventAgeMonthsDefault),
		Value: data.EventAgeMonthsDefault,
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

	freshFlag = &cli.BoolFlag{
		Name:  "fresh",
		Usage: "Clear pagination state and re-import from scratch",
	}

	importCmd = &cli.Command{
		Name:    "import",
		Aliases: []string{"i"},
		Usage:   "List data import operations",
		Subcommands: []*cli.Command{
			{
				Name:    "events",
				Aliases: []string{"e"},
				Usage:   "Imports GitHub repo event data (PRs, comments, issues, etc)",
				Action:  cmdImportEvents,
				Flags: []cli.Flag{
					orgNameFlag,
					repoNameFlag,
					monthsFlag,
					freshFlag,
				},
			},
			{
				Name:    "affiliations",
				Aliases: []string{"a"},
				Usage:   "Updates imported developer entity/identity with CNCF and GitHub data",
				Action:  cmdImportAffiliations,
			},
			{
				Name:    "substitutions",
				Aliases: []string{"s"},
				Usage:   "Create a global data substitutions (e.g. standardize entity name)",
				Action:  cmdSubstitutes,
				Flags: []cli.Flag{
					subTypeFlag,
					oldValFlag,
					newValFlag,
				},
			},
			{
				Name:    "updates",
				Aliases: []string{"u"},
				Usage:   "Update all previously imported org, repos, and affiliations",
				Action:  cmdUpdate,
			},
			{
				Name:    "all",
				Aliases: []string{"al"},
				Usage:   "Import everything (events, affiliations, metadata, releases) for a given org",
				Action:  cmdImportAll,
				Flags: []cli.Flag{
					orgNameFlag,
					repoNameFlag,
					monthsFlag,
					freshFlag,
				},
			},
			{
				Name:    "metadata",
				Aliases: []string{"m"},
				Usage:   "Import repository metadata (stars, forks, language, license, etc)",
				Action:  cmdImportMetadata,
				Flags: []cli.Flag{
					orgNameFlag,
					repoNameFlag,
				},
			},
			{
				Name:    "releases",
				Aliases: []string{"r"},
				Usage:   "Import repository releases and tags",
				Action:  cmdImportReleases,
				Flags: []cli.Flag{
					orgNameFlag,
					repoNameFlag,
				},
			},
		},
	}
)

type EventImportResult struct {
	Org      string         `json:"org,omitempty"`
	Repos    []string       `json:"repos,omitempty"`
	Duration string         `json:"duration,omitempty"`
	Imported map[string]int `json:"imported,omitempty"`
}

type EventUpdateResult struct {
	Duration    string                        `json:"duration,omitempty"`
	Imported    map[string]int                `json:"imported,omitempty"`
	Updated     *data.AffiliationImportResult `json:"updated,omitempty"`
	Substituted []*data.Substitution          `json:"substituted,omitempty"`
}

func cmdUpdate(c *cli.Context) error {
	start := time.Now()
	token, err := getGitHubToken()
	if err != nil {
		return fmt.Errorf("failed to get GitHub token: %w", err)
	}

	if token == "" {
		return cli.ShowSubcommandHelp(c)
	}

	cfg := getConfig(c)
	res := &EventUpdateResult{}

	m, err := data.UpdateEvents(cfg.DBPath, token)
	if err != nil {
		return fmt.Errorf("failed to import events: %w", err)
	}

	// update final state
	res.Imported = m
	res.Duration = time.Since(start).String()

	// also update affiliations
	a, err := importAffiliations(cfg.DB)
	if err != nil {
		return fmt.Errorf("failed to import affiliations: %w", err)
	}
	res.Updated = a

	// also update substitutes
	sub, err := data.ApplySubstitutions(cfg.DB)
	if err != nil {
		return fmt.Errorf("failed to apply substitutions: %w", err)
	}
	res.Substituted = sub

	// also update repo metadata
	if err := data.ImportAllRepoMeta(cfg.DBPath, token); err != nil {
		slog.Error("failed to import repo metadata", "error", err)
	}

	// also update releases
	if err := data.ImportAllReleases(cfg.DBPath, token); err != nil {
		slog.Error("failed to import releases", "error", err)
	}

	if err := getEncoder().Encode(res); err != nil {
		return fmt.Errorf("error encoding list: %+v: %w", res, err)
	}

	return nil
}

type ImportAllResult struct {
	Org          string                        `json:"org"`
	Repos        []string                      `json:"repos"`
	Duration     string                        `json:"duration"`
	Events       map[string]int                `json:"events,omitempty"`
	Affiliations *data.AffiliationImportResult `json:"affiliations,omitempty"`
	Substituted  []*data.Substitution          `json:"substituted,omitempty"`
}

func cmdImportAll(c *cli.Context) error {
	start := time.Now()
	org := c.String(orgNameFlag.Name)
	repo := c.String(repoNameFlag.Name)
	months := c.Int(monthsFlag.Name)

	token, err := getGitHubToken()
	if err != nil {
		return fmt.Errorf("failed to get GitHub token: %w", err)
	}

	if org == "" || token == "" {
		return cli.ShowSubcommandHelp(c)
	}

	cfg := getConfig(c)

	// resolve repos
	var repos []string
	if repo == "" {
		ctx := context.Background()
		client := net.GetOAuthClient(ctx, token)
		repos, err = data.GetOrgRepoNames(ctx, client, org)
		if err != nil {
			return fmt.Errorf("failed to get org %s repos: %w", org, err)
		}
	} else {
		repos = strings.Split(repo, ",")
	}

	res := &ImportAllResult{
		Org:    org,
		Repos:  repos,
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
		m, importErr := data.ImportEvents(cfg.DBPath, token, org, r, months)
		if importErr != nil {
			slog.Error("failed to import events", "org", org, "repo", r, "error", importErr)
			continue
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

	// 4. metadata
	for _, r := range repos {
		slog.Info("importing metadata", "org", org, "repo", r)
		if err := data.ImportRepoMeta(cfg.DBPath, token, org, r); err != nil {
			slog.Error("failed to import repo metadata", "org", org, "repo", r, "error", err)
		}
	}

	// 5. releases
	for _, r := range repos {
		slog.Info("importing releases", "org", org, "repo", r)
		if err := data.ImportReleases(cfg.DBPath, token, org, r); err != nil {
			slog.Error("failed to import releases", "org", org, "repo", r, "error", err)
		}
	}

	res.Duration = time.Since(start).String()

	if err := getEncoder().Encode(res); err != nil {
		return fmt.Errorf("error encoding result: %w", err)
	}

	return nil
}

func cmdImportEvents(c *cli.Context) error {
	start := time.Now()
	org := c.String(orgNameFlag.Name)
	repo := c.String(repoNameFlag.Name)
	months := c.Int(monthsFlag.Name)
	token, err := getGitHubToken()
	if err != nil {
		return fmt.Errorf("failed to get GitHub token: %w", err)
	}

	if org == "" || token == "" {
		return cli.ShowSubcommandHelp(c)
	}

	var repos []string

	if repo == "" {
		ctx := context.Background()
		client := net.GetOAuthClient(ctx, token)
		repos, err = data.GetOrgRepoNames(ctx, client, org)
		if err != nil {
			return fmt.Errorf("failed to get org %s repos: %w", org, err)
		}
	} else {
		repos = strings.Split(repo, ",")
	}

	cfg := getConfig(c)

	if c.Bool(freshFlag.Name) {
		for _, r := range repos {
			if err := data.ClearState(cfg.DB, org, r); err != nil {
				return fmt.Errorf("failed to clear state for %s/%s: %w", org, r, err)
			}
		}
		slog.Info("cleared pagination state", "org", org, "repos", len(repos))
	}

	res := &EventImportResult{
		Org:      org,
		Repos:    repos,
		Imported: make(map[string]int),
	}

	for _, r := range repos {
		m, err := data.ImportEvents(cfg.DBPath, token, org, r, months)
		if err != nil {
			return fmt.Errorf("failed to import events: %w", err)
		}
		for k, v := range m {
			res.Imported[k] = v
		}
	}

	res.Duration = time.Since(start).String()

	if err := getEncoder().Encode(res); err != nil {
		return fmt.Errorf("error encoding list: %+v: %w", res, err)
	}

	return nil
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

func cmdImportAffiliations(c *cli.Context) error {
	cfg := getConfig(c)

	res, err := importAffiliations(cfg.DB)
	if err != nil {
		return fmt.Errorf("failed to import affiliations: %w", err)
	}

	if err := getEncoder().Encode(res); err != nil {
		return fmt.Errorf("error encoding list: %+v: %w", res, err)
	}

	return nil
}

func runImport(c *cli.Context, single func(string, string, string, string) error, all func(string, string) error, name string) error {
	org := c.String(orgNameFlag.Name)
	repo := c.String(repoNameFlag.Name)
	token, err := getGitHubToken()
	if err != nil {
		return fmt.Errorf("failed to get GitHub token: %w", err)
	}

	if token == "" {
		return cli.ShowSubcommandHelp(c)
	}

	cfg := getConfig(c)

	if org != "" && repo != "" {
		if err := single(cfg.DBPath, token, org, repo); err != nil {
			return fmt.Errorf("failed to import %s: %w", name, err)
		}
	} else {
		if err := all(cfg.DBPath, token); err != nil {
			return fmt.Errorf("failed to import all %s: %w", name, err)
		}
	}

	fmt.Printf("%s import complete\n", name)
	return nil
}

func cmdImportMetadata(c *cli.Context) error {
	return runImport(c, data.ImportRepoMeta, data.ImportAllRepoMeta, "metadata")
}

func cmdImportReleases(c *cli.Context) error {
	return runImport(c, data.ImportReleases, data.ImportAllReleases, "releases")
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
		return fmt.Errorf("failed to update names from apache foundation: %w", err)
	}

	if err := getEncoder().Encode(res); err != nil {
		return fmt.Errorf("error encoding list: %+v: %w", res, err)
	}

	return nil
}
