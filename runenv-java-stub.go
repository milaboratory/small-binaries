package main

import (
	"fmt"
	"os"
	"strings"
	"time"
)

func main() {
	cmdName := os.Args[0]

	if len(os.Args) == 1 {
		fmt.Fprintf(os.Stdout, "%q of java run environment was started with no command and arguments", cmdName)
	}

	const columnWidth = 8

	argsReport := strings.Builder{}

	fmt.Fprintf(&argsReport, "%*s = %q\n", columnWidth, "cmd", cmdName)
	for i, a := range os.Args[1:] {
		argStr := fmt.Sprintf("arg[%d]", i)
		fmt.Fprintf(&argsReport, "%*s = %q\n", columnWidth, argStr, a)
	}
	fmt.Fprintf(&argsReport, "%*s = %d", columnWidth, "time", time.Now().Unix())

	fmt.Fprint(os.Stdout, argsReport.String()+"\n")
}
