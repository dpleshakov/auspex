//go:build ignore

package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: release-notes <version>")
		os.Exit(1)
	}
	version := os.Args[1]
	header := "## [" + version + "]"

	f, err := os.Open("CHANGELOG.md")
	if err != nil {
		fmt.Fprintf(os.Stderr, "release-notes: cannot open CHANGELOG.md: %v\n", err)
		os.Exit(1)
	}
	defer f.Close()

	var lines []string
	inSection := false
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "## [") {
			if inSection {
				break
			}
			if strings.HasPrefix(line, header) {
				inSection = true
			}
			continue
		}
		if inSection {
			lines = append(lines, line)
		}
	}
	if err := scanner.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "release-notes: read error: %v\n", err)
		os.Exit(1)
	}
	if !inSection {
		fmt.Fprintf(os.Stderr, "release-notes: section %q not found in CHANGELOG.md\n", header)
		os.Exit(1)
	}

	// Trim leading and trailing blank lines.
	for len(lines) > 0 && strings.TrimSpace(lines[0]) == "" {
		lines = lines[1:]
	}
	for len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
		lines = lines[:len(lines)-1]
	}

	if err := os.MkdirAll("docs", 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "release-notes: cannot create docs/: %v\n", err)
		os.Exit(1)
	}
	out, err := os.Create("docs/release-notes.md")
	if err != nil {
		fmt.Fprintf(os.Stderr, "release-notes: cannot write docs/release-notes.md: %v\n", err)
		os.Exit(1)
	}
	defer out.Close()

	for _, line := range lines {
		fmt.Fprintln(out, line)
	}
}
