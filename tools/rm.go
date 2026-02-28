//go:build ignore

package main

import (
	"flag"
	"fmt"
	"os"
)

func main() {
	recursive := flag.Bool("r", false, "remove directories recursively")
	flag.Parse()

	args := flag.Args()
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: rm [-r] path [path...]")
		os.Exit(1)
	}

	for _, path := range args {
		var err error
		if *recursive {
			// RemoveAll returns nil for non-existent paths — no -f handling needed
			err = os.RemoveAll(path)
		} else {
			err = os.Remove(path)
			if os.IsNotExist(err) {
				err = nil // silent skip, like rm -f
			}
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "rm: %v\n", err)
			os.Exit(1)
		}
	}
}
