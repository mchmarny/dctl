package main

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
	"github.com/urfave/cli/v2"
)

const (
	dirMode      = 0700
	appConfigKey = "app-config"
)

var (
	name    = "dctl"
	version = "v0.0.1-default"
	commit  = ""
	date    = ""

	debugFlag = &cli.BoolFlag{
		Name:  "debug",
		Usage: "Prints verbose logs (optional, default: false)",
	}

	dbFilePathFlag = &cli.StringFlag{
		Name:  "db",
		Usage: "Path to the Sqlite database file",
	}
)

type appConfig struct {
	DBPath string
	Debug  bool
	DB     *sql.DB
}

func getConfig(c *cli.Context) *appConfig {
	return c.App.Metadata[appConfigKey].(*appConfig)
}

func main() {
	initLogging(false)

	app := &cli.App{
		Name:     "dctl",
		Version:  fmt.Sprintf("%s (%s - %s)", version, commit, date),
		Compiled: time.Now(),
		Usage:    "CLI for quick insight into the GitHub org/repo activity",
		Flags: []cli.Flag{
			debugFlag,
			dbFilePathFlag,
		},
		Commands: []*cli.Command{
			authCmd,
			importCmd,
			queryCmd,
			serverCmd,
		},
		Before: func(c *cli.Context) error {
			if c.Bool(debugFlag.Name) {
				initLogging(true)
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
		After: func(c *cli.Context) error {
			if cfg, ok := c.App.Metadata[appConfigKey].(*appConfig); ok && cfg.DB != nil {
				cfg.DB.Close()
			}
			return nil
		},
	}

	if err := app.Run(os.Args); err != nil {
		slog.Error("fatal error", "error", err)
		os.Exit(1)
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

	dirName := "." + name
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

func getEncoder() *json.Encoder {
	e := json.NewEncoder(os.Stdout)
	e.SetIndent("", "  ")
	return e
}
