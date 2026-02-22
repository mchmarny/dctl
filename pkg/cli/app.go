package cli

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path"
	"path/filepath"
	"time"

	"github.com/mchmarny/dctl/pkg/data"
	urfave "github.com/urfave/cli/v2"
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
		Name:  "debug",
		Usage: "Prints verbose logs (optional, default: false)",
	}

	dbFilePathFlag = &urfave.StringFlag{
		Name:  "db",
		Usage: "Path to the Sqlite database file",
	}

	formatFlag = &urfave.StringFlag{
		Name:  "format",
		Usage: "Output format [json, yaml]",
		Value: formatJSON,
	}
)

// Execute creates and runs the CLI application.
func Execute() {
	initLogging(false)

	app := newApp()
	if err := app.Run(os.Args); err != nil {
		slog.Error("fatal error", "error", err)
		os.Exit(1)
	}
}

type appConfig struct {
	DBPath string
	Debug  bool
	DB     *sql.DB
}

func getConfig(c *urfave.Context) *appConfig {
	return c.App.Metadata[appConfigKey].(*appConfig)
}

func newApp() *urfave.App {
	return &urfave.App{
		Name:                 "dctl",
		Version:              fmt.Sprintf("%s (%s - %s)", version, commit, date),
		Compiled:             time.Now(),
		EnableBashCompletion: true,
		HideHelpCommand:      true,
		Usage:                "CLI for quick insight into the GitHub org/repo activity",
		Flags: []urfave.Flag{
			debugFlag,
			dbFilePathFlag,
			formatFlag,
		},
		Commands: []*urfave.Command{
			authCmd,
			importCmd,
			substituteCmd,
			queryCmd,
			serverCmd,
		},
		Before: func(c *urfave.Context) error {
			if c.Bool(debugFlag.Name) {
				initLogging(true)
			}

			f := c.String(formatFlag.Name)
			if f == formatYAML || f == "yml" {
				outputFormat = formatYAML
			}

			dbPath := c.String(dbFilePathFlag.Name)
			if dbPath == "" {
				dbPath = path.Join(getHomeDir(), data.DataFileName)
			}

			if err := data.Init(dbPath); err != nil {
				return fmt.Errorf("initializing database: %w", err)
			}

			db, err := data.GetDB(dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}

			c.App.Metadata[appConfigKey] = &appConfig{
				DBPath: dbPath,
				Debug:  c.Bool(debugFlag.Name),
				DB:     db,
			}
			return nil
		},
		After: func(c *urfave.Context) error {
			if cfg, ok := c.App.Metadata[appConfigKey].(*appConfig); ok && cfg.DB != nil {
				cfg.DB.Close()
			}
			return nil
		},
	}
}

func initLogging(debug bool) {
	level := slog.LevelInfo
	if debug {
		level = slog.LevelDebug
	}
	h := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: level})
	slog.SetDefault(slog.New(h))
}

func getHomeDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		slog.Debug("error getting home dir, using current dir instead", "error", err)
		return "."
	}
	slog.Debug("home dir", "path", home)

	dirName := ".dctl"
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

func encode(v any) error {
	if outputFormat == formatYAML {
		return yaml.NewEncoder(os.Stdout).Encode(v)
	}
	e := json.NewEncoder(os.Stdout)
	e.SetIndent("", "  ")
	return e.Encode(v)
}
