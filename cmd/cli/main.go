package main

import (
	"fmt"
	"os"
	"time"

	"github.com/mchmarny/cli/pkg/config"
	"github.com/mchmarny/cli/pkg/data"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/urfave/cli"
)

const (
	name = "cli"
)

var (
	version = "v0.0.1-default"

	cfg *config.Config
)

func main() {
	initLogging(name, version)

	homeDir, created, err := config.GetOrCreateHomeDir(name)
	fatalErr(err)
	log.Debug().Msgf("home dir (created: %v): %s", created, homeDir)

	cfg, err = config.ReadOrCreate(homeDir)
	fatalErr(err)

	if err = data.Init(homeDir); err != nil {
		fatalErr(err)
	}
	defer data.Close()

	app := &cli.App{
		Name:     "twee",
		Version:  fmt.Sprintf("%s - %s", version, cfg.Value),
		Compiled: time.Now(),
		Usage:    "cli",
		Commands: []cli.Command{
			simpleCmd,
		},
	}

	err = app.Run(os.Args)
	if err != nil {
		fmt.Print(err)
	}
}

func fatalErr(err error) {
	if err != nil {
		log.Fatal().Err(err).Msg("fatal error")
	}
}

func initLogging(name, version string) {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	zerolog.TimestampFieldName = "ts"
	zerolog.LevelFieldName = "level"
	zerolog.MessageFieldName = "msg"
	zerolog.ErrorFieldName = "err"
	zerolog.CallerFieldName = "caller"
	zerolog.SetGlobalLevel(zerolog.InfoLevel)
}
