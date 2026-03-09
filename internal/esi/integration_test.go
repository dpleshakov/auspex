//go:build integration

package esi_test

import (
	"context"
	"encoding/json"
	"flag"
	"net/http"
	"os"
	"strconv"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/dpleshakov/auspex/internal/esi"
)

var update = flag.Bool("update", false, "overwrite golden fixtures")

func TestMain(m *testing.M) {
	flag.Parse()
	os.Exit(m.Run())
}

// mustToken reads ESI_ACCESS_TOKEN from the environment.
// The test is skipped if the variable is empty.
func mustToken(t *testing.T) string {
	t.Helper()
	tok := os.Getenv("ESI_ACCESS_TOKEN")
	if tok == "" {
		t.Skip("ESI_ACCESS_TOKEN not set")
	}
	return tok
}

// mustEnvID reads a named environment variable and parses it as int64.
// The test is skipped if the variable is absent or cannot be parsed.
func mustEnvID(t *testing.T, name string) int64 {
	t.Helper()
	raw := os.Getenv(name)
	if raw == "" {
		t.Skipf("%s not set", name)
	}
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		t.Skipf("%s is not a valid int64: %v", name, err)
	}
	return id
}

// saveFixture marshals v to indented JSON and writes it to testdata/{name}.json.
func saveFixture(t *testing.T, name string, v any) {
	t.Helper()
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		t.Fatalf("saveFixture: marshaling %s: %v", name, err)
	}
	path := "testdata/" + name + ".json"
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("saveFixture: writing %s: %v", path, err)
	}
}

// compareFixture either updates the golden fixture (when -update is set) or
// reads the existing fixture and compares it with got using cmp.Diff.
// Comparison is on the parsed Go struct, so extra ESI fields not mapped to
// struct fields are silently ignored.
func compareFixture[T any](t *testing.T, name string, got T) {
	t.Helper()
	if *update {
		saveFixture(t, name, got)
		return
	}
	data, err := os.ReadFile("testdata/" + name + ".json")
	if err != nil {
		t.Fatalf("compareFixture: reading fixture %s: %v", name, err)
	}
	var want T
	if err := json.Unmarshal(data, &want); err != nil {
		t.Fatalf("compareFixture: unmarshaling fixture %s: %v", name, err)
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("compareFixture %s mismatch (-want +got):\n%s", name, diff)
	}
}

func TestIntegration_GetCharacterBlueprints(t *testing.T) {
	token := mustToken(t)
	characterID := mustEnvID(t, "ESI_CHARACTER_ID")

	client := esi.NewClient(http.DefaultClient)
	bps, _, err := client.GetCharacterBlueprints(context.Background(), characterID, token)
	if err != nil {
		t.Fatalf("GetCharacterBlueprints: %v", err)
	}
	if len(bps) == 0 {
		t.Fatal("GetCharacterBlueprints: expected non-empty slice")
	}
	if bps[0].ItemID == 0 {
		t.Error("GetCharacterBlueprints: bps[0].ItemID is 0")
	}
	if bps[0].TypeID == 0 {
		t.Error("GetCharacterBlueprints: bps[0].TypeID is 0")
	}
	compareFixture(t, "character_blueprints", bps)
}

func TestIntegration_GetCharacterJobs(t *testing.T) {
	token := mustToken(t)
	characterID := mustEnvID(t, "ESI_CHARACTER_ID")

	client := esi.NewClient(http.DefaultClient)
	jobs, _, err := client.GetCharacterJobs(context.Background(), characterID, token)
	if err != nil {
		t.Fatalf("GetCharacterJobs: %v", err)
	}
	compareFixture(t, "character_jobs", jobs)
}

func TestIntegration_GetCorporationBlueprints(t *testing.T) {
	token := mustToken(t)
	corporationID := mustEnvID(t, "ESI_CORPORATION_ID")

	client := esi.NewClient(http.DefaultClient)
	bps, _, err := client.GetCorporationBlueprints(context.Background(), corporationID, token)
	if err != nil {
		t.Fatalf("GetCorporationBlueprints: %v", err)
	}
	compareFixture(t, "corporation_blueprints", bps)
}

func TestIntegration_GetCorporationJobs(t *testing.T) {
	token := mustToken(t)
	corporationID := mustEnvID(t, "ESI_CORPORATION_ID")

	client := esi.NewClient(http.DefaultClient)
	jobs, _, err := client.GetCorporationJobs(context.Background(), corporationID, token)
	if err != nil {
		t.Fatalf("GetCorporationJobs: %v", err)
	}
	compareFixture(t, "corporation_jobs", jobs)
}

func TestIntegration_GetUniverseType(t *testing.T) {
	const typeID = 34 // Tritanium — stable, well-known item

	client := esi.NewClient(http.DefaultClient)
	ut, err := client.GetUniverseType(context.Background(), typeID)
	if err != nil {
		t.Fatalf("GetUniverseType: %v", err)
	}
	if ut.TypeName == "" {
		t.Error("GetUniverseType: TypeName is empty")
	}
	if ut.CategoryID == 0 {
		t.Error("GetUniverseType: CategoryID is 0")
	}
	compareFixture(t, "universe_type_34", ut)
}
