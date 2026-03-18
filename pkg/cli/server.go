package cli

import (
	"context"
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
	"strings"
	"syscall"
	"time"

	"github.com/mchmarny/devpulse/pkg/data"
	"github.com/urfave/cli/v3"
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
		Name:    "port",
		Usage:   "Port on which the server will listen",
		Value:   serverPortDefault,
		Sources: cli.EnvVars("DEVPULSE_PORT"),
	}

	noBrowserFlag = &cli.BoolFlag{
		Name:    "no-browser",
		Aliases: []string{"nb"},
		Usage:   "Do not open browser automatically",
		Sources: cli.EnvVars("DEVPULSE_NO_BROWSER"),
	}

	basePathFlag = &cli.StringFlag{
		Name:    "base-path",
		Usage:   "Base URL path when hosted behind a reverse proxy (e.g. /devpulse)",
		Sources: cli.EnvVars("DEVPULSE_BASE_PATH"),
	}

	addressFlag = &cli.StringFlag{
		Name:    "address",
		Aliases: []string{"addr"},
		Usage:   "Bind address (e.g. 0.0.0.0 for all interfaces)",
		Value:   "127.0.0.1",
		Sources: cli.EnvVars("DEVPULSE_ADDRESS"),
	}

	serverCmd = &cli.Command{
		Name:    "server",
		Aliases: []string{"view"},
		Usage:   "Start local HTTP server",
		Action:  cmdStartServer,
		Flags: []cli.Flag{
			portFlag,
			addressFlag,
			noBrowserFlag,
			basePathFlag,
			debugFlag,
			logJSONFlag,
		},
	}
)

var errInvalidBasePath = errors.New("base path must not contain '..' or '://'")

func normalizeBasePath(raw string) (string, error) {
	bp := strings.TrimRight(strings.TrimSpace(raw), "/")
	if bp == "" {
		return "", nil
	}
	if strings.Contains(bp, "..") || strings.Contains(bp, "://") {
		return "", errInvalidBasePath
	}
	if !strings.HasPrefix(bp, "/") {
		bp = "/" + bp
	}
	return bp, nil
}

func cmdStartServer(_ context.Context, cmd *cli.Command) error {
	applyFlags(cmd)
	cfg := getConfig(cmd)
	port := cmd.Int(portFlag.Name)
	addr := cmd.String(addressFlag.Name)
	address := fmt.Sprintf("%s:%d", addr, port)
	basePath, err := normalizeBasePath(cmd.String(basePathFlag.Name))
	if err != nil {
		return fmt.Errorf("invalid base path: %w", err)
	}

	mux := makeRouter(cfg.Store, basePath)

	var handler http.Handler = mux
	if basePath != "" {
		handler = http.StripPrefix(basePath, mux)
	}

	s := &http.Server{
		Addr:           address,
		Handler:        handler,
		ReadTimeout:    serverTimeoutSeconds * time.Second,
		WriteTimeout:   serverTimeoutSeconds * time.Second,
		MaxHeaderBytes: 1 << serverMaxHeaderBytes,
	}

	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		if err := s.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("failed to start", "error", err)
		}
	}()

	url := fmt.Sprintf("http://%s%s", address, basePath)
	slog.Info("started", "address", url)

	if !cmd.Bool(noBrowserFlag.Name) {
		openBrowser(url)
	}

	<-done

	ctx, cancel := context.WithTimeout(context.Background(), serverShutdownWaitSeconds*time.Second)
	defer cancel()

	if err := s.Shutdown(ctx); err != nil && !errors.Is(err, http.ErrServerClosed) {
		slog.Error("shutdown failed", "error", err)
	}
	return nil
}

func makeRouter(store data.Store, basePath string) *http.ServeMux {
	tmpl := template.Must(template.New("").ParseFS(embedFS, "templates/*.html"))

	mux := http.NewServeMux()

	// Static files
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServerFS(embedFS)))
	mux.HandleFunc("GET /favicon.ico", faviconHandler())

	// Views
	mux.HandleFunc("GET /{$}", homeViewHandler(tmpl, basePath))

	// Data API
	mux.HandleFunc("GET /data/min-date", minDateAPIHandler(store))
	mux.HandleFunc("GET /data/query", queryAPIHandler(store))
	mux.HandleFunc("GET /data/type", eventDataAPIHandler(store))
	mux.HandleFunc("GET /data/entity", entityDataAPIHandler(store))
	mux.HandleFunc("GET /data/developer", developerDataAPIHandler(store))
	mux.HandleFunc("POST /data/search", eventSearchAPIHandler(store))
	mux.HandleFunc("GET /data/entity/developers", entityDevelopersAPIHandler(store))

	// Insights API
	mux.HandleFunc("GET /data/insights/summary", insightsSummaryAPIHandler(store))
	mux.HandleFunc("GET /data/insights/daily-activity", insightsDailyActivityAPIHandler(store))
	mux.HandleFunc("GET /data/insights/retention", insightsRetentionAPIHandler(store))
	mux.HandleFunc("GET /data/insights/pr-ratio", insightsPRRatioAPIHandler(store))
	mux.HandleFunc("GET /data/insights/time-to-merge", insightsTimeToMergeAPIHandler(store))
	mux.HandleFunc("GET /data/insights/time-to-close", insightsTimeToCloseAPIHandler(store))
	mux.HandleFunc("GET /data/insights/time-to-restore", insightsTimeToRestoreAPIHandler(store))
	mux.HandleFunc("GET /data/insights/review-latency", insightsReviewLatencyAPIHandler(store))
	mux.HandleFunc("GET /data/insights/forks-and-activity", insightsForksAndActivityAPIHandler(store))
	mux.HandleFunc("GET /data/insights/repo-meta", insightsRepoMetaAPIHandler(store))
	mux.HandleFunc("GET /data/insights/repo-overview", insightsRepoOverviewAPIHandler(store))
	mux.HandleFunc("GET /data/insights/repo-metric-history", insightsRepoMetricHistoryAPIHandler(store))
	mux.HandleFunc("GET /data/insights/change-failure-rate", insightsChangeFailureRateAPIHandler(store))
	mux.HandleFunc("GET /data/insights/pr-size", insightsPRSizeAPIHandler(store))
	mux.HandleFunc("GET /data/insights/contributor-momentum", insightsContributorMomentumAPIHandler(store))
	mux.HandleFunc("GET /data/insights/contributor-funnel", insightsContributorFunnelAPIHandler(store))
	mux.HandleFunc("GET /data/insights/contributor-profile", insightsContributorProfileAPIHandler(store))
	mux.HandleFunc("GET /data/developer/search", developerSearchAPIHandler(store))
	mux.HandleFunc("GET /data/insights/release-cadence", insightsReleaseCadenceAPIHandler(store))
	mux.HandleFunc("GET /data/insights/release-downloads", insightsReleaseDownloadsAPIHandler(store))
	mux.HandleFunc("GET /data/insights/release-downloads-by-tag", insightsReleaseDownloadsByTagAPIHandler(store))
	mux.HandleFunc("GET /data/insights/container-activity", insightsContainerActivityAPIHandler(store))
	mux.HandleFunc("GET /data/insights/reputation", insightsReputationAPIHandler(store))

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
