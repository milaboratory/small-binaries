package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func main() {
	cmdName := os.Args[0]
	if len(os.Args) == 1 {
		fmt.Fprintf(os.Stdout, "%q of python run environment was started with no command and arguments", cmdName)
	}

	const venvDir = "./venv"
	const columnWidth = 8

	err := os.MkdirAll(venvDir, 0o750)
	if err != nil {
		panic(fmt.Errorf("python stub: failed to create %q dir: %w", venvDir, err))
	}

	argsReport := strings.Builder{}

	fmt.Fprintf(&argsReport, "%*s = %q\n", columnWidth, "cmd", cmdName)
	for i, a := range os.Args[1:] {
		argStr := fmt.Sprintf("arg[%d]", i)
		fmt.Fprintf(&argsReport, "%*s = %q\n", columnWidth, argStr, a)
	}
	fmt.Fprintf(&argsReport, "%*s = %d", columnWidth, "time", time.Now().Unix())

	contentFileValue := argsReport.String() + "\n"

	contentFilePath := filepath.Join(venvDir, cmdName+".txt")
	err = os.WriteFile(contentFilePath, []byte(contentFileValue), 0o640)

	fmt.Fprintf(os.Stdout, "file %q created\n", contentFilePath)
	fmt.Fprint(os.Stdout, contentFileValue)
}
