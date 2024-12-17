package main

import (
    "fmt"
    "os"
    "strconv"
    "time"
)

func main() {
    if len(os.Args) != 2 {
	fmt.Fprintf(os.Stderr, "Usage: %s <seconds>\n", os.Args[0])
	os.Exit(1)
    }

    seconds, err := strconv.Atoi(os.Args[1])
    if err != nil {
	fmt.Fprintf(os.Stderr, "Invalid number of seconds: %s\n", err)
	os.Exit(1)
    }

    time.Sleep(time.Duration(seconds) * time.Second)
}
