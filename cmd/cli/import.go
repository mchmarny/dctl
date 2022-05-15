package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/mchmarny/dctl/pkg/data"
	"github.com/mchmarny/dctl/pkg/net"
	"github.com/pkg/errors"
	"github.com/urfave/cli/v2"
)

var (
	ghTokenFlag = &cli.StringFlag{
		Name:        "token",
		Usage:       "GitHub user token",
		Required:    true,
		EnvVars:     []string{gitHubAccessTokenEnvVar},
		DefaultText: "variable",
	}

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

	importAffiliationCmd = &cli.Command{
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
					ghTokenFlag,
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
				Flags: []cli.Flag{
					ghTokenFlag,
				},
			},
			{
				Name:    "names",
				Aliases: []string{"n"},
				Usage:   "Updates imported developer names with Apache Foundation data",
				Action:  cmdUpdateAFNames,
			},
		},
	}
)

type EventImportResult struct {
	Org      string         `json:"org,omitempty"`
	Repo     string         `json:"repo,omitempty"`
	Duration string         `json:"duration,omitempty"`
	Imported map[string]int `json:"imported,omitempty"`
}

func cmdImportEvents(c *cli.Context) error {
	start := time.Now()
	org := c.String(orgNameFlag.Name)
	repo := c.String(repoNameFlag.Name)
	months := c.Int(monthsFlag.Name)
	token := c.String(ghTokenFlag.Name)

	if org == "" || repo == "" || token == "" {
		return cli.ShowSubcommandHelp(c)
	}

	res := &EventImportResult{
		Org:  org,
		Repo: repo,
	}

	m, err := data.ImportEvents(dbFilePath, token, org, repo, months)
	if err != nil {
		return errors.Wrap(err, "failed to import events")
	}

	// update final state
	res.Imported = m
	res.Duration = time.Since(start).String()

	if err := json.NewEncoder(os.Stdout).Encode(res); err != nil {
		return errors.Wrapf(err, "error encoding list: %+v", res)
	}

	return nil
}

func cmdImportAffiliations(c *cli.Context) error {
	token := c.String(ghTokenFlag.Name)

	if token == "" {
		return cli.ShowSubcommandHelp(c)
	}

	ctx := context.Background()
	client := net.GetOAuthClient(ctx, token)

	db := getDBOrFail()
	defer db.Close()

	res, err := data.UpdateDevelopersWithCNCFEntityAffiliations(ctx, db, client)
	if err != nil {
		return errors.Wrap(err, "failed to import affiliations")
	}

	if err := json.NewEncoder(os.Stdout).Encode(res); err != nil {
		return errors.Wrapf(err, "error encoding list: %+v", res)
	}

	return nil
}

func cmdUpdateAFNames(c *cli.Context) error {
	db := getDBOrFail()
	defer db.Close()

	res, err := data.UpdateNoFullnameDevelopersFromApache(db)
	if err != nil {
		return errors.Wrap(err, "failed to update names from apache foundation")
	}

	if err := json.NewEncoder(os.Stdout).Encode(res); err != nil {
		return errors.Wrapf(err, "error encoding list: %+v", res)
	}

	return nil
}
