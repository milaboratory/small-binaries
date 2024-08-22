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

	fmt.Fprint(os.Stdout, strings.Join(os.Args[1:], "\n"))
}
