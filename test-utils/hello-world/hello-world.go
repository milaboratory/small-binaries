package main

import (
    "fmt"
    "os"
)

func main() {
    if len(os.Args) > 2 {
	fmt.Fprintf(os.Stderr, "Usage: %s [<message>]\n", os.Args[0])
	os.Exit(1)
    }

	msg := "Hello from go binary"
	if len(os.Args) == 2 {
		msg = os.Args[1]
	}

	fmt.Fprintf(os.Stdout, "%s\n", msg)
}
