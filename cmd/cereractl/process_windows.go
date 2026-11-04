//go:build windows

package main

import (
	"os"
	"os/exec"
)

func launchNode(cmd *exec.Cmd) error {
	return cmd.Start()
}

func signalNodeStop(cmd *exec.Cmd) error {
	if cmd == nil || cmd.Process == nil {
		return nil
	}
	return cmd.Process.Signal(os.Interrupt)
}
