package cli

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/mchmarny/devpulse/pkg/data"
	"github.com/mchmarny/devpulse/pkg/data/ghutil"
	"github.com/mchmarny/devpulse/pkg/net"
	"github.com/urfave/cli/v3"
)

const (
	queryResultLimitDefault = 500
)

var (
	ghUserNameQueryFlag = &cli.StringFlag{
		Name:     "username",
		Usage:    "GitHub username",
		Required: true,
		Sources:  cli.EnvVars("DEVPULSE_USERNAME"),
	}

	developerLikeQueryFlag = &cli.StringFlag{
		Name:     "like",
		Usage:    "GitHub user like query (e.g. username, email, company)",
		Required: true,
		Sources:  cli.EnvVars("DEVPULSE_LIKE"),
	}

	entityLikeQueryFlag = &cli.StringFlag{
		Name:     "like",
		Usage:    "Entity like query (e.g. company name)",
		Required: true,
		Sources:  cli.EnvVars("DEVPULSE_LIKE"),
	}

	entityNameQueryFlag = &cli.StringFlag{
		Name:     "name",
		Usage:    "Entity name",
		Required: true,
		Sources:  cli.EnvVars("DEVPULSE_NAME"),
	}

	eventAuthorFlag = &cli.StringFlag{
		Name:    "author",
		Usage:   "Event author (GitHub username)",
		Sources: cli.EnvVars("DEVPULSE_AUTHOR"),
	}

	eventEntityFlag = &cli.StringFlag{
		Name:    "entity",
		Usage:   "Event entity (company name or affiliated organization)",
		Sources: cli.EnvVars("DEVPULSE_ENTITY"),
	}

	eventTypeFlag = &cli.StringFlag{
		Name:    "type",
		Usage:   "Event type (pr, issue, issue_comment, pr_review, fork)",
		Sources: cli.EnvVars("DEVPULSE_EVENT_TYPE"),
	}

	eventSinceFlag = &cli.StringFlag{
		Name:    "since",
		Usage:   "Event since date (YYYY-MM-DD)",
		Sources: cli.EnvVars("DEVPULSE_SINCE"),
	}

	eventMentionFlag = &cli.StringFlag{
		Name:    "mention",
		Usage:   "GitHub mention (like query on @username in body or assignments)",
		Sources: cli.EnvVars("DEVPULSE_MENTION"),
	}

	eventLabelFlag = &cli.StringFlag{
		Name:    "label",
		Usage:   "GitHub label (like query on issues and PRs)",
		Sources: cli.EnvVars("DEVPULSE_LABEL"),
	}

	queryLimitFlag = &cli.IntFlag{
		Name:    "limit",
		Usage:   fmt.Sprintf("Limits number of result returned (default: %d)", queryResultLimitDefault),
		Value:   queryResultLimitDefault,
		Sources: cli.EnvVars("DEVPULSE_LIMIT"),
	}

	// commonFlags are shared across all query subcommands.
	commonFlags = []cli.Flag{formatFlag, debugFlag}

	queryCmd = &cli.Command{
		Name:            "query",
		HideHelpCommand: true,
		Usage:           "Query imported data",
		Flags:           commonFlags,
		UsageText: `devpulse query <subcommand> [options]

Examples:
  devpulse query events --org <ORG> --repo <REPO>                  # list events for a repo
  devpulse query events --org <ORG> --type pr --since 2025-01-01   # PRs since date
  devpulse query developer list --org <ORG>                        # list developers
  devpulse query entity list --org <ORG>                           # list entities
  devpulse query org repos --org <ORG>                             # list org repos`,
		Commands: []*cli.Command{
			{
				Name:            "developers",
				Aliases:         []string{"developer", "dev"},
				HideHelpCommand: true,
				Usage:           "List developer operations",
				Commands: []*cli.Command{
					{
						Name:   "list",
						Usage:  "List developers",
						Action: cmdQueryDevelopers,
						Flags: append(commonFlags,
							developerLikeQueryFlag,
							queryLimitFlag,
						),
					},
					{
						Name:    "details",
						Aliases: []string{"detail"},
						Usage:   "Get specific developer details, identities and associated entities",
						Action:  cmdQueryDeveloper,
						Flags: append(commonFlags,
							ghUserNameQueryFlag,
						),
					},
				},
			},
			{
				Name:    "entities",
				Usage:   "List entity operations",
				Aliases: []string{"entity"},
				Commands: []*cli.Command{
					{
						Name:   "list",
						Usage:  "List entities (companies or organizations with which users are affiliated)",
						Action: cmdQueryEntities,
						Flags: append(commonFlags,
							entityLikeQueryFlag,
							queryLimitFlag,
						),
					},
					{
						Name:    "details",
						Aliases: []string{"detail"},
						Usage:   "Get specific entity and its associated developers",
						Action:  cmdQueryEntity,
						Flags: append(commonFlags,
							entityNameQueryFlag,
						),
					},
				},
			},
			{
				Name:  "org",
				Usage: "List GitHub org/user operations",
				Commands: []*cli.Command{
					{
						Name:    "repos",
						Aliases: []string{"repo"},
						Usage:   "List GitHub org/user repositories",
						Action:  cmdQueryOrgRepos,
						Flags: append(commonFlags,
							orgNameFlag,
						),
					},
				},
			},
			{
				Name:    "events",
				Usage:   "List GitHub events",
				Aliases: []string{"event"},
				Action:  cmdQueryEvents,
				Flags: append(commonFlags,
					orgNameFlag,
					repoNameFlag,
					eventTypeFlag,
					eventSinceFlag,
					eventAuthorFlag,
					eventEntityFlag,
					eventMentionFlag,
					eventLabelFlag,
					queryLimitFlag,
				),
			},
		},
	}
)

func optional(val string) *string {
	if val == "" {
		return nil
	}
	return &val
}

func cmdQueryEntity(_ context.Context, cmd *cli.Command) error {
	applyFlags(cmd)
	val := cmd.String(entityNameQueryFlag.Name)
	if val == "" {
		return cli.ShowSubcommandHelp(cmd)
	}

	cfg := getConfig(cmd)

	ent, err := cfg.Store.GetEntity(val)
	if err != nil {
		return fmt.Errorf("failed to query entity: %w", err)
	}

	if err := encode(ent); err != nil {
		return fmt.Errorf("error encoding: %+v: %w", ent, err)
	}

	return nil
}

func cmdQueryEvents(_ context.Context, cmd *cli.Command) error {
	applyFlags(cmd)
	org := cmd.String(orgNameFlag.Name)
	repoSlice := cmd.StringSlice(repoNameFlag.Name)
	var repo string
	if len(repoSlice) > 0 {
		repo = repoSlice[0]
	}
	author := cmd.String(eventAuthorFlag.Name)
	entity := cmd.String(eventEntityFlag.Name)
	since := cmd.String(eventSinceFlag.Name)
	etype := cmd.String(eventTypeFlag.Name)
	mention := cmd.String(eventMentionFlag.Name)
	label := cmd.String(eventLabelFlag.Name)

	limit := cmd.Int(queryLimitFlag.Name)
	if limit == 0 || limit > queryResultLimitDefault {
		limit = queryResultLimitDefault
	}

	slog.Debug("events",
		"org", org,
		"repo", repo,
		"author", author,
		"entity", entity,
		"since", since,
		"type", etype,
		"limit", limit,
		"mention", mention,
		"label", label,
	)

	q := &data.EventSearchCriteria{
		Org:      optional(org),
		Repo:     optional(repo),
		Type:     optional(etype),
		Username: optional(author),
		Entity:   optional(entity),
		FromDate: optional(since),
		Mention:  optional(mention),
		Label:    optional(label),
		Page:     1,
		PageSize: limit,
	}

	cfg := getConfig(cmd)

	list, err := cfg.Store.SearchEvents(q)
	if err != nil {
		return fmt.Errorf("error searching events: %w", err)
	}

	if err := encode(list); err != nil {
		return fmt.Errorf("error encoding: %w", err)
	}

	return nil
}

func cmdQueryList[T any](cmd *cli.Command, flag *cli.StringFlag, fn func(string, int) ([]*T, error)) error {
	applyFlags(cmd)
	val := cmd.String(flag.Name)
	if val == "" {
		return cli.ShowSubcommandHelp(cmd)
	}

	limit := cmd.Int(queryLimitFlag.Name)
	if limit == 0 || limit > queryResultLimitDefault {
		limit = queryResultLimitDefault
	}

	list, err := fn(val, limit)
	if err != nil {
		return fmt.Errorf("error searching: %w", err)
	}

	return encode(list)
}

func cmdQueryEntities(_ context.Context, cmd *cli.Command) error {
	cfg := getConfig(cmd)
	return cmdQueryList(cmd, entityLikeQueryFlag, cfg.Store.QueryEntities)
}

func cmdQueryDeveloper(ctx context.Context, cmd *cli.Command) error {
	applyFlags(cmd)
	val := cmd.String(ghUserNameQueryFlag.Name)
	token, err := getGitHubToken()
	if err != nil {
		return fmt.Errorf("failed to get GitHub token: %w", err)
	}

	if val == "" || token == "" {
		return cli.ShowSubcommandHelp(cmd)
	}

	cfg := getConfig(cmd)

	dev, err := cfg.Store.GetDeveloper(val)
	if err != nil {
		return fmt.Errorf("failed to query developer: %w", err)
	}
	if dev == nil {
		return fmt.Errorf("developer not found: %s", val)
	}

	client := net.GetOAuthClient(ctx, token)
	dev.Organizations, err = ghutil.GetUserOrgs(ctx, client, val, 0)
	if err != nil {
		slog.Warn("failed to get user orgs", "error", err)
	}

	if err := encode(dev); err != nil {
		return fmt.Errorf("error encoding: %+v: %w", dev, err)
	}

	return nil
}

func cmdQueryDevelopers(_ context.Context, cmd *cli.Command) error {
	cfg := getConfig(cmd)
	return cmdQueryList(cmd, developerLikeQueryFlag, cfg.Store.SearchDevelopers)
}

func cmdQueryOrgRepos(ctx context.Context, cmd *cli.Command) error {
	applyFlags(cmd)
	org := cmd.String(orgNameFlag.Name)
	token, err := getGitHubToken()
	if err != nil {
		return fmt.Errorf("failed to get GitHub token: %w", err)
	}

	if org == "" || token == "" {
		return cli.ShowSubcommandHelp(cmd)
	}

	client := net.GetOAuthClient(ctx, token)
	list, err := ghutil.GetOrgRepos(ctx, client, org)
	if err != nil {
		return fmt.Errorf("failed to list org repos: %w", err)
	}

	if err := encode(list); err != nil {
		return fmt.Errorf("error encoding: %w", err)
	}

	return nil
}
