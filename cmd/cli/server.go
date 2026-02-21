package main

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
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

	serverCmd = &cli.Command{
		Name:    "server",
		Aliases: []string{"s"},
		Usage:   "Start local HTTP server",
		Action:  cmdStartServer,
		Flags: []cli.Flag{
			portFlag,
		},
	}
)

func cmdStartServer(c *cli.Context) error {
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

	slog.Info("server started", "address", address)

	<-done

	ctx, cancel := context.WithTimeout(context.Background(), serverShutdownWaitSeconds*time.Second)
	defer cancel()

	if err := s.Shutdown(ctx); err != nil && err != http.ErrServerClosed {
		slog.Error("error shutting down server", "error", err)
	}
	return nil
}

func makeRouter(db *sql.DB) *http.ServeMux {
	tmpl := template.Must(template.New("").ParseFS(embedFS, "templates/*.html"))

	mux := http.NewServeMux()

	// Static files
	mux.Handle("GET /static/", http.FileServerFS(embedFS))
	mux.HandleFunc("GET /favicon.ico", faviconHandler)

	// Views
	mux.HandleFunc("GET /{$}", homeViewHandler(tmpl))

	// Data API
	mux.HandleFunc("GET /data/query", queryAPIHandler(db))
	mux.HandleFunc("GET /data/type", eventDataAPIHandler(db))
	mux.HandleFunc("GET /data/entity", entityDataAPIHandler(db))
	mux.HandleFunc("GET /data/developer", developerDataAPIHandler(db))
	mux.HandleFunc("POST /data/search", eventSearchAPIHandler(db))

	return mux
}
