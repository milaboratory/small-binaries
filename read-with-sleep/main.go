// This program reads file by filePath line by line and prints lines to stdout.
// It sleeps every sleepMs milliseconds between lines.
// Also, every percentageSleepMs it prints a percentage with a percentagePattern.
// This can help to test logs streaming and statuses from logs, like in MiXCR running.
package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"
)

func main() {
	ctx, cancelCtx := signal.NotifyContext(context.Background(), os.Interrupt, os.Kill, syscall.SIGTERM)
	defer cancelCtx()

	if len(os.Args) < 4 {
		fatal("Usage: %s <filePath> <percentagePattern> <sleepMs> <percentageSleepMs>", os.Args[0])
	}

	filePath := os.Args[1]
	percentagePattern := os.Args[2]
	sleepMs, err := strconv.Atoi(os.Args[3])
	if err != nil {
		fatal("cannot parse sleep ms: %q", os.Args[3])
	}
	percentageSleepMs, err := strconv.Atoi(os.Args[4])
	if err != nil {
		fatal("cannot parse percentage sleep ms: %q", os.Args[4])
	}

	f, err := os.Open(filePath)
	if err != nil {
		fatal("cannot open file %q: %v", filePath, err)
	}
	defer func() {
		if err := f.Close(); err != nil {
			fatal("cannot close file %q: %v", filePath, err)
		}
	}()
	scanner := bufio.NewScanner(f)

	stat, err := f.Stat()
	if err != nil {
		fatal("cannot read size of a file: %q, %v", filePath, err)
	}
	var bytesRead int64
	bytesTotal := stat.Size()
	if bytesTotal == 0 {
		fatal("size of a file is 0: %q", filePath)
	}

	wg := sync.WaitGroup{}
	wg.Add(1)
	defer wg.Wait()
	go func() {
		defer wg.Done()
		for {
			select {
			case <-ctx.Done():
				fmt.Printf("percentage goroutine is done\n")
				return
			case <-time.After(toDuration(percentageSleepMs)):
				percent := strconv.FormatFloat(float64(bytesRead)/float64(bytesTotal)*100, 'f', 2, 64)
				fmt.Printf("%s%s%% bytes read\n", percentagePattern, percent)
			}
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(toDuration(sleepMs)):
			if canContinue := scanner.Scan(); !canContinue {
				cancelCtx()
				return
			}
			fmt.Println(scanner.Text())
			bytesRead += int64(len(scanner.Bytes()))
		}
	}
}

func fatal(msg string, args ...any) {
	fmt.Fprintf(os.Stderr, msg+"\n", args...)
	os.Exit(1)
}

func toDuration(ms int) time.Duration {
	return time.Millisecond * time.Duration(ms)
}
