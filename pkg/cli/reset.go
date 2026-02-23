package cli

import (
	"bufio"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/mchmarny/devpulse/pkg/data"
	"github.com/urfave/cli/v2"
)

var resetCmd = &cli.Command{
	Name:            "reset",
	Usage:           "Delete all imported data and start fresh",
	HideHelpCommand: true,
	Flags:           []cli.Flag{debugFlag},
	Action:          cmdReset,
}

func cmdReset(c *cli.Context) error {
	applyFlags(c)
	cfg := getConfig(c)

	fmt.Printf("This will permanently delete all data in %s\n", cfg.DBPath)
	fmt.Print("Are you sure? [y/N]: ")

	reader := bufio.NewReader(os.Stdin)
	answer, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("reading input: %w", err)
	}

	if strings.ToLower(strings.TrimSpace(answer)) != "y" {
		fmt.Println("Aborted.")
		return nil
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
