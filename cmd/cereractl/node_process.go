package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

type managedNode struct {
	cmd      *exec.Cmd
	done     chan error
	finished bool
	exitErr  error
	logPath  string
}

func startManagedNode(binary, dataDir, p2pPort string, httpPort int, tls, mining bool, nodeKeyPath string) (*managedNode, error) {
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		return nil, fmt.Errorf("create data directory: %w", err)
	}
	logPath := filepath.Join(dataDir, "logfile")
	args := []string{
		"-mode=server",
		"-port=" + p2pPort,
		fmt.Sprintf("-http=%d", httpPort),
		fmt.Sprintf("-miner=%t", mining),
		"-data-dir=" + dataDir,
		"-key=" + nodeKeyPath,
	}
	if tls {
		args = append(args, "-s")
	}
	cmd := exec.Command(binary, args...)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	if err := launchNode(cmd); err != nil {
		return nil, fmt.Errorf("start %s: %w", binary, err)
	}
	node := &managedNode{cmd: cmd, done: make(chan error, 1), logPath: logPath}
	go func() {
		node.done <- cmd.Wait()
	}()
	return node, nil
}

func (n *managedNode) refresh() {
	if n == nil || n.finished {
		return
	}
	select {
	case n.exitErr = <-n.done:
		n.finished = true
	default:
	}
}

func (n *managedNode) running() bool {
	if n == nil {
		return false
	}
	n.refresh()
	return !n.finished
}

func stopManagedNode(n *managedNode) error {
	if n == nil || !n.running() {
		return nil
	}
	if err := signalNodeStop(n.cmd); err != nil {
		return fmt.Errorf("send shutdown signal: %w", err)
	}
	select {
	case n.exitErr = <-n.done:
		n.finished = true
		return nil
	case <-time.After(5 * time.Second):
		if err := n.cmd.Process.Kill(); err != nil {
			return fmt.Errorf("force stop after timeout: %w", err)
		}
		n.exitErr = <-n.done
		n.finished = true
		return nil
	}
}
