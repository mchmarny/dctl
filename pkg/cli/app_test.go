package cli

import (
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/mchmarny/devpulse/pkg/data/sqlite"
)

const (
	testDir = "../../tmp"
)

func TestMain(m *testing.M) {
	os.RemoveAll(testDir)
	if err := os.MkdirAll(testDir, 0o700); err != nil {
		slog.Error("creating test dir", "error", err)
		os.Exit(1)
	}
	initLogging(false)

	store, err := sqlite.New(filepath.Join(testDir, "data.db"))
	if err != nil {
		slog.Error("fatal error", "error", err)
		os.Exit(1)
	}

	code := m.Run()
	store.Close()
	os.Exit(code)
}
