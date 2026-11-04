//go:build !windows

package main

import (
	"os"
	"os/signal"
	"syscall"
)

func registerShutdownSignals(ch chan os.Signal) {
	signal.Notify(ch, os.Interrupt, syscall.SIGTERM)
}
