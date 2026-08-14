package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"syscall"

	"github.com/cerera/core/chain"
	"github.com/cerera/internal/service"
)

var commands = map[int][]string{
	100:  {"config"},
	101:  {"ipconfig"},
	110:  {"showpeers"},
	1000: {"status", "st"},
	1010: {"help", "h", "?"},
	1100: {"quit", "q", "exit"},
}

var descriptions = map[int]string{
	100:  "Show or modify configuration",
	101:  "Show IP configuration",
	110:  "Show connected peers",
	1000: "Print current status",
	1010: "Show available commands",
	1100: "Exit the program",
}

func Usage() string {
	var result strings.Builder
	for code, cmds := range commands {
		cmdString := strings.Join(cmds, ", ")
		description := descriptions[code]
		result.WriteString(fmt.Sprintf("\t%s: %s\r\n", cmdString, description))
	}
	return result.String()
}

type CommandHandler struct {
	scanner *bufio.Scanner
	running bool
	sigChan chan os.Signal
}

func NewCommandHandler(sigs chan os.Signal) *CommandHandler {
	return &CommandHandler{
		scanner: bufio.NewScanner(os.Stdin),
		running: true,
		sigChan: sigs,
	}
}

func (c *CommandHandler) Start() {
	for c.running {
		fmt.Print("> ")
		if !c.scanner.Scan() {
			break
		}

		input := strings.TrimSpace(c.scanner.Text())
		if input == "" {
			continue
		}

		c.ExecuteCommand(input)
	}
}

func (c *CommandHandler) ExecuteCommand(input string) {
	parts := strings.Fields(input)
	if len(parts) == 0 {
		return
	}

	command := parts[0]
	// args := parts[1:]

	var commandCode = -1
	for code, cmds := range commands {
		for _, cmd := range cmds {
			if cmd == command {
				commandCode = code
				break
			}
		}
	}

	if commandCode == -1 {
		fmt.Printf("Unknown command: %s\n", command)
		fmt.Println("Enter 'help' or 'h' or '?' for a list of available commands")
		return
	}

	switch commandCode {
	case 1010:
		// c.showHelp()
		fmt.Println(Usage())
	case 1100:
		fmt.Println("Shutting down...")
		c.running = false
		c.sigChan <- syscall.SIGINT
	case 1000:
		execResult := service.Exec("cerera.chain.getInfo", nil)
		status := execResult.(chain.BlockChainStatus)
		println(status.String())
	default:
		fmt.Printf("Unknown command: %s\n", command)
		fmt.Println("Enter 'help' or 'h' or '?' for a list of available commands")
	}
}
