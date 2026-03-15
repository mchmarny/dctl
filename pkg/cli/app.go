package cli

import (
	"bufio"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/mchmarny/devpulse/pkg/data"
	"github.com/mchmarny/devpulse/pkg/logging"
	urfave "github.com/urfave/cli/v3"
	"gopkg.in/yaml.v3"
)

const (
	dirMode      = 0700
	appConfigKey = "app-config"

	formatJSON = "json"
	formatYAML = "yaml"
)

var (
	version = "v0.0.1-default"
	commit  = ""
	date    = ""

	outputFormat = formatJSON

	debugFlag = &urfave.BoolFlag{
		Name:    "debug",
		Usage:   "Prints verbose logs (optional, default: false)",
		Sources: urfave.EnvVars("DEVPULSE_DEBUG"),
	}

	dbFilePathFlag = &urfave.StringFlag{
		Name:    "db",
		Usage:   "Path to the Sqlite database file",
		Value:   filepath.Join(getHomeDir(), data.DataFileName),
		Sources: urfave.EnvVars("DEVPULSE_DB"),
	}

	formatFlag = &urfave.StringFlag{
		Name:    "format",
		Usage:   "Output format [json, yaml]",
		Value:   formatJSON,
		Sources: urfave.EnvVars("DEVPULSE_FORMAT"),
	}

	forceFlag = &urfave.BoolFlag{
		Name:    "force",
		Usage:   "Skip confirmation prompt",
		Sources: urfave.EnvVars("DEVPULSE_FORCE"),
	}
)

// Execute creates and runs the CLI application.
func Execute() {
	initLogging(false)

	cmd := newApp()
	if err := cmd.Run(context.Background(), os.Args); err != nil {
		slog.Error("fatal error", "error", err)
		os.Exit(1)
	}
}

type appConfig struct {
	DBPath string
	Debug  bool
	DB     *sql.DB
}

func getConfig(cmd *urfave.Command) *appConfig {
	return cmd.Root().Metadata[appConfigKey].(*appConfig)
}

func newApp() *urfave.Command {
	return &urfave.Command{
		Name:                  "devpulse",
		Version:               fmt.Sprintf("%s (%s - %s)", version, commit, date),
		EnableShellCompletion: true,
		HideHelpCommand:       true,
		Usage:                 "CLI for quick insight into the GitHub org/repo activity",
		Flags: []urfave.Flag{
			debugFlag,
			dbFilePathFlag,
			formatFlag,
		},
		Commands: []*urfave.Command{
			authCmd,
			importCmd,
			deleteCmd,
			scoreCmd,
			substituteCmd,
			queryCmd,
			serverCmd,
			resetCmd,
		},
		Before: func(ctx context.Context, cmd *urfave.Command) (context.Context, error) {
			applyFlags(cmd)

			dbPath := cmd.String(dbFilePathFlag.Name)

			if err := data.Init(dbPath); err != nil {
				return ctx, fmt.Errorf("initializing database: %w", err)
			}

			db, err := data.GetDB(dbPath)
			if err != nil {
				return ctx, fmt.Errorf("opening database: %w", err)
			}

			cmd.Root().Metadata[appConfigKey] = &appConfig{
				DBPath: dbPath,
				Debug:  cmd.Bool(debugFlag.Name),
				DB:     db,
			}
			return ctx, nil
		},
		After: func(ctx context.Context, cmd *urfave.Command) error {
			if cfg, ok := cmd.Root().Metadata[appConfigKey].(*appConfig); ok && cfg.DB != nil {
				if err := cfg.DB.Close(); err != nil {
					slog.Error("error closing database", "error", err)
				}
			}
			return nil
		},
		Metadata: map[string]any{},
	}
}

func initLogging(debug bool) {
	level := "info"
	if debug {
		level = "debug"
	}
	logging.SetDefaultCLILogger(level)
}

func getHomeDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		slog.Debug("error getting home dir, using current dir instead", "error", err)
		return "."
	}
	slog.Debug("home dir", "path", home)

	dirName := ".devpulse"
	dirPath := filepath.Join(home, dirName)
	if _, err := os.Stat(dirPath); errors.Is(err, os.ErrNotExist) {
		slog.Debug("creating dir", "path", dirPath)
		err := os.Mkdir(dirPath, dirMode)
		if err != nil {
			slog.Debug("error creating dir", "path", dirPath, "home", home, "error", err)
			return home
		}
	}
	return dirPath
}

func applyFlags(cmd *urfave.Command) {
	if cmd.Bool(debugFlag.Name) {
		initLogging(true)
	}
	f := cmd.String(formatFlag.Name)
	if f == formatYAML || f == "yml" {
		outputFormat = formatYAML
	}
}

func confirmAction(prompt string) (bool, error) {
	fmt.Print(prompt)
	reader := bufio.NewReader(os.Stdin)
	response, err := reader.ReadString('\n')
	if err != nil {
		return false, fmt.Errorf("error reading input: %w", err)
	}
	response = strings.TrimSpace(strings.ToLower(response))
	return response == "y" || response == "yes", nil
}

func encode(v any) error {
	if outputFormat == formatYAML {
		return yaml.NewEncoder(os.Stdout).Encode(v)
	}
	e := json.NewEncoder(os.Stdout)
	e.SetIndent("", "  ")
	return e.Encode(v)
}
