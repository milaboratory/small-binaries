package main

import (
	"fmt"
	"os"
	"strings"
)

func main() {
	if len(os.Args) == 1 {
		fmt.Fprintf(os.Stdout, "run environment was started with no command and arguments")
	}

	cmdToRun := make([]string, 0, len(os.Args))
	for _, arg := range cmdToRun {
		cmdToRun = append(cmdToRun, fmt.Sprintf("%q", arg))
	}

	fmt.Fprint(os.Stdout, strings.Join(cmdToRun, " "))
}
