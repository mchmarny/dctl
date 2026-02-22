package cli

import (
	"fmt"
	"strings"

	"github.com/mchmarny/dctl/pkg/data"
	"github.com/urfave/cli/v2"
)

var (
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

	substituteCmd = &cli.Command{
		Name:    "substitute",
		Aliases: []string{"sub"},
		Usage:   "Create a global data substitution (e.g. standardize entity name)",
		UsageText: `dctl substitute --type entity --old "Old Corp" --new "NEW CORP"   # rename entity
   dctl sub --type entity --old "ACME INC" --new "ACME"              # standardize name`,
		HideHelpCommand: true,
		Action:          cmdSubstitutes,
		Flags: []cli.Flag{
			subTypeFlag,
			oldValFlag,
			newValFlag,
		},
	}
)

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
		return fmt.Errorf("failed to apply substitution: %w", err)
	}

	if err := getEncoder().Encode(res); err != nil {
		return fmt.Errorf("error encoding result: %w", err)
	}

	return nil
}
