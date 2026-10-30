package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
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
	defer logger.Sync()

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	n, err := node.New(&opts.Config, ctx, opts.Mode, opts.Port, opts.HTTPPort, opts.Mine)
	if err != nil {
		appLog.Errorw("Failed to initialize Cerera", "err", err)
		os.Exit(1)
	}

	appLog.Infow("Cerera node running; press Ctrl+C to stop")
	<-ctx.Done()
	appLog.Infow("Shutdown signal received")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()
	if err := n.Shutdown(shutdownCtx); err != nil {
		appLog.Errorw("Shutdown completed with errors", "err", err)
	}
}
