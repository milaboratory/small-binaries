package main

import (
	"fmt"
	"os"
	"strings"
)

func main() {
	cmdName := os.Args[0]

	if len(os.Args) == 1 {
		fmt.Fprintf(os.Stdout, "%q was started with no arguments", cmdName)
	}

	const columnWidth = 8

	argsReport := strings.Builder{}

	fmt.Fprintf(&argsReport, "%*s = %q\n", columnWidth, "cmd", cmdName)
	for i, a := range os.Args[1:] {
		argStr := fmt.Sprintf("arg[%d]", i)
		fmt.Fprintf(&argsReport, "%*s = %q\n", columnWidth, argStr, a)
	}

	fmt.Fprint(os.Stdout, argsReport.String()+"\n")
}
