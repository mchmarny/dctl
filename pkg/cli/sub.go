package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/mchmarny/devpulse/pkg/data"
	"github.com/urfave/cli/v3"
)

var (
	subTypeFlag = &cli.StringFlag{
		Name:    "type",
		Usage:   fmt.Sprintf("Substitution type [%s]", strings.Join(data.UpdatableProperties, ",")),
		Sources: cli.EnvVars("DEVPULSE_TYPE"),
	}

	oldValFlag = &cli.StringFlag{
		Name:     "old",
		Usage:    "Old value",
		Required: true,
		Sources:  cli.EnvVars("DEVPULSE_OLD"),
	}

	newValFlag = &cli.StringFlag{
		Name:     "new",
		Usage:    "New value",
		Required: true,
		Sources:  cli.EnvVars("DEVPULSE_NEW"),
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
			debugFlag,
		},
	}
)

func cmdSubstitutes(_ context.Context, cmd *cli.Command) error {
	applyFlags(cmd)
	sub := cmd.String(subTypeFlag.Name)
	old := cmd.String(oldValFlag.Name)
	new := cmd.String(newValFlag.Name)

	if sub == "" || old == "" || new == "" {
		return cli.ShowSubcommandHelp(cmd)
	}

	cfg := getConfig(cmd)

	res, err := cfg.Store.SaveAndApplyDeveloperSub(sub, old, new)
	if err != nil {
		return fmt.Errorf("failed to apply substitution: %w", err)
	}

	if err := encode(res); err != nil {
		return fmt.Errorf("error encoding result: %w", err)
	}

	return nil
}
