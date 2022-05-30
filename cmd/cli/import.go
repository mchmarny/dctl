package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/mchmarny/dctl/pkg/data"
	"github.com/mchmarny/dctl/pkg/net"
	"github.com/pkg/errors"
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
		Name:  "sub",
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
				Usage:   "Create a global data substitutions (e.g. standardize location or entity name)",
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
		return errors.Wrap(err, "failed to get GitHub token")
	}

	if token == "" {
		return cli.ShowSubcommandHelp(c)
	}

	res := &EventUpdateResult{}

	m, err := data.UpdateEvents(dbFilePath, token)
	if err != nil {
		return errors.Wrap(err, "failed to import events")
	}

	db := getDBOrFail()
	defer db.Close()

	// update final state
	res.Imported = m
	res.Duration = time.Since(start).String()

	// also update affiliations
	a, err := importAffiliations(db)
	if err != nil {
		return errors.Wrap(err, "failed to import affiliations")
	}
	res.Updated = a

	// also update substitutes
	sub, err := data.ApplySubstitutions(db)
	if err != nil {
		return errors.Wrap(err, "failed to apply substitutions")
	}
	res.Substituted = sub

	if err := json.NewEncoder(os.Stdout).Encode(res); err != nil {
		return errors.Wrapf(err, "error encoding list: %+v", res)
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
		return errors.Wrap(err, "failed to get GitHub token")
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
			return errors.Wrapf(err, "failed to get org %s repos", org)
		}
	} else {
		repos = strings.Split(repo, ",")
	}

	res := &EventImportResult{
		Org:      org,
		Repos:    repos,
		Imported: make(map[string]int),
	}

	for _, r := range repos {
		m, err := data.ImportEvents(dbFilePath, token, org, r, months)
		if err != nil {
			return errors.Wrap(err, "failed to import events")
		}
		for k, v := range m {
			res.Imported[k] = v
		}
	}

	res.Duration = time.Since(start).String()

	if err := json.NewEncoder(os.Stdout).Encode(res); err != nil {
		return errors.Wrapf(err, "error encoding list: %+v", res)
	}

	return nil
}

func importAffiliations(db *sql.DB) (*data.AffiliationImportResult, error) {
	token, err := getGitHubToken()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get GitHub token")
	}

	if token == "" {
		return nil, errors.New("no GitHub token")
	}

	ctx := context.Background()
	client := net.GetOAuthClient(ctx, token)

	res, err := data.UpdateDevelopersWithCNCFEntityAffiliations(ctx, db, client)
	if err != nil {
		return nil, errors.Wrap(err, "failed to import affiliations")
	}

	return res, nil
}

func cmdImportAffiliations(c *cli.Context) error {
	db := getDBOrFail()
	defer db.Close()

	res, err := importAffiliations(db)
	if err != nil {
		return errors.Wrap(err, "failed to import affiliations")
	}

	if err := json.NewEncoder(os.Stdout).Encode(res); err != nil {
		return errors.Wrapf(err, "error encoding list: %+v", res)
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

	db := getDBOrFail()
	defer db.Close()

	res, err := data.SaveAndApplyDeveloperSub(db, sub, old, new)
	if err != nil {
		return errors.Wrap(err, "failed to update names from apache foundation")
	}

	if err := json.NewEncoder(os.Stdout).Encode(res); err != nil {
		return errors.Wrapf(err, "error encoding list: %+v", res)
	}

	return nil
}
