package main

import (
	"os"
	"testing"

	"github.com/mchmarny/dctl/pkg/data"
)

const (
	testDir = "../../tmp"
)

func TestMain(m *testing.M) {
	os.RemoveAll(testDir)
	initLogging(name, version)

	if err := data.Init(testDir); err != nil {
		fatalErr(err)
	}

	code := m.Run()
	os.Exit(code)
}
