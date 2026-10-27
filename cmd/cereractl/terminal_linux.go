//go:build linux

package main

import (
	"bufio"
	"os"
	"strings"
	"syscall"
	"unsafe"
)

const terminalNeedsEnter = false

var terminalInput = bufio.NewReader(os.Stdin)

func readKey() (rune, error) {
	fd := os.Stdin.Fd()
	var original syscall.Termios
	if _, _, errno := syscall.Syscall(syscall.SYS_IOCTL, fd, uintptr(syscall.TCGETS), uintptr(unsafe.Pointer(&original))); errno != 0 {
		return 0, errno
	}
	raw := original
	raw.Lflag &^= syscall.ICANON | syscall.ECHO
	if _, _, errno := syscall.Syscall(syscall.SYS_IOCTL, fd, uintptr(syscall.TCSETS), uintptr(unsafe.Pointer(&raw))); errno != 0 {
		return 0, errno
	}
	defer syscall.Syscall(syscall.SYS_IOCTL, fd, uintptr(syscall.TCSETS), uintptr(unsafe.Pointer(&original)))

	input, err := terminalInput.ReadByte()
	if err != nil {
		return 0, err
	}
	return rune(input), nil
}

func setupTerminal() func() { return func() {} }

func readTextLine() (string, error) {
	line, err := terminalInput.ReadString('\n')
	return strings.TrimRight(line, "\r\n"), err
}
