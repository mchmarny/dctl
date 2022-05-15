package main

import (
	"database/sql"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"time"

	"github.com/mchmarny/dctl/pkg/data"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
)

const (
	dirMode = 0700

	gitHubAccessTokenEnvVar = "GITHUB_ACCESS_TOKEN"
)

var (
	name    = "dctl"
	version = "v0.0.1-default"
	commit  = ""

	dbFilePath = path.Join(getHomeDir(), data.DataFileName)
	debug      = false

	debugFlag = &cli.BoolFlag{
		Name:        "debug",
		Usage:       "Prints verbose logs (optional, default: false)",
		Destination: &debug,
	}

	dbFilePathFlag = &cli.StringFlag{
		Name:        "db",
		Usage:       fmt.Sprintf("Path to the Sqlite database file (optional, defaults to $HOME/.%s/data.db)", name),
		Destination: &dbFilePath,
		Value:       dbFilePath,
	}
)

func main() {
	initLogging(name, version)

	var err error
	if err = data.Init(dbFilePath); err != nil {
		fatalErr(err)
	}

	app := &cli.App{
		Name:     "dctl",
		Version:  fmt.Sprintf("%s - (commit: %s)", version, commit),
		Compiled: time.Now(),
		Usage:    "CLI for developer data",
		Flags: []cli.Flag{
			debugFlag,
			dbFilePathFlag,
		},
		Commands: []*cli.Command{
			importAffiliationCmd,
			updateCmd,
			developerQueryCmd,
			serverCmd,
		},
		Before: func(c *cli.Context) error {
			if c.Bool(debugFlag.Name) {
				log.SetLevel(log.DebugLevel)
				// log.SetReportCaller(true)
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
		log.Fatalf("fatal error: %v", err)
		os.Exit(1)
	}
}

func getDBOrFail() *sql.DB {
	db, err := data.GetDB(dbFilePath)
	if err != nil {
		log.Fatalf("fatal error creating DB: %v", err)
		os.Exit(1)
	}
	return db
}

func initLogging(name, version string) {
	log.SetOutput(os.Stdout)
	log.SetLevel(log.InfoLevel)
	log.SetReportCaller(false)
	log.SetFormatter(&log.TextFormatter{
		FullTimestamp:          false,
		DisableTimestamp:       true,
		ForceColors:            true,
		DisableLevelTruncation: true,
		PadLevelText:           true,
	})
}

func getHomeDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		log.Debugf("error getting home dir, using current dir instead: %v", err)
		return "."
	}
	log.Debugf("home dir: %s", home)

	dirName := "." + name
	dirPath := filepath.Join(home, dirName)
	if _, err := os.Stat(dirPath); errors.Is(err, os.ErrNotExist) {
		log.Debugf("creating dir: %s", dirPath)
		err := os.Mkdir(dirPath, dirMode)
		if err != nil {
			log.Debugf("error creating dir: %s, using home: %s - %v", dirPath, home, err)
			return home
		}
	}
	return dirPath
}
