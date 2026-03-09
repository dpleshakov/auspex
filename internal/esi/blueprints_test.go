package esi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
)

func TestGetCharacterBlueprints_ParsesResponse(t *testing.T) {
	payload := `[
		{"item_id":1,"type_id":100,"location_id":60000004,"material_efficiency":10,"time_efficiency":20,"quantity":-1},
		{"item_id":2,"type_id":200,"location_id":60000005,"material_efficiency":5,"time_efficiency":12,"quantity":-1}
	]`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(payload))
	}))
	defer srv.Close()

	c := newTestClient(srv)
	bps, _, err := c.GetCharacterBlueprints(context.Background(), 12345, "tok")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(bps) != 2 {
		t.Fatalf("expected 2 blueprints, got %d", len(bps))
	}
	if bps[0].ItemID != 1 || bps[0].TypeID != 100 || bps[0].MELevel != 10 || bps[0].TELevel != 20 || bps[0].LocationID != 60000004 {
		t.Errorf("blueprint[0] mismatch: %+v", bps[0])
	}
}

func TestGetCharacterBlueprints_FiltersBPCs(t *testing.T) {
	payload := `[
		{"item_id":1,"type_id":100,"location_id":60000004,"material_efficiency":10,"time_efficiency":20,"quantity":-1},
		{"item_id":2,"type_id":200,"location_id":60000004,"material_efficiency":0,"time_efficiency":0,"quantity":5},
		{"item_id":3,"type_id":300,"location_id":60000004,"material_efficiency":0,"time_efficiency":0,"quantity":10}
	]`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(payload))
	}))
	defer srv.Close()

	c := newTestClient(srv)
	bps, _, err := c.GetCharacterBlueprints(context.Background(), 12345, "tok")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(bps) != 1 {
		t.Fatalf("expected 1 BPO (BPCs filtered), got %d", len(bps))
	}
	if bps[0].ItemID != 1 {
		t.Errorf("wrong item kept: got item_id %d, want 1", bps[0].ItemID)
	}
}

func TestGetCharacterBlueprints_EmptyList(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`[]`))
	}))
	defer srv.Close()

	c := newTestClient(srv)
	bps, _, err := c.GetCharacterBlueprints(context.Background(), 12345, "tok")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(bps) != 0 {
		t.Errorf("expected empty slice, got %d items", len(bps))
	}
}

func TestGetCharacterBlueprints_UsesCorrectURL(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`[]`))
	}))
	defer srv.Close()

	c := newTestClient(srv)
	_, _, err := c.GetCharacterBlueprints(context.Background(), 42, "tok")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(gotPath, "/characters/42/blueprints") {
		t.Errorf("unexpected URL path: %q", gotPath)
	}
}

func TestGetCorporationBlueprints_FiltersBPCs(t *testing.T) {
	payload := `[
		{"item_id":10,"type_id":100,"location_id":60000004,"material_efficiency":8,"time_efficiency":16,"quantity":-1},
		{"item_id":11,"type_id":200,"location_id":60000004,"material_efficiency":0,"time_efficiency":0,"quantity":3}
	]`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(payload))
	}))
	defer srv.Close()

	c := newTestClient(srv)
	bps, _, err := c.GetCorporationBlueprints(context.Background(), 99999, "tok")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(bps) != 1 || bps[0].ItemID != 10 {
		t.Errorf("expected 1 BPO with item_id 10, got %+v", bps)
	}
}

func TestGetCharacterBlueprints_ParsesLocationFlag(t *testing.T) {
	payload := `[
		{"item_id":1,"type_id":100,"location_id":1052718829566,"location_flag":"CorpSAG1","material_efficiency":10,"time_efficiency":20,"quantity":-1},
		{"item_id":2,"type_id":200,"location_id":60000004,"location_flag":"Hangar","material_efficiency":5,"time_efficiency":12,"quantity":-1}
	]`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(payload))
	}))
	defer srv.Close()

	c := newTestClient(srv)
	bps, _, err := c.GetCharacterBlueprints(context.Background(), 12345, "tok")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(bps) != 2 {
		t.Fatalf("expected 2 blueprints, got %d", len(bps))
	}
	if bps[0].LocationFlag != "CorpSAG1" {
		t.Errorf("blueprint[0] LocationFlag: got %q, want CorpSAG1", bps[0].LocationFlag)
	}
	if bps[1].LocationFlag != "Hangar" {
		t.Errorf("blueprint[1] LocationFlag: got %q, want Hangar", bps[1].LocationFlag)
	}
}

func TestGetCorporationBlueprints_UsesCorrectURL(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`[]`))
	}))
	defer srv.Close()

	c := newTestClient(srv)
	_, _, err := c.GetCorporationBlueprints(context.Background(), 99999, "tok")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(gotPath, "/corporations/99999/blueprints") {
		t.Errorf("unexpected URL path: %q", gotPath)
	}
}

func TestGetCharacterBlueprints_AllBPCsReturnsEmpty(t *testing.T) {
	payload := `[
		{"item_id":1,"type_id":100,"location_id":60000004,"material_efficiency":0,"time_efficiency":0,"quantity":5},
		{"item_id":2,"type_id":200,"location_id":60000004,"material_efficiency":0,"time_efficiency":0,"quantity":10}
	]`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(payload))
	}))
	defer srv.Close()

	c := newTestClient(srv)
	bps, _, err := c.GetCharacterBlueprints(context.Background(), 12345, "tok")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(bps) != 0 {
		t.Errorf("expected empty slice (all BPCs filtered), got %d items", len(bps))
	}
}

func TestGetCharacterBlueprints_LocationFlagAbsent(t *testing.T) {
	// location_flag field is omitted — Go should decode it as the zero value "".
	payload := `[{"item_id":1,"type_id":100,"location_id":60000004,"material_efficiency":10,"time_efficiency":20,"quantity":-1}]`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(payload))
	}))
	defer srv.Close()

	c := newTestClient(srv)
	bps, _, err := c.GetCharacterBlueprints(context.Background(), 12345, "tok")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(bps) != 1 {
		t.Fatalf("expected 1 blueprint, got %d", len(bps))
	}
	if bps[0].LocationFlag != "" {
		t.Errorf("LocationFlag: got %q, want empty string", bps[0].LocationFlag)
	}
}

func TestGetCharacterBlueprints_ZeroMETE(t *testing.T) {
	// A BPO with ME=0 and TE=0 must survive the BPC filter (quantity=-1) and parse to 0.
	payload := `[{"item_id":1,"type_id":100,"location_id":60000004,"material_efficiency":0,"time_efficiency":0,"quantity":-1}]`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(payload))
	}))
	defer srv.Close()

	c := newTestClient(srv)
	bps, _, err := c.GetCharacterBlueprints(context.Background(), 12345, "tok")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(bps) != 1 {
		t.Fatalf("expected 1 BPO, got %d", len(bps))
	}
	if bps[0].MELevel != 0 {
		t.Errorf("MELevel: got %d, want 0", bps[0].MELevel)
	}
	if bps[0].TELevel != 0 {
		t.Errorf("TELevel: got %d, want 0", bps[0].TELevel)
	}
}

// --- fixture-based full ESI response tests ---

func TestGetCharacterBlueprints_FullESIResponse(t *testing.T) {
	data, err := os.ReadFile("testdata/character_blueprints.json")
	if err != nil {
		t.Fatalf("reading fixture: %v", err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(data)
	}))
	defer srv.Close()

	c := newTestClient(srv)
	bps, _, err := c.GetCharacterBlueprints(context.Background(), 12345, "tok")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Fixture has 2 BPOs + 1 BPC; only BPOs survive the filter.
	if len(bps) != 2 {
		t.Fatalf("expected 2 BPOs (BPC filtered), got %d", len(bps))
	}
	bp := bps[0]
	if bp.ItemID != 1052548709012 {
		t.Errorf("ItemID: got %d, want 1052548709012", bp.ItemID)
	}
	if bp.TypeID != 15511 {
		t.Errorf("TypeID: got %d, want 15511", bp.TypeID)
	}
	if bp.MELevel != 10 {
		t.Errorf("MELevel: got %d, want 10", bp.MELevel)
	}
	if bp.TELevel != 20 {
		t.Errorf("TELevel: got %d, want 20", bp.TELevel)
	}
	if bp.LocationID != 60003997 {
		t.Errorf("LocationID: got %d, want 60003997", bp.LocationID)
	}
	if bp.LocationFlag != "Hangar" {
		t.Errorf("LocationFlag: got %q, want Hangar", bp.LocationFlag)
	}
}

func TestGetCorporationBlueprints_FullESIResponse(t *testing.T) {
	data, err := os.ReadFile("testdata/corporation_blueprints.json")
	if err != nil {
		t.Fatalf("reading fixture: %v", err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(data)
	}))
	defer srv.Close()

	c := newTestClient(srv)
	bps, _, err := c.GetCorporationBlueprints(context.Background(), 99999, "tok")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(bps) != 2 {
		t.Fatalf("expected 2 BPOs, got %d", len(bps))
	}
	bp := bps[0]
	if bp.ItemID != 1052525096791 {
		t.Errorf("ItemID: got %d, want 1052525096791", bp.ItemID)
	}
	if bp.TypeID != 2204 {
		t.Errorf("TypeID: got %d, want 2204", bp.TypeID)
	}
	if bp.MELevel != 10 {
		t.Errorf("MELevel: got %d, want 10", bp.MELevel)
	}
	if bp.TELevel != 20 {
		t.Errorf("TELevel: got %d, want 20", bp.TELevel)
	}
	if bp.LocationID != 1052718829566 {
		t.Errorf("LocationID: got %d, want 1052718829566", bp.LocationID)
	}
	if bp.LocationFlag != "CorpSAG3" {
		t.Errorf("LocationFlag: got %q, want CorpSAG3", bp.LocationFlag)
	}
}

// --- parseXPages ---

func TestParseXPages(t *testing.T) {
	tests := []struct {
		name string
		s    string
		want int
	}{
		{"valid integer", "3", 3},
		{"one", "1", 1},
		{"absent", "", 1},
		{"malformed", "abc", 1},
		{"zero", "0", 1},
		{"negative", "-1", 1},
		{"at cap", "40", 40},
		{"exceeds cap", "9999", 40},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := parseXPages(tc.s); got != tc.want {
				t.Errorf("parseXPages(%q) = %d, want %d", tc.s, got, tc.want)
			}
		})
	}
}

// --- pagination ---

func TestGetCharacterBlueprints_MultiPage(t *testing.T) {
	page1, err := os.ReadFile("testdata/character_blueprints_page1.json")
	if err != nil {
		t.Fatalf("reading page1 fixture: %v", err)
	}
	page2, err := os.ReadFile("testdata/character_blueprints_page2.json")
	if err != nil {
		t.Fatalf("reading page2 fixture: %v", err)
	}

	var requestCount atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)
		w.Header().Set("X-Pages", "2")
		if r.URL.Query().Get("page") == "2" {
			_, _ = w.Write(page2)
		} else {
			_, _ = w.Write(page1)
		}
	}))
	defer srv.Close()

	c := newTestClient(srv)
	bps, _, err := c.GetCharacterBlueprints(context.Background(), 12345, "tok")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(bps) != 2 {
		t.Fatalf("expected 2 BPOs, got %d", len(bps))
	}
	if bps[0].ItemID != 1052548709012 {
		t.Errorf("bps[0].ItemID: got %d, want 1052548709012", bps[0].ItemID)
	}
	if bps[1].ItemID != 1052548712662 {
		t.Errorf("bps[1].ItemID: got %d, want 1052548712662", bps[1].ItemID)
	}
	if got := requestCount.Load(); got != 2 {
		t.Errorf("expected 2 HTTP requests (page 1 + page 2), got %d", got)
	}
}

func TestGetCharacterBlueprints_XPagesAbsent(t *testing.T) {
	var requestCount atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)
		// No X-Pages header — single-page response.
		_, _ = w.Write([]byte(`[{"item_id":1,"type_id":100,"location_id":60000004,"material_efficiency":10,"time_efficiency":20,"quantity":-1}]`))
	}))
	defer srv.Close()

	c := newTestClient(srv)
	bps, _, err := c.GetCharacterBlueprints(context.Background(), 12345, "tok")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(bps) != 1 {
		t.Errorf("expected 1 BPO, got %d", len(bps))
	}
	if got := requestCount.Load(); got != 1 {
		t.Errorf("expected exactly 1 HTTP request, got %d", got)
	}
}

func TestGetCharacterBlueprints_XPagesOne(t *testing.T) {
	var requestCount atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)
		w.Header().Set("X-Pages", "1")
		_, _ = w.Write([]byte(`[{"item_id":1,"type_id":100,"location_id":60000004,"material_efficiency":10,"time_efficiency":20,"quantity":-1}]`))
	}))
	defer srv.Close()

	c := newTestClient(srv)
	bps, _, err := c.GetCharacterBlueprints(context.Background(), 12345, "tok")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(bps) != 1 {
		t.Errorf("expected 1 BPO, got %d", len(bps))
	}
	if got := requestCount.Load(); got != 1 {
		t.Errorf("expected exactly 1 HTTP request, got %d", got)
	}
}

func TestGetCorporationBlueprints_MultiPage(t *testing.T) {
	page1, err := os.ReadFile("testdata/corporation_blueprints_page1.json")
	if err != nil {
		t.Fatalf("reading page1 fixture: %v", err)
	}
	page2, err := os.ReadFile("testdata/corporation_blueprints_page2.json")
	if err != nil {
		t.Fatalf("reading page2 fixture: %v", err)
	}

	var requestCount atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)
		w.Header().Set("X-Pages", "2")
		if r.URL.Query().Get("page") == "2" {
			_, _ = w.Write(page2)
		} else {
			_, _ = w.Write(page1)
		}
	}))
	defer srv.Close()

	c := newTestClient(srv)
	bps, _, err := c.GetCorporationBlueprints(context.Background(), 99999, "tok")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(bps) != 2 {
		t.Fatalf("expected 2 BPOs, got %d", len(bps))
	}
	if bps[0].ItemID != 1052525096791 {
		t.Errorf("bps[0].ItemID: got %d, want 1052525096791", bps[0].ItemID)
	}
	if bps[1].ItemID != 1052525097804 {
		t.Errorf("bps[1].ItemID: got %d, want 1052525097804", bps[1].ItemID)
	}
	if got := requestCount.Load(); got != 2 {
		t.Errorf("expected 2 HTTP requests (page 1 + page 2), got %d", got)
	}
}
