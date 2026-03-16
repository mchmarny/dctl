package cli

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/mchmarny/devpulse/pkg/data"
	"github.com/urfave/cli/v3"
)

var deleteCmd = &cli.Command{
	Name:            "delete",
	Aliases:         []string{"del"},
	HideHelpCommand: true,
	Usage:           "Delete imported data for an org or specific repos",
	UsageText: `devpulse delete --org <ORG> [--repo <REPO>...] [--force]

Examples:
  devpulse delete --org myorg                        # delete all repos in org
  devpulse delete --org myorg --repo repo1           # delete specific repo
  devpulse delete --org myorg --repo r1 --repo r2    # delete multiple repos
  devpulse delete --org myorg --force                # skip confirmation`,
	Action: cmdDelete,
	Flags: []cli.Flag{
		orgNameFlag,
		repoNameFlag,
		forceFlag,
		formatFlag,
		debugFlag,
	},
}

type DeleteCommandResult struct {
	Org      string               `json:"org" yaml:"org"`
	Repos    []*data.DeleteResult `json:"repos" yaml:"repos"`
	Duration string               `json:"duration" yaml:"duration"`
}

func cmdDelete(_ context.Context, cmd *cli.Command) error {
	start := time.Now()
	applyFlags(cmd)

	org := cmd.String(orgNameFlag.Name)
	if org == "" {
		return cli.ShowSubcommandHelp(cmd)
	}

	cfg := getConfig(cmd)

	// Resolve repos
	repos := cmd.StringSlice(repoNameFlag.Name)

	if len(repos) == 0 {
		// Find all repos for this org from existing data
		items, err := cfg.Store.GetAllOrgRepos()
		if err != nil {
			return fmt.Errorf("listing repos for org %s: %w", org, err)
		}
		for _, item := range items {
			if item.Org == org {
				repos = append(repos, item.Repo)
			}
		}
		if len(repos) == 0 {
			slog.Info("no data found for org", "org", org)
			return nil
		}
	}

	// Confirmation prompt
	if !cmd.Bool(forceFlag.Name) {
		fmt.Println("Delete all data for:")
		for _, r := range repos {
			fmt.Printf("  - %s/%s\n", org, r)
		}
		confirmed, err := confirmAction("Continue? [y/N]: ")
		if err != nil {
			return err
		}
		if !confirmed {
			fmt.Println("Aborted.")
			return nil
		}
	}

	res := &DeleteCommandResult{
		Org:   org,
		Repos: make([]*data.DeleteResult, 0, len(repos)),
	}

	for _, r := range repos {
		slog.Info("deleting", "org", org, "repo", r)
		dr, err := cfg.Store.DeleteRepoData(org, r)
		if err != nil {
			slog.Error("failed to delete repo data", "org", org, "repo", r, "error", err)
			continue
		}
		res.Repos = append(res.Repos, dr)
	}

	res.Duration = time.Since(start).String()

	if err := encode(res); err != nil {
		return fmt.Errorf("encoding result: %w", err)
	}

	return nil
}
