//go:build integration

package esi_test

import (
	"encoding/json"
	"flag"
	"os"
	"strconv"
	"testing"

	"github.com/google/go-cmp/cmp"
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
