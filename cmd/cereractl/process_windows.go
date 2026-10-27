//go:build windows

package main

import (
	"os/exec"
	"syscall"
)

var generateConsoleCtrlEvent = kernel32.NewProc("GenerateConsoleCtrlEvent")

func launchNode(cmd *exec.Cmd) error {
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: 0x00000200} // CREATE_NEW_PROCESS_GROUP
	return cmd.Start()
}

func signalNodeStop(cmd *exec.Cmd) error {
	const ctrlBreakEvent = 1
	ok, _, err := generateConsoleCtrlEvent.Call(ctrlBreakEvent, uintptr(cmd.Process.Pid))
	if ok == 0 {
		return err
	}
	return nil
}
