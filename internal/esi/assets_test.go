package esi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestGetCorporationAssets_ParsesResponse(t *testing.T) {
	payload := `[
		{"item_id":1052718829566,"location_flag":"OfficeFolder","location_id":60015146,"location_type":"station"},
		{"item_id":9900000001,"location_flag":"Hangar","location_id":1052718829566,"location_type":"item"}
	]`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Pages", "1")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(payload))
	}))
	defer srv.Close()

	c := newTestClient(srv)
	assets, totalPages, _, err := c.GetCorporationAssets(context.Background(), 99000001, "tok", 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if totalPages != 1 {
		t.Errorf("totalPages: got %d, want 1", totalPages)
	}
	if len(assets) != 2 {
		t.Fatalf("expected 2 assets, got %d", len(assets))
	}
	if assets[0].ItemID != 1052718829566 {
		t.Errorf("ItemID: got %d, want 1052718829566", assets[0].ItemID)
	}
	if assets[0].LocationFlag != "OfficeFolder" {
		t.Errorf("LocationFlag: got %q, want OfficeFolder", assets[0].LocationFlag)
	}
	if assets[0].LocationID != 60015146 {
		t.Errorf("LocationID: got %d, want 60015146", assets[0].LocationID)
	}
	if assets[0].LocationType != "station" {
		t.Errorf("LocationType: got %q, want station", assets[0].LocationType)
	}
}

func TestGetCorporationAssets_ReadsXPages(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Pages", "5")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`[]`))
	}))
	defer srv.Close()

	c := newTestClient(srv)
	_, totalPages, _, err := c.GetCorporationAssets(context.Background(), 99000001, "tok", 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if totalPages != 5 {
		t.Errorf("totalPages: got %d, want 5", totalPages)
	}
}

func TestGetCorporationAssets_MissingXPages_DefaultsToOne(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// No X-Pages header.
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`[]`))
	}))
	defer srv.Close()

	c := newTestClient(srv)
	_, totalPages, _, err := c.GetCorporationAssets(context.Background(), 99000001, "tok", 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if totalPages != 1 {
		t.Errorf("totalPages: got %d, want 1 (default when header absent)", totalPages)
	}
}

func TestGetCorporationAssets_ReturnsCacheUntil(t *testing.T) {
	const expiresHeader = "Wed, 22 Feb 2026 12:00:00 GMT"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Pages", "1")
		w.Header().Set("Expires", expiresHeader)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`[]`))
	}))
	defer srv.Close()

	c := newTestClient(srv)
	_, _, cacheUntil, err := c.GetCorporationAssets(context.Background(), 99000001, "tok", 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want, _ := http.ParseTime(expiresHeader)
	if !cacheUntil.Equal(want) {
		t.Errorf("cacheUntil: got %v, want %v", cacheUntil, want)
	}
}

func TestGetCorporationAssets_SendsAuthToken(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("X-Pages", "1")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`[]`))
	}))
	defer srv.Close()

	c := newTestClient(srv)
	_, _, _, err := c.GetCorporationAssets(context.Background(), 99000001, "mytoken", 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotAuth != "Bearer mytoken" {
		t.Errorf("Authorization: got %q, want %q", gotAuth, "Bearer mytoken")
	}
}

func TestGetCorporationAssets_UsesCorrectURL(t *testing.T) {
	var gotPath, gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotQuery = r.URL.RawQuery
		w.Header().Set("X-Pages", "1")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`[]`))
	}))
	defer srv.Close()

	c := newTestClient(srv)
	_, _, _, err := c.GetCorporationAssets(context.Background(), 98765, "tok", 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(gotPath, "/corporations/98765/assets/") {
		t.Errorf("unexpected URL path: %q", gotPath)
	}
	if !strings.Contains(gotQuery, "page=3") {
		t.Errorf("expected page=3 in query, got %q", gotQuery)
	}
}

func TestGetCorporationAssets_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"error":"forbidden"}`))
	}))
	defer srv.Close()

	c := newTestClient(srv)
	_, _, _, err := c.GetCorporationAssets(context.Background(), 99000001, "tok", 1)
	if err == nil {
		t.Fatal("expected error for 403, got nil")
	}
}

func TestGetCorporationAssets_EmptyList(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Pages", "1")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`[]`))
	}))
	defer srv.Close()

	c := newTestClient(srv)
	assets, totalPages, cacheUntil, err := c.GetCorporationAssets(context.Background(), 99000001, "tok", 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(assets) != 0 {
		t.Errorf("expected empty slice, got %d items", len(assets))
	}
	if totalPages != 1 {
		t.Errorf("expected totalPages=1, got %d", totalPages)
	}
	if cacheUntil.IsZero() {
		t.Error("expected non-zero cacheUntil, got zero value")
	}
}

func TestGetCorporationAssets_MultiPage_SecondPage(t *testing.T) {
	// Page 2 returns different items; totalPages should still come from this response's X-Pages.
	payload := `[{"item_id":9900000042,"location_flag":"OfficeFolder","location_id":60003760,"location_type":"station"}]`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Pages", "3")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(payload))
	}))
	defer srv.Close()

	c := newTestClient(srv)
	assets, totalPages, _, err := c.GetCorporationAssets(context.Background(), 99000001, "tok", 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(assets) != 1 {
		t.Fatalf("expected 1 asset, got %d", len(assets))
	}
	if totalPages != 3 {
		t.Errorf("totalPages: got %d, want 3", totalPages)
	}
}

// --- GetStation ---

func TestGetStation_ReturnsName(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/universe/stations/60015146/" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"station_id":60015146,"name":"Ibura IX - Moon 11 - Spacelane Patrol Testing Facilities"}`))
	}))
	defer srv.Close()

	c := newTestClient(srv)
	name, err := c.GetStation(context.Background(), 60015146)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if name != "Ibura IX - Moon 11 - Spacelane Patrol Testing Facilities" {
		t.Errorf("name: got %q", name)
	}
}

func TestGetStation_NoTokenSent(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"station_id":60003760,"name":"Jita IV - Moon 4 - Caldari Navy Assembly Plant"}`))
	}))
	defer srv.Close()

	c := newTestClient(srv)
	_, err := c.GetStation(context.Background(), 60003760)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotAuth != "" {
		t.Errorf("expected no Authorization header for public endpoint, got %q", gotAuth)
	}
}

func TestGetStation_UsesCorrectURL(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"station_id":60015146,"name":"Test Station"}`))
	}))
	defer srv.Close()

	c := newTestClient(srv)
	_, err := c.GetStation(context.Background(), 60015146)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotPath != "/universe/stations/60015146/" {
		t.Errorf("unexpected URL path: %q", gotPath)
	}
}

func TestGetStation_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":"not found"}`))
	}))
	defer srv.Close()

	c := newTestClient(srv)
	_, err := c.GetStation(context.Background(), 99999)
	if err == nil {
		t.Fatal("expected error for 404 response, got nil")
	}
}

func TestGetStation_CacheUntilNotReturned(t *testing.T) {
	// GetStation only returns (string, error). Verify we get the name and no error
	// even when an Expires header is present — the cache time is intentionally discarded.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Expires", time.Now().Add(10*time.Minute).UTC().Format(http.TimeFormat))
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"station_id":60003760,"name":"Jita Station"}`))
	}))
	defer srv.Close()

	c := newTestClient(srv)
	name, err := c.GetStation(context.Background(), 60003760)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if name != "Jita Station" {
		t.Errorf("name: got %q, want Jita Station", name)
	}
}
