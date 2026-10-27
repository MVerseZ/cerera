//go:build !windows && !linux

package main

import (
	"bufio"
	"os"
	"strings"
)

const terminalNeedsEnter = true

var terminalInput = bufio.NewReader(os.Stdin)

// Unix terminals fall back to line input when raw terminal handling is not
// available in the standard library build.
func readKey() (rune, error) {
	line, err := terminalInput.ReadString('\n')
	if err != nil && len(line) == 0 {
		return 0, err
	}
	line = strings.TrimSpace(line)
	if line == "" {
		return '\n', nil
	}
	for _, r := range line {
		return r, nil
	}
	return '\n', nil
}

func setupTerminal() func() { return func() {} }

func readTextLine() (string, error) {
	line, err := terminalInput.ReadString('\n')
	return strings.TrimRight(line, "\r\n"), err
}
