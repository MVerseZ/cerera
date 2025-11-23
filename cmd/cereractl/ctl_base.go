package main

import (
	"fmt"
	"strings"
)

var commands = map[string]int{
	"config":    100,
	"ipconfig":  101,
	"showpeers": 110,
	"status":    1000,
	"help":      1010,
	"exit":      1100,
}

var descriptions = map[string]string{
	"config":    "Show or modify configuration",
	"ipconfig":  "Show IP configuration",
	"showpeers": "Show connected peers",
	"status":    "Print current status",
	"help":      "Show available commands",
	"exit":      "Exit the program",
}

func Usage() string {
	keys := make([]string, 0, len(commands))
	for k := range commands {
		desc := descriptions[k]
		if desc == "" {
			desc = "No description available"
		}
		keys = append(keys, fmt.Sprintf("\t%s: %s\r\n", k, desc))
	}
	return strings.Join(keys, "")
}
