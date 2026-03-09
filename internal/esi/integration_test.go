//go:build integration

package esi_test

import (
	"context"
	"flag"
	"net/http"
	"os"
	"strconv"
	"testing"

	"github.com/dpleshakov/auspex/internal/esi"
)

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

func TestIntegration_GetCharacterBlueprints(t *testing.T) {
	token := mustToken(t)
	characterID := mustEnvID(t, "ESI_CHARACTER_ID")

	client := esi.NewClient(http.DefaultClient)
	bps, _, err := client.GetCharacterBlueprints(context.Background(), characterID, token)
	if err != nil {
		t.Fatalf("GetCharacterBlueprints: %v", err)
	}
	if len(bps) > 0 {
		if bps[0].ItemID == 0 {
			t.Error("GetCharacterBlueprints: bps[0].ItemID is 0")
		}
		if bps[0].TypeID == 0 {
			t.Error("GetCharacterBlueprints: bps[0].TypeID is 0")
		}
	}
}

func TestIntegration_GetCharacterJobs(t *testing.T) {
	token := mustToken(t)
	characterID := mustEnvID(t, "ESI_CHARACTER_ID")

	client := esi.NewClient(http.DefaultClient)
	_, _, err := client.GetCharacterJobs(context.Background(), characterID, token)
	if err != nil {
		t.Fatalf("GetCharacterJobs: %v", err)
	}
}

func TestIntegration_GetCorporationBlueprints(t *testing.T) {
	token := mustToken(t)
	corporationID := mustEnvID(t, "ESI_CORPORATION_ID")

	client := esi.NewClient(http.DefaultClient)
	_, _, err := client.GetCorporationBlueprints(context.Background(), corporationID, token)
	if err != nil {
		t.Fatalf("GetCorporationBlueprints: %v", err)
	}
}

func TestIntegration_GetCorporationJobs(t *testing.T) {
	token := mustToken(t)
	corporationID := mustEnvID(t, "ESI_CORPORATION_ID")

	client := esi.NewClient(http.DefaultClient)
	_, _, err := client.GetCorporationJobs(context.Background(), corporationID, token)
	if err != nil {
		t.Fatalf("GetCorporationJobs: %v", err)
	}
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
}
