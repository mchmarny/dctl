package cli

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/urfave/cli/v2"
)

const (
	serverShutdownWaitSeconds = 5
	serverTimeoutSeconds      = 300
	serverMaxHeaderBytes      = 20
	serverPortDefault         = 8080
)

var (
	//go:embed assets/* templates/*
	embedFS embed.FS

	portFlag = &cli.IntFlag{
		Name:     "port",
		Usage:    "Port on which the server will listen",
		Value:    serverPortDefault,
		Required: false,
	}

	noBrowserFlag = &cli.BoolFlag{
		Name:    "no-browser",
		Aliases: []string{"nb"},
		Usage:   "Do not open browser automatically",
	}

	serverCmd = &cli.Command{
		Name:    "server",
		Aliases: []string{"serve"},
		Usage:   "Start local HTTP server",
		Action:  cmdStartServer,
		Flags: []cli.Flag{
			portFlag,
			noBrowserFlag,
			debugFlag,
		},
	}
)

func cmdStartServer(c *cli.Context) error {
	applyFlags(c)
	cfg := getConfig(c)
	port := c.Int(portFlag.Name)
	address := fmt.Sprintf("127.0.0.1:%d", port)

	mux := makeRouter(cfg.DB)
	s := &http.Server{
		Addr:           address,
		Handler:        mux,
		ReadTimeout:    serverTimeoutSeconds * time.Second,
		WriteTimeout:   serverTimeoutSeconds * time.Second,
		MaxHeaderBytes: 1 << serverMaxHeaderBytes,
	}

	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		if err := s.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("error starting server", "error", err)
		}
	}()

	url := fmt.Sprintf("http://%s", address)
	slog.Info("server started", "address", url)

	if !c.Bool(noBrowserFlag.Name) {
		openBrowser(url)
	}

	<-done

	ctx, cancel := context.WithTimeout(context.Background(), serverShutdownWaitSeconds*time.Second)
	defer cancel()

	if err := s.Shutdown(ctx); err != nil && !errors.Is(err, http.ErrServerClosed) {
		slog.Error("error shutting down server", "error", err)
	}
	return nil
}

func makeRouter(db *sql.DB) *http.ServeMux {
	tmpl := template.Must(template.New("").ParseFS(embedFS, "templates/*.html"))

	mux := http.NewServeMux()

	// Static files
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServerFS(embedFS)))
	mux.HandleFunc("GET /favicon.ico", faviconHandler)

	// Views
	mux.HandleFunc("GET /{$}", homeViewHandler(tmpl))

	// Data API
	mux.HandleFunc("GET /data/min-date", minDateAPIHandler(db))
	mux.HandleFunc("GET /data/query", queryAPIHandler(db))
	mux.HandleFunc("GET /data/type", eventDataAPIHandler(db))
	mux.HandleFunc("GET /data/entity", entityDataAPIHandler(db))
	mux.HandleFunc("GET /data/developer", developerDataAPIHandler(db))
	mux.HandleFunc("POST /data/search", eventSearchAPIHandler(db))
	mux.HandleFunc("GET /data/entity/developers", entityDevelopersAPIHandler(db))

	// Insights API
	mux.HandleFunc("GET /data/insights/summary", insightsSummaryAPIHandler(db))
	mux.HandleFunc("GET /data/insights/retention", insightsRetentionAPIHandler(db))
	mux.HandleFunc("GET /data/insights/pr-ratio", insightsPRRatioAPIHandler(db))
	mux.HandleFunc("GET /data/insights/time-to-merge", insightsTimeToMergeAPIHandler(db))
	mux.HandleFunc("GET /data/insights/time-to-close", insightsTimeToCloseAPIHandler(db))
	mux.HandleFunc("GET /data/insights/forks-and-activity", insightsForksAndActivityAPIHandler(db))
	mux.HandleFunc("GET /data/insights/repo-meta", insightsRepoMetaAPIHandler(db))
	mux.HandleFunc("GET /data/insights/release-cadence", insightsReleaseCadenceAPIHandler(db))
	mux.HandleFunc("GET /data/insights/release-downloads", insightsReleaseDownloadsAPIHandler(db))
	mux.HandleFunc("GET /data/insights/release-downloads-by-tag", insightsReleaseDownloadsByTagAPIHandler(db))
	mux.HandleFunc("GET /data/insights/reputation", insightsReputationAPIHandler(db))
	mux.HandleFunc("GET /data/insights/reputation/user", reputationUserAPIHandler(db))

	return mux
}

func openBrowser(url string) {
	var cmd string
	args := make([]string, 0, 1)

	switch runtime.GOOS {
	case "darwin":
		cmd = "open"
	case "linux":
		cmd = "xdg-open"
	default: // windows
		cmd = "rundll32"
		args = []string{"url.dll,FileProtocolHandler"}
	}

	args = append(args, url)
	if err := exec.Command(cmd, args...).Start(); err != nil {
		slog.Error("failed to open browser", "error", err)
	}
}
