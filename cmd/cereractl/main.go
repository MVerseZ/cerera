package main

import (
	"fmt"
	"os"

	"github.com/cerera/internal/gigea/gigea"
)

func main() {
	os.Exit(run())
}

func run() int {

	argsWithoutProg := os.Args[1:]

	if len(argsWithoutProg) != 1 {
		fmt.Printf("Usage:\r\n\r\n")
		fmt.Println(Usage())
		return 0
	}

	var cereraCtlCommand = argsWithoutProg[0]
	var code = commands[cereraCtlCommand]

	if code == 0 {
		fmt.Println("Wrong command")
		fmt.Printf("Usage:\r\n\r\n")
		fmt.Println(Usage())
		return 0
	}

	gigea.ExecuteCtl(code)
	return 1
}
