package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	cmdName := os.Args[0]
	if len(os.Args) == 1 {
		fmt.Fprintf(os.Stdout, "%q of python run environment was started with no command and arguments", cmdName)
	}

	const columnWidth = 8

	argsReport := strings.Builder{}

	fmt.Fprintf(&argsReport, "%*s = %q\n", columnWidth, "cmd", cmdName)
	for i, a := range os.Args[1:] {
		argStr := fmt.Sprintf("arg[%d]", i)
		fmt.Fprintf(&argsReport, "%*s = %q\n", columnWidth, argStr, a)
	}

	fmt.Fprint(os.Stdout, argsReport.String()+"\n")

	if isVenvCreation(os.Args) {
		createFakeVenv(os.Args)
		fmt.Fprintf(os.Stdout, "venv directory created\n")
		return
	}
}

func isVenvCreation(args []string) bool {
	if len(args) < 3 {
		return false
	}

	return strings.HasSuffix(args[0], "python") &&
		args[1] == "-m" && args[2] == "venv"
}

func createFakeVenv(args []string) {
	venvDir := args[len(args)-1]
	binDir := filepath.Join(venvDir, "bin")

	must(
		os.MkdirAll(binDir, 0o750),
		"python stub: failed to create %q dir", binDir,
	)

	exec, err := os.Executable()
	must(err, "failed to get current executable path")

	must(copyFile(exec, filepath.Join(binDir, "python")), "failed to put 'python' into %q", binDir)
	must(copyFile(exec, filepath.Join(binDir, "pip")), "failed to put 'pip' into %q", binDir)
}

func copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	if err != nil {
		return err
	}

	sourceInfo, err := os.Stat(src)
	if err != nil {
		return err
	}

	err = os.Chmod(dst, sourceInfo.Mode())
	if err != nil {
		return err
	}

	return nil
}

func must(err error, msg string, args ...any) {
	if err != nil {
		args = append(args, err)
		panic(fmt.Errorf(msg+": %w", args...))
	}
}
