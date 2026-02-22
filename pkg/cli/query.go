package cli

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/mchmarny/dctl/pkg/data"
	"github.com/mchmarny/dctl/pkg/net"
	"github.com/urfave/cli/v2"
)

const (
	queryResultLimitDefault = 500
)

var (
	queryLimitFlag = &cli.IntFlag{
		Name:     "limit",
		Usage:    "Limits number of result returned",
		Value:    queryResultLimitDefault,
		Required: false,
	}

	developerLikeQueryFlag = &cli.StringFlag{
		Name:     "like",
		Usage:    "Fuzzy search developers, identities, or entities",
		Required: true,
	}

	ghUserNameQueryFlag = &cli.StringFlag{
		Name:     "name",
		Usage:    "GitHub username",
		Required: false,
	}

	entityLikeQueryFlag = &cli.StringFlag{
		Name:     "like",
		Usage:    "Fuzzy entities search",
		Required: true,
	}

	entityNameQueryFlag = &cli.StringFlag{
		Name:     "name",
		Usage:    "Entity name",
		Required: true,
	}

	eventSinceFlag = &cli.StringFlag{
		Name:     "since",
		Usage:    "Event since date (YYYY-MM-DD)",
		Required: false,
	}

	eventAuthorFlag = &cli.StringFlag{
		Name:     "author",
		Usage:    "Event author (GitHub username)",
		Required: false,
	}

	eventEntityFlag = &cli.StringFlag{
		Name:     "entity",
		Usage:    "Event entity (company name or affiliated organization)",
		Required: false,
	}

	eventTypeFlag = &cli.StringFlag{
		Name:     "type",
		Usage:    fmt.Sprintf("Event type (%s)", strings.Join(data.EventTypes, ", ")),
		Required: false,
	}

	eventMentionFlag = &cli.StringFlag{
		Name:     "mention",
		Usage:    "GitHub mention (like query on @username in body or assignments)",
		Required: false,
	}

	eventLabelFlag = &cli.StringFlag{
		Name:     "label",
		Usage:    "GitHub label (like query on issues and PRs)",
		Required: false,
	}

	queryCmd = &cli.Command{
		Name:    "query",
		Aliases: []string{"q"},
		Usage:   "List data query operations",
		Subcommands: []*cli.Command{
			{
				Name:    "developer",
				Usage:   "List developer operations",
				Aliases: []string{"d"},
				Subcommands: []*cli.Command{
					{
						Name:    "list",
						Usage:   "List developers",
						Aliases: []string{"l"},
						Action:  cmdQueryDevelopers,
						Flags: []cli.Flag{
							developerLikeQueryFlag,
							queryLimitFlag,
						},
					},
					{
						Name:    "detail",
						Usage:   "Get specific developer details, identities and associated entities",
						Aliases: []string{"d"},
						Action:  cmdQueryDeveloper,
						Flags: []cli.Flag{
							ghUserNameQueryFlag,
						},
					},
				},
			},
			{
				Name:    "entity",
				Usage:   "List entity operations",
				Aliases: []string{"c"},
				Subcommands: []*cli.Command{
					{
						Name:    "list",
						Usage:   "List entities (companies or organizations with which users are affiliated)",
						Aliases: []string{"l"},
						Action:  cmdQueryEntities,
						Flags: []cli.Flag{
							entityLikeQueryFlag,
							queryLimitFlag,
						},
					},
					{
						Name:    "detail",
						Usage:   "Get specific entity and its associated developers",
						Aliases: []string{"d"},
						Action:  cmdQueryEntity,
						Flags: []cli.Flag{
							entityNameQueryFlag,
						},
					},
				},
			},
			{
				Name:    "org",
				Usage:   "List GitHub org/user operations",
				Aliases: []string{"o"},
				Subcommands: []*cli.Command{
					{
						Name:   "repos",
						Usage:  "List GitHub org/user repositories",
						Action: cmdQueryOrgRepos,
						Flags: []cli.Flag{
							orgNameFlag,
						},
					},
				},
			},
			{
				Name:    "events",
				Usage:   "List GitHub events",
				Aliases: []string{"e"},
				Action:  cmdQueryEvents,
				Flags: []cli.Flag{
					orgNameFlag,
					repoNameFlag,
					eventTypeFlag,
					eventSinceFlag,
					eventAuthorFlag,
					eventEntityFlag,
					eventMentionFlag,
					eventLabelFlag,
					queryLimitFlag,
				},
			},
		},
	}
)

func optional(val string) *string {
	if val == "" || val == "undefined" {
		return nil
	}
	return &val
}

func cmdQueryEntity(c *cli.Context) error {
	val := c.String(entityNameQueryFlag.Name)
	if val == "" {
		return cli.ShowSubcommandHelp(c)
	}

	cfg := getConfig(c)

	ent, err := data.GetEntity(cfg.DB, val)
	if err != nil {
		return fmt.Errorf("failed to query entity: %w", err)
	}

	if err := getEncoder().Encode(ent); err != nil {
		return fmt.Errorf("error encoding: %+v: %w", ent, err)
	}

	return nil
}

func cmdQueryEvents(c *cli.Context) error {
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

	slog.Debug("query events",
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
		Username: optional(author),
		Entity:   optional(entity),
		Type:     optional(etype),
		FromDate: optional(since),
		Mention:  optional(mention),
		Label:    optional(label),
		PageSize: limit,
	}

	cfg := getConfig(c)

	list, err := data.SearchEvents(cfg.DB, q)
	if err != nil {
		return fmt.Errorf("failed to query events: %w", err)
	}

	if err := getEncoder().Encode(list); err != nil {
		return fmt.Errorf("error encoding list: %+v: %w", list, err)
	}

	return nil
}

func cmdQueryList[T any](c *cli.Context, flag *cli.StringFlag, fn func(*sql.DB, string, int) ([]*T, error)) error {
	val := c.String(flag.Name)
	if val == "" {
		return cli.ShowSubcommandHelp(c)
	}

	limit := c.Int(queryLimitFlag.Name)
	if limit == 0 || limit > queryResultLimitDefault {
		limit = queryResultLimitDefault
	}

	cfg := getConfig(c)

	list, err := fn(cfg.DB, val, limit)
	if err != nil {
		return fmt.Errorf("failed to query %s: %w", flag.Name, err)
	}

	return getEncoder().Encode(list)
}

func cmdQueryEntities(c *cli.Context) error {
	return cmdQueryList(c, entityLikeQueryFlag, data.QueryEntities)
}

func cmdQueryDeveloper(c *cli.Context) error {
	val := c.String(ghUserNameQueryFlag.Name)
	token, err := getGitHubToken()
	if err != nil {
		return fmt.Errorf("failed to get GitHub token: %w", err)
	}

	if val == "" || token == "" {
		return cli.ShowSubcommandHelp(c)
	}

	cfg := getConfig(c)

	slog.Debug("query developer data", "name", val)
	dev, err := data.GetDeveloper(cfg.DB, val)
	if err != nil {
		return fmt.Errorf("failed to query developer: %w", err)
	}

	if dev == nil || dev.Username == "" {
		fmt.Fprint(os.Stdout, "{}")
		return nil
	}

	ctx := context.Background()
	client := net.GetOAuthClient(ctx, token)

	slog.Debug("query developer gh organizations", "name", dev.Username)
	dev.Organizations, err = data.GetUserOrgs(ctx, client, dev.Username, queryResultLimitDefault)
	if err != nil {
		return fmt.Errorf("failed to query orgs: %w", err)
	}

	if err := getEncoder().Encode(dev); err != nil {
		return fmt.Errorf("error encoding: %+v: %w", dev, err)
	}

	return nil
}

func cmdQueryDevelopers(c *cli.Context) error {
	return cmdQueryList(c, developerLikeQueryFlag, data.SearchDevelopers)
}

func cmdQueryOrgRepos(c *cli.Context) error {
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
		return fmt.Errorf("failed to query repos: %w", err)
	}

	if err := getEncoder().Encode(list); err != nil {
		return fmt.Errorf("error encoding: %+v: %w", list, err)
	}

	return nil
}
