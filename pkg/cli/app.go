package cli

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/mchmarny/devpulse/pkg/data"
	"github.com/mchmarny/devpulse/pkg/data/sqlite"
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

	logJSONFlag = &urfave.BoolFlag{
		Name:    "log-json",
		Usage:   "Output logs in JSON format (optional, default: false)",
		Sources: urfave.EnvVars("DEVPULSE_LOG_JSON"),
	}

	dbFilePathFlag = &urfave.StringFlag{
		Name:    "db",
		Usage:   "SQLite file path",
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
	initLogging(envBool("DEVPULSE_DEBUG"), envBool("DEVPULSE_LOG_JSON"))

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)

	cmd := newApp()
	if err := cmd.Run(ctx, os.Args); err != nil {
		stop()
		slog.Error("fatal error", "error", err)
		os.Exit(1)
	}
	stop()
}

type appConfig struct {
	DSN   string
	Store data.Store
}

func getConfig(cmd *urfave.Command) *appConfig {
	if cfg, ok := cmd.Root().Metadata[appConfigKey].(*appConfig); ok {
		return cfg
	}

	applyFlags(cmd)

	dsn := cmd.String(dbFilePathFlag.Name)
	store, err := openStore(dsn)
	if err != nil {
		slog.Error("initializing store", "error", err)
		os.Exit(1)
	}

	cfg := &appConfig{DSN: dsn, Store: store}
	cmd.Root().Metadata[appConfigKey] = cfg
	return cfg
}

func newApp() *urfave.Command {
	return &urfave.Command{
		Name:                  "devpulse",
		Version:               fmt.Sprintf("%s (%s - %s)", version, commit, date),
		EnableShellCompletion: true,
		HideHelpCommand:       true,
		Usage:                 "CLI for quick insight into the GitHub org/repo activity",
		Commands: []*urfave.Command{
			authCmd,
			importCmd,
			deleteCmd,
			scoreCmd,
			substituteCmd,
			queryCmd,
			serverCmd,
			syncCmd,
			resetCmd,
		},
		After: func(ctx context.Context, cmd *urfave.Command) error {
			if cfg, ok := cmd.Root().Metadata[appConfigKey].(*appConfig); ok && cfg.Store != nil {
				if err := cfg.Store.Close(); err != nil {
					slog.Error("error closing store", "error", err)
				}
			}
			return nil
		},
		Metadata: map[string]any{},
	}
}

func openStore(dsn string) (data.Store, error) {
	return sqlite.New(dsn)
}

func initLogging(debug, jsonFormat bool) {
	level := "info"
	if debug {
		level = "debug"
	}
	if jsonFormat {
		logging.SetDefaultJSONLogger(level)
	} else {
		logging.SetDefaultCLILogger(level)
	}
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
	debug := cmd.Bool(debugFlag.Name)
	jsonLog := cmd.Bool(logJSONFlag.Name)
	if debug || jsonLog {
		initLogging(debug, jsonLog)
	}
	f := cmd.String(formatFlag.Name)
	if f == formatYAML || f == "yml" {
		outputFormat = formatYAML
	}
}

func envBool(key string) bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv(key)))
	return v == "true" || v == "1"
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
