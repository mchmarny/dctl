package cli

import (
	"fmt"
	"strings"

	"github.com/mchmarny/devpulse/pkg/data"
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
		UsageText: `devpulse substitute --type entity --old "Old Corp" --new "NEW CORP"   # rename entity
   devpulse sub --type entity --old "ACME INC" --new "ACME"              # standardize name`,
		HideHelpCommand: true,
		Action:          cmdSubstitutes,
		Flags: []cli.Flag{
			subTypeFlag,
			oldValFlag,
			newValFlag,
			formatFlag,
		},
	}
)

func cmdSubstitutes(c *cli.Context) error {
	applyFlags(c)
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

	if err := encode(res); err != nil {
		return fmt.Errorf("error encoding result: %w", err)
	}

	return nil
}
