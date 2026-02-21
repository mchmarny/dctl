package main

import (
	"context"
	"embed"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/urfave/cli/v2"
)

const (
	serverShutdownWaitSeconds = 5
	serverTimeoutSeconds      = 300
	serverMaxHeaderBytes      = 20
	serverPortDevault         = 8080
)

var (
	//go:embed assets/* templates/*
	f embed.FS

	portFlag = &cli.IntFlag{
		Name:     "port",
		Usage:    "Port on which the server will listen",
		Value:    serverPortDevault,
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
	port := c.Int(portFlag.Name)
	address := fmt.Sprintf("127.0.0.1:%d", port)

	r := makeRouter()
	s := &http.Server{
		Addr:           address,
		Handler:        r,
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

func makeRouter() *gin.Engine {
	gin.SetMode(gin.ReleaseMode)

	var r *gin.Engine
	if debug {
		r = gin.Default()
	} else {
		r = gin.New()
		r.Use(gin.Recovery())
	}

	// templates
	r.SetHTMLTemplate(template.Must(template.New("").ParseFS(f, "templates/*.html")))

	// enables '/static/assets/img/logo.png'
	r.StaticFS("/static", http.FS(f))

	// // statics resources
	r.GET("favicon.ico", faveIcon)

	// dynamic views
	r.GET("/", homeViewHandler)

	// data queries returning JSON
	data := r.Group("/data")
	data.GET("/query", queryHandler)
	data.GET("/type", eventDataHandler)
	data.GET("/entity", entityDataHandler)
	data.GET("/developer", developerDataHandler)
	data.POST("/search", eventSearchHandler)

	return r
}
