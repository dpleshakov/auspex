//go:build ignore

// check-coverage reads coverage.out (produced by go test -coverprofile),
// runs "go tool cover -func=coverage.out" to extract the total coverage
// percentage, and exits with code 1 if the total is below the threshold.
//
// Usage: go run tools/check-coverage.go [threshold]
//
// threshold defaults to 60 (percent). Pass an integer or decimal value.
package main

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

func main() {
	threshold := 60.0

	if len(os.Args) > 1 {
		t, err := strconv.ParseFloat(os.Args[1], 64)
		if err != nil {
			fmt.Fprintf(os.Stderr, "check-coverage: invalid threshold %q: %v\n", os.Args[1], err)
			os.Exit(2)
		}
		threshold = t
	}

	out, err := exec.Command("go", "tool", "cover", "-func=coverage.out").Output()
	if err != nil {
		fmt.Fprintf(os.Stderr, "check-coverage: go tool cover failed: %v\n", err)
		os.Exit(2)
	}

	total, err := parseTotalCoverage(string(out))
	if err != nil {
		fmt.Fprintf(os.Stderr, "check-coverage: %v\n", err)
		os.Exit(2)
	}

	if total < threshold {
		fmt.Fprintf(os.Stderr, "check-coverage: total coverage %.1f%% is below threshold %.1f%%\n", total, threshold)
		os.Exit(1)
	}

	fmt.Printf("check-coverage: total coverage %.1f%% >= threshold %.1f%% OK\n", total, threshold)
}

// parseTotalCoverage extracts the percentage from the "total:" line of
// "go tool cover -func" output, e.g.:
//
//	total:	(statements)	61.3%
func parseTotalCoverage(output string) (float64, error) {
	for _, line := range strings.Split(output, "\n") {
		if !strings.HasPrefix(line, "total:") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 3 {
			return 0, fmt.Errorf("unexpected total line format: %q", line)
		}
		pct := strings.TrimSuffix(fields[len(fields)-1], "%")
		v, err := strconv.ParseFloat(pct, 64)
		if err != nil {
			return 0, fmt.Errorf("cannot parse coverage percentage from %q: %v", line, err)
		}
		return v, nil
	}
	return 0, fmt.Errorf("no 'total:' line found in coverage output")
}
