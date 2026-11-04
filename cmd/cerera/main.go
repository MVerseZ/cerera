package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/cerera/internal/cli"
	"github.com/cerera/internal/logger"
	"github.com/cerera/internal/node"
)

var appLog = logger.Named("cmd.cerera")

func setupLogging(logPath string) error {
	_, err := logger.Init(logger.Config{
		Path:    logPath,
		Level:   "debug",
		Console: true,
	})
	return err
}

func waitForShutdownSignal() {
	sigCh := make(chan os.Signal, 2)
	registerShutdownSignals(sigCh)
	sig := <-sigCh
	fmt.Fprintf(os.Stderr, "\nShutdown requested (%v)\n", sig)
}

func main() {
	opts, err := cli.ParseOptions(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load options: %v\n", err)
		os.Exit(1)
	}

	if err := setupLogging(opts.LogPath); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to setup logging: %v\n", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	n, err := node.New(&opts.Config, ctx, opts.Mode, opts.Port, opts.HTTPPort, opts.Mine)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Cerera failed to start: %v\n", err)
		appLog.Errorw("Failed to initialize Cerera", "err", err)
		logger.Sync()
		os.Exit(1)
	}

	appLog.Infow("Cerera node running; press Ctrl+C to stop")
	fmt.Fprintln(os.Stderr, "Cerera running — Ctrl+C to stop")

	waitForShutdownSignal()
	cancel()
	appLog.Infow("Shutdown signal received")

	forceCh := make(chan os.Signal, 1)
	registerShutdownSignals(forceCh)
	go func() {
		<-forceCh
		fmt.Fprintln(os.Stderr, "Force exit")
		os.Exit(1)
	}()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer shutdownCancel()

	shutdownDone := make(chan error, 1)
	go func() {
		shutdownDone <- n.Shutdown(shutdownCtx)
	}()

	select {
	case err := <-shutdownDone:
		if err != nil {
			appLog.Errorw("Shutdown completed with errors", "err", err)
			fmt.Fprintf(os.Stderr, "Shutdown errors: %v\n", err)
		} else {
			fmt.Fprintln(os.Stderr, "Shutdown complete")
		}
	case <-shutdownCtx.Done():
		fmt.Fprintln(os.Stderr, "Shutdown timed out — forcing exit")
		os.Exit(1)
	}

	logger.Sync()
	os.Exit(0)
}
