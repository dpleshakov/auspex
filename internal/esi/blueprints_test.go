package esi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
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
