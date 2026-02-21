package main

import (
	"context"
	"database/sql"
	"fmt"
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

	if err := getEncoder().Encode(res); err != nil {
		return fmt.Errorf("error encoding list: %+v: %w", res, err)
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
