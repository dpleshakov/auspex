package esi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGetCorporationOffices_ParsesResponse(t *testing.T) {
	payload := `[{"office_id":1052718829566,"station_id":60004588}]`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(payload))
	}))
	defer srv.Close()

	c := newTestClient(srv)
	offices, err := c.GetCorporationOffices(context.Background(), 123, "tok")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(offices) != 1 {
		t.Fatalf("expected 1 office, got %d", len(offices))
	}
	if offices[0].OfficeID != 1052718829566 {
		t.Errorf("OfficeID: got %d, want 1052718829566", offices[0].OfficeID)
	}
	if offices[0].StationID != 60004588 {
		t.Errorf("StationID: got %d, want 60004588", offices[0].StationID)
	}
}

func TestGetCorporationOffices_SendsAuthToken(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`[]`))
	}))
	defer srv.Close()

	c := newTestClient(srv)
	_, err := c.GetCorporationOffices(context.Background(), 123, "mytoken")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotAuth != "Bearer mytoken" {
		t.Errorf("Authorization: got %q, want %q", gotAuth, "Bearer mytoken")
	}
}

func TestGetCorporationOffices_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"error":"forbidden"}`))
	}))
	defer srv.Close()

	c := newTestClient(srv)
	_, err := c.GetCorporationOffices(context.Background(), 123, "tok")
	if err == nil {
		t.Fatal("expected error for 403, got nil")
	}
}

func TestGetCorporationOffices_UsesCorrectURL(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`[]`))
	}))
	defer srv.Close()

	c := newTestClient(srv)
	_, err := c.GetCorporationOffices(context.Background(), 98765, "tok")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(gotPath, "/corporations/98765/offices/") {
		t.Errorf("unexpected URL path: %q", gotPath)
	}
}
