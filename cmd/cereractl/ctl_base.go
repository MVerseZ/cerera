package main

import (
	"fmt"
	"strings"
)

var commands = map[string]int{
	"config":    100,
	"ipconfig":  101,
	"showpeers": 110,
	"start":     111,
	"status":    1000,
	"stop":      1001,
	"test":      1010,
	"verify":    1100,
}

var descriptions = map[string]string{
	"status": "Print current status",
	"test":   "Test command",
}

func Usage() string {
	mymap := make(map[int]string)
	keys := make([]string, 0, len(mymap))
	for k := range commands {
		keys = append(keys, fmt.Sprintf("\t%s: %s\r\n", k, descriptions[k]))
	}
	return strings.Join(keys, "")
}
