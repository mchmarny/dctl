package cli

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"

	"github.com/mchmarny/devpulse/pkg/data"
	"github.com/mchmarny/devpulse/pkg/net"
	"github.com/urfave/cli/v2"
)

const (
	queryResultLimitDefault = 500
)

var (
	ghUserNameQueryFlag = &cli.StringFlag{
		Name:     "username",
		Usage:    "GitHub username",
		Required: true,
	}

	developerLikeQueryFlag = &cli.StringFlag{
		Name:     "like",
		Usage:    "GitHub user like query (e.g. username, email, company)",
		Required: true,
	}

	entityLikeQueryFlag = &cli.StringFlag{
		Name:     "like",
		Usage:    "Entity like query (e.g. company name)",
		Required: true,
	}

	entityNameQueryFlag = &cli.StringFlag{
		Name:     "name",
		Usage:    "Entity name",
		Required: true,
	}

	eventAuthorFlag = &cli.StringFlag{
		Name:  "author",
		Usage: "Event author (GitHub username)",
	}

	eventEntityFlag = &cli.StringFlag{
		Name:  "entity",
		Usage: "Event entity (company name or affiliated organization)",
	}

	eventTypeFlag = &cli.StringFlag{
		Name:  "type",
		Usage: "Event type (pr, issue, issue_comment, pr_review, fork)",
	}

	eventSinceFlag = &cli.StringFlag{
		Name:  "since",
		Usage: "Event since date (YYYY-MM-DD)",
	}

	eventMentionFlag = &cli.StringFlag{
		Name:  "mention",
		Usage: "GitHub mention (like query on @username in body or assignments)",
	}

	eventLabelFlag = &cli.StringFlag{
		Name:     "label",
		Usage:    "GitHub label (like query on issues and PRs)",
		Required: false,
	}

	queryLimitFlag = &cli.IntFlag{
		Name:  "limit",
		Usage: fmt.Sprintf("Limits number of result returned (default: %d)", queryResultLimitDefault),
		Value: queryResultLimitDefault,
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
		Subcommands: []*cli.Command{
			{
				Name:            "developers",
				Aliases:         []string{"developer", "dev"},
				HideHelpCommand: true,
				Usage:           "List developer operations",
				Subcommands: []*cli.Command{
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
				Subcommands: []*cli.Command{
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
				Subcommands: []*cli.Command{
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

func cmdQueryEntity(c *cli.Context) error {
	applyFlags(c)
	val := c.String(entityNameQueryFlag.Name)
	if val == "" {
		return cli.ShowSubcommandHelp(c)
	}

	cfg := getConfig(c)

	ent, err := data.GetEntity(cfg.DB, val)
	if err != nil {
		return fmt.Errorf("failed to query entity: %w", err)
	}

	if err := encode(ent); err != nil {
		return fmt.Errorf("error encoding: %+v: %w", ent, err)
	}

	return nil
}

func cmdQueryEvents(c *cli.Context) error {
	applyFlags(c)
	org := c.String(orgNameFlag.Name)
	repoSlice := c.StringSlice(repoNameFlag.Name)
	var repo string
	if len(repoSlice) > 0 {
		repo = repoSlice[0]
	}
	author := c.String(eventAuthorFlag.Name)
	entity := c.String(eventEntityFlag.Name)
	since := c.String(eventSinceFlag.Name)
	etype := c.String(eventTypeFlag.Name)
	mention := c.String(eventMentionFlag.Name)
	label := c.String(eventLabelFlag.Name)

	limit := c.Int(queryLimitFlag.Name)
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

	cfg := getConfig(c)

	list, err := data.SearchEvents(cfg.DB, q)
	if err != nil {
		return fmt.Errorf("error searching events: %w", err)
	}

	if err := encode(list); err != nil {
		return fmt.Errorf("error encoding: %w", err)
	}

	return nil
}

func cmdQueryList[T any](c *cli.Context, flag *cli.StringFlag, fn func(*sql.DB, string, int) ([]*T, error)) error {
	applyFlags(c)
	val := c.String(flag.Name)
	if val == "" {
		return cli.ShowSubcommandHelp(c)
	}

	cfg := getConfig(c)

	limit := c.Int(queryLimitFlag.Name)
	if limit == 0 || limit > queryResultLimitDefault {
		limit = queryResultLimitDefault
	}

	list, err := fn(cfg.DB, val, limit)
	if err != nil {
		return fmt.Errorf("error searching: %w", err)
	}

	return encode(list)
}

func cmdQueryEntities(c *cli.Context) error {
	return cmdQueryList(c, entityLikeQueryFlag, data.QueryEntities)
}

func cmdQueryDeveloper(c *cli.Context) error {
	applyFlags(c)
	val := c.String(ghUserNameQueryFlag.Name)
	token, err := getGitHubToken()
	if err != nil {
		return fmt.Errorf("failed to get GitHub token: %w", err)
	}

	if val == "" || token == "" {
		return cli.ShowSubcommandHelp(c)
	}

	cfg := getConfig(c)

	dev, err := data.GetDeveloper(cfg.DB, val)
	if err != nil {
		return fmt.Errorf("failed to query developer: %w", err)
	}
	if dev == nil {
		return fmt.Errorf("developer not found: %s", val)
	}

	ctx := context.Background()
	client := net.GetOAuthClient(ctx, token)
	dev.Organizations, err = data.GetUserOrgs(ctx, client, val, 0)
	if err != nil {
		slog.Warn("failed to get user orgs", "error", err)
	}

	if err := encode(dev); err != nil {
		return fmt.Errorf("error encoding: %+v: %w", dev, err)
	}

	return nil
}

func cmdQueryDevelopers(c *cli.Context) error {
	return cmdQueryList(c, developerLikeQueryFlag, data.SearchDevelopers)
}

func cmdQueryOrgRepos(c *cli.Context) error {
	applyFlags(c)
	org := c.String(orgNameFlag.Name)
	token, err := getGitHubToken()
	if err != nil {
		return fmt.Errorf("failed to get GitHub token: %w", err)
	}

	if org == "" || token == "" {
		return cli.ShowSubcommandHelp(c)
	}

	ctx := context.Background()
	client := net.GetOAuthClient(ctx, token)
	list, err := data.GetOrgRepos(ctx, client, org)
	if err != nil {
		return fmt.Errorf("failed to list org repos: %w", err)
	}

	if err := encode(list); err != nil {
		return fmt.Errorf("error encoding: %w", err)
	}

	return nil
}
