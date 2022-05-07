package main

import (
	"os"
	"testing"

	"github.com/mchmarny/cli/pkg/config"
	"github.com/mchmarny/cli/pkg/data"
	"github.com/rs/zerolog/log"
)

const (
	testDir = "../../tmp"
)

func TestMain(m *testing.M) {
	os.RemoveAll(testDir)
	initLogging(name, version)

	cfg, err := config.ReadOrCreate(testDir)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to read or create config")
	}
	log.Debug().Msgf("config: %+v", cfg)

	if err = data.Init(testDir); err != nil {
		fatalErr(err)
	}
	defer data.Close()

	code := m.Run()
	os.Exit(code)
}
