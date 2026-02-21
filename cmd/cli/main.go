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
	dirMode = 0700
)

var (
	name    = "dctl"
	version = "v0.0.1-default"
	commit  = ""
	date    = ""

	dbFilePath = path.Join(getHomeDir(), data.DataFileName)
	debug      = false

	debugFlag = &cli.BoolFlag{
		Name:        "debug",
		Usage:       "Prints verbose logs (optional, default: false)",
		Destination: &debug,
	}

	dbFilePathFlag = &cli.StringFlag{
		Name:        "db",
		Usage:       "Path to the Sqlite database file",
		Destination: &dbFilePath,
		Value:       dbFilePath,
	}
)

func main() {
	initLogging(false)

	var err error
	if err = data.Init(dbFilePath); err != nil {
		fatalErr(err)
	}

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

			path := c.String(dbFilePathFlag.Name)
			if path != "" {
				dbFilePath = path
			}
			return nil
		},
	}

	err = app.Run(os.Args)
	if err != nil {
		fatalErr(err)
	}
}

func fatalErr(err error) {
	if err != nil {
		slog.Error("fatal error", "error", err)
		os.Exit(1)
	}
}

func getDBOrFail() *sql.DB {
	db, err := data.GetDB(dbFilePath)
	if err != nil {
		slog.Error("fatal error creating DB", "error", err)
		os.Exit(1)
	}
	return db
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
