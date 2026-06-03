package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/milaboratory/small-binaries/line-counter/internal"
)

func main() {
	input := flag.String("input", "", "input file (optionally .gz/.bz2/.zst)")
	output := flag.String("output", "", "output file for the line count")
	flag.Parse()
	if *input == "" || *output == "" {
		fmt.Fprintln(os.Stderr, "usage: line-counter --input <file> --output <file>")
		os.Exit(2)
	}
	n, err := internal.CountLines(*input)
	if err != nil {
		fmt.Fprintln(os.Stderr, "line-counter:", err)
		os.Exit(1)
	}
	if err := os.WriteFile(*output, []byte(fmt.Sprintf("%d", n)), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, "line-counter:", err)
		os.Exit(1)
	}
}
