//go:build ignore

// esi-dump fetches live data from the EVE ESI API and writes the parsed
// structs as JSON files into internal/esi/testdata/. Use it to refresh
// the fixture snapshots that unit tests compare against.
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
	"context"
	"encoding/json"
	"flag"
	"fmt"
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
	client := esi.NewClient(http.DefaultClient)

	if *doChar {
		token := requireEnv("ESI_ACCESS_TOKEN")
		charID := requireEnvID("ESI_CHARACTER_ID")

		bps, _, err := client.GetCharacterBlueprints(ctx, charID, token)
		if err != nil {
			fatalf("GetCharacterBlueprints: %v", err)
		}
		saveJSON(*outDir, "character_blueprints", bps)

		jobs, _, err := client.GetCharacterJobs(ctx, charID, token)
		if err != nil {
			fatalf("GetCharacterJobs: %v", err)
		}
		saveJSON(*outDir, "character_jobs", jobs)
	}

	if *doCorp {
		token := requireEnv("ESI_ACCESS_TOKEN")
		corpID := requireEnvID("ESI_CORPORATION_ID")

		bps, _, err := client.GetCorporationBlueprints(ctx, corpID, token)
		if err != nil {
			fatalf("GetCorporationBlueprints: %v", err)
		}
		saveJSON(*outDir, "corporation_blueprints", bps)

		jobs, _, err := client.GetCorporationJobs(ctx, corpID, token)
		if err != nil {
			fatalf("GetCorporationJobs: %v", err)
		}
		saveJSON(*outDir, "corporation_jobs", jobs)
	}

	if *typeID != 0 {
		ut, err := client.GetUniverseType(ctx, *typeID)
		if err != nil {
			fatalf("GetUniverseType(%d): %v", *typeID, err)
		}
		saveJSON(*outDir, fmt.Sprintf("universe_type_%d", *typeID), ut)
	}
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

// saveJSON marshals v to indented JSON and writes it to outDir/name.json.
func saveJSON(outDir, name string, v any) {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		fatalf("marshaling %s: %v", name, err)
	}
	path := outDir + "/" + name + ".json"
	if err := os.WriteFile(path, data, 0o644); err != nil {
		fatalf("writing %s: %v", path, err)
	}
	fmt.Printf("wrote %s (%d bytes)\n", path, len(data))
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "esi-dump: "+format+"\n", args...)
	os.Exit(1)
}
