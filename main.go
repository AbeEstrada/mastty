package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"

	"github.com/AbeEstrada/tuit/tui"
)

func main() {
	os.Exit(run())
}

func run() int {
	closeLog := setupLogging()
	defer closeLog()

	app, err := tui.CreateApp()
	if err != nil {
		fmt.Fprintf(os.Stderr, "tuit: %v\n", err)
		return 1
	}

	err = app.Run()
	// Restore the terminal before printing anything so the message is readable.
	app.Close()
	if err != nil {
		fmt.Fprintf(os.Stderr, "tuit: %v\n", err)
		return 1
	}
	return 0
}

// setupLogging sends the standard logger to a debug file when TUIT_DEBUG is set
// and discards it otherwise. The file lives in the user cache directory
// (for example ~/Library/Caches/tuit/debug.log on macOS) so it never lands in
// the working directory.
func setupLogging() func() {
	if os.Getenv("TUIT_DEBUG") == "" {
		log.SetOutput(io.Discard)
		return func() {}
	}

	dir, err := os.UserCacheDir()
	if err != nil {
		dir = os.TempDir()
	}
	dir = filepath.Join(dir, "tuit")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		fmt.Fprintf(os.Stderr, "tuit: cannot create log directory: %v\n", err)
		log.SetOutput(io.Discard)
		return func() {}
	}

	path := filepath.Join(dir, "debug.log")
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		fmt.Fprintf(os.Stderr, "tuit: cannot open log file: %v\n", err)
		log.SetOutput(io.Discard)
		return func() {}
	}
	log.SetOutput(f)
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	return func() { f.Close() }
}
