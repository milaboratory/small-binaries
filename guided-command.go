package main

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	ctx, cancelCtx := signal.NotifyContext(context.Background(), os.Interrupt, os.Kill, syscall.SIGTERM)
	defer cancelCtx()

	if len(os.Args) < 4 {
		fmt.Fprintf(os.Stderr, "Usage: %s <startedPath> <unlockPath> <donePath> [<doneText>]", os.Args[0])
		os.Exit(1)
	}

	startedPath := os.Args[1]
	unlockPath := os.Args[2]
	donePath := os.Args[3]

	doneText := "done"
	if len(os.Args) == 5 {
		doneText = os.Args[4]
	}

	pidStr := fmt.Sprintf("%d", os.Getpid())
	err := writeFileAtomic(startedPath, []byte(pidStr), 0o666)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create 'i'm started' file %q: %v", startedPath, err)
		os.Exit(1)
	}

WaitForUnblockFile:
	for {
		select {
		case <-ctx.Done():
			err = os.Remove(startedPath)

			if err != nil {
				fmt.Fprintf(os.Stderr, "failed to remove 'i'm started' file %q after stop signal: %v", startedPath, err)
				os.Exit(1)
			}

			os.Exit(0)

		case <-time.After(10 * time.Millisecond):
			_, err = os.Stat(unlockPath)
			if err == nil {
				break WaitForUnblockFile
			}

			if errors.Is(err, os.ErrNotExist) {
				continue
			}

			fmt.Fprintf(os.Stderr, "failed wait for  'unlock' file %q: %v", unlockPath, err)
			os.Exit(1)
		}
	}

	err = writeFileAtomic(donePath, []byte(doneText), 0o666)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create 'i'm done' file %q: %v", donePath, err)
		os.Exit(1)
	}
}

func writeFileAtomic(filePath string, data []byte, mode os.FileMode) error {
	filePathTmp := fmt.Sprintf("%s.%s", filePath, randStr(5))

	err := os.WriteFile(filePathTmp, data, mode)
	if err != nil {
		return fmt.Errorf("failed to create intermediate tmp file for %q: %w", filePathTmp, err)
	}

	err = os.Rename(filePathTmp, filePath)
	if err != nil {
		return fmt.Errorf("failed to rename intermediate tmp file %q to %q: %w", filePathTmp, filePath, err)
	}

	return nil
}

func randStr(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

	variety := len(charset)

	ret := make([]byte, length)
	for i := range ret {
		ret[i] = charset[rand.Intn(variety)]
	}

	return string(ret)
}
