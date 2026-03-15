package cli

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/mchmarny/devpulse/pkg/data"
	"github.com/urfave/cli/v3"
)

var resetCmd = &cli.Command{
	Name:            "reset",
	Usage:           "Delete all imported data and start fresh",
	HideHelpCommand: true,
	Flags:           []cli.Flag{debugFlag, forceFlag},
	Action:          cmdReset,
}

func cmdReset(_ context.Context, cmd *cli.Command) error {
	applyFlags(cmd)
	cfg := getConfig(cmd)

	if !cmd.Bool(forceFlag.Name) {
		fmt.Printf("This will permanently delete all data in %s\n", cfg.DBPath)
		confirmed, err := confirmAction("Are you sure? [y/N]: ")
		if err != nil {
			return err
		}
		if !confirmed {
			fmt.Println("Aborted.")
			return nil
		}
	}

	// close the DB before deleting the file
	if cfg.DB != nil {
		cfg.DB.Close()
		cfg.DB = nil
	}

	if err := os.Remove(cfg.DBPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("deleting database: %w", err)
	}

	slog.Info("database deleted", "path", cfg.DBPath)

	// re-initialize empty database
	if err := data.Init(cfg.DBPath); err != nil {
		return fmt.Errorf("re-initializing database: %w", err)
	}

	slog.Info("database re-initialized", "path", cfg.DBPath)
	fmt.Println("Reset complete.")
	return nil
}
