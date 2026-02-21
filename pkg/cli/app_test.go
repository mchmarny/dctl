package cli

import (
	"log/slog"
	"os"
	"testing"

	"github.com/mchmarny/dctl/pkg/data"
)

const (
	testDir = "../../tmp"
)

func TestMain(m *testing.M) {
	os.RemoveAll(testDir)
	initLogging(false)

	if err := data.Init(testDir); err != nil {
		slog.Error("fatal error", "error", err)
		os.Exit(1)
	}

	code := m.Run()
	os.Exit(code)
}
