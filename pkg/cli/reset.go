package cli

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/urfave/cli/v3"
)

var resetCmd = &cli.Command{
	Name:            "reset",
	Usage:           "Delete all imported data and start fresh",
	HideHelpCommand: true,
	Flags:           []cli.Flag{dbFilePathFlag, debugFlag, logJSONFlag, forceFlag},
	Action:          cmdReset,
}

func cmdReset(_ context.Context, cmd *cli.Command) error {
	applyFlags(cmd)
	cfg := getConfig(cmd)

	if !cmd.Bool(forceFlag.Name) {
		fmt.Printf("This will permanently delete all data in %s\n", cfg.DSN)
		confirmed, err := confirmAction("Are you sure? [y/N]: ")
		if err != nil {
			return err
		}
		if !confirmed {
			fmt.Println("Aborted.")
			return nil
		}
	}

	if strings.HasPrefix(cfg.DSN, "postgres://") || strings.HasPrefix(cfg.DSN, "postgresql://") {
		return errors.New("reset is not supported for PostgreSQL databases, use psql to drop/recreate")
	}

	// close the store before deleting the file
	if cfg.Store != nil {
		cfg.Store.Close()
		cfg.Store = nil
	}

	if err := os.Remove(cfg.DSN); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("deleting database: %w", err)
	}

	slog.Info("database deleted", "path", cfg.DSN)

	// re-initialize empty database via openStore
	store, err := openStore(cfg.DSN)
	if err != nil {
		return fmt.Errorf("re-initializing store: %w", err)
	}
	cfg.Store = store

	slog.Info("database re-initialized", "path", cfg.DSN)
	fmt.Println("Reset complete.")
	return nil
}
