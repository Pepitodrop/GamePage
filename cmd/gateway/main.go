package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Pepitodrop/GamePage/internal/gateway"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "healthcheck" {
		os.Exit(runHealthcheck())
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	config, err := gateway.LoadConfig()
	if err != nil {
		logger.Error("invalid configuration", "error", err)
		os.Exit(1)
	}
	app, err := gateway.New(config, logger)
	if err != nil {
		logger.Error("could not initialize gateway", "error", err)
		os.Exit(1)
	}

	server := &http.Server{
		Addr:              config.ListenAddress,
		Handler:           app.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       75 * time.Second,
		MaxHeaderBytes:    32 << 10,
	}

	shutdownSignal, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		<-shutdownSignal.Done()
		shutdownContext, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownContext); err != nil {
			logger.Error("graceful shutdown failed", "error", err)
		}
	}()

	logger.Info("gateway starting", "config", gateway.MarshalConfigSummary(config))
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Error("gateway stopped unexpectedly", "error", err)
		os.Exit(1)
	}
	logger.Info("gateway stopped")
}

func runHealthcheck() int {
	client := &http.Client{Timeout: 2 * time.Second}
	response, err := client.Get("http://127.0.0.1:8080/healthz")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		fmt.Fprintf(os.Stderr, "unexpected status: %s\n", response.Status)
		return 1
	}
	return 0
}
