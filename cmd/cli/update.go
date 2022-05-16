package main

import (
	"encoding/json"
	"os"
	"time"

	"github.com/mchmarny/dctl/pkg/data"
	"github.com/pkg/errors"
	"github.com/urfave/cli/v2"
)

var (
	updateCmd = &cli.Command{
		Name:    "update",
		Aliases: []string{"u"},
		Usage:   "Update all previously imported org, repos, and affiliations",
		Action:  cmdUpdate,
	}
)

type EventUpdateResult struct {
	Duration string         `json:"duration,omitempty"`
	Imported map[string]int `json:"imported,omitempty"`
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

	// update final state
	res.Imported = m
	res.Duration = time.Since(start).String()

	if err := json.NewEncoder(os.Stdout).Encode(res); err != nil {
		return errors.Wrapf(err, "error encoding list: %+v", res)
	}

	return nil
}
