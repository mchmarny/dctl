package main

import (
	"context"
	"fmt"
	"os"

	"github.com/mchmarny/dctl/pkg/data"
	"github.com/mchmarny/dctl/pkg/net"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
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

	eventTypeFlag = &cli.StringFlag{
		Name: "type",
		Usage: fmt.Sprintf("Event type (%s, %s, %s, %s)",
			data.EventTypePR, data.EventTypeIssue, data.EventTypePRComment, data.EventTypeIssueComment),
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
					eventMentionFlag,
					eventLabelFlag,
					queryLimitFlag,
				},
			},
		},
	}
)

func writeEmpty(c *cli.Context) error {
	_, err := os.Stdout.Write([]byte("{}"))
	return err
}

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

	db := getDBOrFail()
	defer db.Close()

	ent, err := data.GetEntity(db, val)
	if err != nil {
		return errors.Wrap(err, "failed to query entity")
	}

	if err := getEncoder().Encode(ent); err != nil {
		return errors.Wrapf(err, "error encoding: %+v", ent)
	}

	return nil
}

func cmdQueryEvents(c *cli.Context) error {
	org := c.String(orgNameFlag.Name)
	repo := c.String(repoNameFlag.Name)
	author := c.String(eventAuthorFlag.Name)
	since := c.String(eventSinceFlag.Name)
	etype := c.String(eventTypeFlag.Name)
	mention := c.String(eventMentionFlag.Name)
	label := c.String(eventLabelFlag.Name)

	limit := c.Int(queryLimitFlag.Name)
	if limit == 0 || limit > queryResultLimitDefault {
		limit = queryResultLimitDefault
	}

	log.WithFields(log.Fields{
		"org":     org,
		"repo":    repo,
		"author":  author,
		"since":   since,
		"type":    etype,
		"limit":   limit,
		"mention": mention,
		"label":   label,
	}).Debug("query events")

	q := &data.EventSearchCriteria{
		Org:       optional(org),
		Repo:      optional(repo),
		Username:  optional(author),
		EventType: optional(etype),
		FromDate:  optional(since),
		Mention:   optional(mention),
		Label:     optional(label),
		PageSize:  limit,
	}

	db := getDBOrFail()
	defer db.Close()

	list, err := data.SearchEvents(db, q)
	if err != nil {
		return errors.Wrap(err, "failed to query events")
	}

	if err := getEncoder().Encode(list); err != nil {
		return errors.Wrapf(err, "error encoding list: %+v", list)
	}

	return nil
}

func cmdQueryEntities(c *cli.Context) error {
	val := c.String(entityLikeQueryFlag.Name)
	if val == "" {
		return cli.ShowSubcommandHelp(c)
	}

	limit := c.Int(queryLimitFlag.Name)
	if limit == 0 || limit > queryResultLimitDefault {
		limit = queryResultLimitDefault
	}

	log.WithFields(log.Fields{
		"val":   val,
		"limit": limit,
	}).Debug("query developers")

	db := getDBOrFail()
	defer db.Close()

	list, err := data.QueryEntities(db, val, limit)
	if err != nil {
		return errors.Wrap(err, "failed to query entities")
	}

	if err := getEncoder().Encode(list); err != nil {
		return errors.Wrapf(err, "error encoding list: %+v", list)
	}

	return nil
}

func cmdQueryDeveloper(c *cli.Context) error {
	val := c.String(ghUserNameQueryFlag.Name)
	token, err := getGitHubToken()
	if err != nil {
		return errors.Wrap(err, "failed to get GitHub token")
	}

	if val == "" || token == "" {
		return cli.ShowSubcommandHelp(c)
	}

	db := getDBOrFail()
	defer db.Close()

	log.WithFields(log.Fields{"name": val}).Debug("query developer data...")
	dev, err := data.GetDeveloper(db, val)
	if err != nil {
		return errors.Wrap(err, "failed to query developer")
	}

	if dev == nil || dev.Username == "" {
		return writeEmpty(c)
	}

	ctx := context.Background()
	client := net.GetOAuthClient(ctx, token)

	log.WithFields(log.Fields{"name": dev.Username}).Debug("query developer gh organizations...")
	dev.Organizations, err = data.GetUserOrgs(ctx, client, dev.Username, queryResultLimitDefault)
	if err != nil {
		return errors.Wrap(err, "failed to query orgs")
	}

	if err := getEncoder().Encode(dev); err != nil {
		return errors.Wrapf(err, "error encoding: %+v", dev)
	}

	return nil
}

func cmdQueryDevelopers(c *cli.Context) error {
	val := c.String(developerLikeQueryFlag.Name)
	if val == "" {
		return cli.ShowSubcommandHelp(c)
	}

	limit := c.Int(queryLimitFlag.Name)
	if limit == 0 || limit > queryResultLimitDefault {
		limit = queryResultLimitDefault
	}

	log.WithFields(log.Fields{
		"val":   val,
		"limit": limit,
	}).Debug("query developer")

	db := getDBOrFail()
	defer db.Close()

	list, err := data.SearchDevelopers(db, val, limit)
	if err != nil {
		return errors.Wrap(err, "error quering CNCF developer")
	}

	if err := getEncoder().Encode(list); err != nil {
		return errors.Wrapf(err, "error encoding: %+v", list)
	}

	return nil
}

func cmdQueryOrgRepos(c *cli.Context) error {
	org := c.String(orgNameFlag.Name)
	token, err := getGitHubToken()
	if err != nil {
		return errors.Wrap(err, "failed to get GitHub token")
	}

	if org == "" || token == "" {
		return cli.ShowSubcommandHelp(c)
	}

	ctx := context.Background()
	client := net.GetOAuthClient(ctx, token)
	list, err := data.GetOrgRepos(ctx, client, org)
	if err != nil {
		return errors.Wrap(err, "failed to query repos")
	}

	if err := getEncoder().Encode(list); err != nil {
		return errors.Wrapf(err, "error encoding: %+v", list)
	}

	return nil
}
