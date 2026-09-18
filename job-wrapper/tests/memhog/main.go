// memhog is a test workload for job-wrapper: it allocates and touches memory, optionally burns
// CPU, optionally leaves an orphan behind, holds for a while, then exits with a chosen code.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"time"
)

func main() {
	mb := flag.Int("mb", 0, "MiB to allocate and touch")
	hold := flag.Duration("hold", 0, "how long to hold the memory before exiting")
	spin := flag.Bool("spin", false, "burn one CPU while holding")
	orphan := flag.Bool("orphan", false, "spawn `sh -c 'sleep 0.2 &'`, leaving an orphan for PID 1 to reap")
	exitCode := flag.Int("exit", 0, "exit code")
	flag.Parse()

	fmt.Printf("memhog: pid=%d mb=%d hold=%s spin=%v\n", os.Getpid(), *mb, *hold, *spin)

	if *orphan {
		cmd := exec.CommandContext(context.Background(), "sh", "-c", "sleep 0.2 &")
		err := cmd.Run()
		if err != nil {
			fmt.Fprintln(os.Stderr, "memhog: orphan spawn:", err)
		}
	}

	var block []byte
	if *mb > 0 {
		block = make([]byte, *mb<<20)
		for i := 0; i < len(block); i += 4096 {
			block[i] = 1
		}
		fmt.Printf("memhog: touched %d MiB\n", *mb)
	}

	deadline := time.Now().Add(*hold)
	if *spin {
		x := uint64(0)
		for time.Now().Before(deadline) {
			for range 1_000_000 {
				x = x*6364136223846793005 + 1442695040888963407
			}
		}
		_ = x
	} else if *hold > 0 {
		time.Sleep(*hold)
	}
	runtime.KeepAlive(block)

	fmt.Fprintf(os.Stderr, "memhog: exiting with %d\n", *exitCode)
	os.Exit(*exitCode)
}
