//go:build ignore

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: touch path [path...]")
		os.Exit(1)
	}

	for _, path := range os.Args[1:] {
		// Create parent directories if needed (e.g. dist/ after rm -r)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			fmt.Fprintf(os.Stderr, "touch: %v\n", err)
			os.Exit(1)
		}

		f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			fmt.Fprintf(os.Stderr, "touch: %v\n", err)
			os.Exit(1)
		}
		f.Close()

		now := time.Now()
		if err := os.Chtimes(path, now, now); err != nil {
			fmt.Fprintf(os.Stderr, "touch: %v\n", err)
			os.Exit(1)
		}
	}
}
