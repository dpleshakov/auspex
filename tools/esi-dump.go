//go:build ignore

// esi-dump fetches live data from the EVE ESI API and writes the raw ESI
// responses as pretty-printed JSON files into internal/esi/testdata/.
// Use it to refresh the fixture snapshots that unit tests compare against.
//
// Usage:
//
//	ESI_ACCESS_TOKEN=<token> ESI_CHARACTER_ID=<id> ESI_CORPORATION_ID=<id> \
//	  go run tools/esi-dump.go [flags]
//
// Flags:
//
//	-out   directory to write JSON files into (default: internal/esi/testdata)
//	-char  fetch character endpoints (blueprints, jobs); requires ESI_CHARACTER_ID
//	-corp  fetch corporation endpoints (blueprints, jobs); requires ESI_CORPORATION_ID
//	-type  type ID to fetch from /universe/types/:id (default: 34 = Tritanium)
//
// With no flags set, all endpoints are attempted. Individual flags enable
// only the selected subsets.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"

	"github.com/dpleshakov/auspex/internal/esi"
)

func main() {
	outDir := flag.String("out", "internal/esi/testdata", "output directory for JSON fixtures")
	doChar := flag.Bool("char", false, "fetch character blueprints and jobs")
	doCorp := flag.Bool("corp", false, "fetch corporation blueprints and jobs")
	typeID := flag.Int64("type", 34, "type ID to fetch from /universe/types/:id (0 = skip)")
	flag.Parse()

	// If no specific subset was requested, enable all.
	all := !*doChar && !*doCorp
	if all {
		*doChar = true
		*doCorp = true
	}

	if err := os.MkdirAll(*outDir, 0o755); err != nil {
		fatalf("creating output directory %s: %v", *outDir, err)
	}

	ctx := context.Background()

	if *doChar {
		token := requireEnv("ESI_ACCESS_TOKEN")
		charID := requireEnvID("ESI_CHARACTER_ID")

		fetchAndSave(ctx, *outDir, "character_blueprints",
			fmt.Sprintf("%s/characters/%d/blueprints", esi.BaseURL, charID), token)
		fetchAndSave(ctx, *outDir, "character_jobs",
			fmt.Sprintf("%s/characters/%d/industry/jobs", esi.BaseURL, charID), token)
	}

	if *doCorp {
		token := requireEnv("ESI_ACCESS_TOKEN")
		corpID := requireEnvID("ESI_CORPORATION_ID")

		fetchAndSave(ctx, *outDir, "corporation_blueprints",
			fmt.Sprintf("%s/corporations/%d/blueprints", esi.BaseURL, corpID), token)
		fetchAndSave(ctx, *outDir, "corporation_jobs",
			fmt.Sprintf("%s/corporations/%d/industry/jobs", esi.BaseURL, corpID), token)
	}

	if *typeID != 0 {
		fetchAndSave(ctx, *outDir, fmt.Sprintf("universe_type_%d", *typeID),
			fmt.Sprintf("%s/universe/types/%d/", esi.BaseURL, *typeID), "")
	}
}

// fetchAndSave performs a raw GET request and writes the pretty-printed ESI
// response body to outDir/name.json. The file preserves the original ESI
// field names (snake_case) without any Go struct transformation.
func fetchAndSave(ctx context.Context, outDir, name, url, token string) {
	raw, err := fetchRaw(ctx, url, token)
	if err != nil {
		fatalf("fetching %s: %v", name, err)
	}
	var buf bytes.Buffer
	if err := json.Indent(&buf, raw, "", "  "); err != nil {
		fatalf("formatting %s: %v", name, err)
	}
	buf.WriteByte('\n')
	path := outDir + "/" + name + ".json"
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		fatalf("writing %s: %v", path, err)
	}
	fmt.Printf("wrote %s (%d bytes)\n", path, buf.Len())
}

// fetchRaw performs a GET request with an optional Bearer token and returns
// the raw response body.
func fetchRaw(ctx context.Context, url, token string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("building request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("sending request: %w", err)
	}
	body, err := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("ESI status %d: %s", resp.StatusCode, body)
	}
	return body, nil
}

// requireEnv returns the value of the named environment variable or exits.
func requireEnv(name string) string {
	v := os.Getenv(name)
	if v == "" {
		fatalf("%s is not set", name)
	}
	return v
}

// requireEnvID parses the named environment variable as int64 or exits.
func requireEnvID(name string) int64 {
	raw := requireEnv(name)
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		fatalf("%s is not a valid int64: %v", name, err)
	}
	return id
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "esi-dump: "+format+"\n", args...)
	os.Exit(1)
}
