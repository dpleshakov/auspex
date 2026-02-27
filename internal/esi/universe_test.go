package esi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

// serveUniverse returns a handler that responds to the three universe endpoints
// for type_id=34 (Tritanium), group_id=18 (Mineral), category_id=4 (Material).
func serveUniverse(t *testing.T) http.Handler {
	t.Helper()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/universe/types/34":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"type_id":34,"name":"Tritanium","group_id":18}`))
		case "/universe/groups/18":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"group_id":18,"name":"Mineral","category_id":4}`))
		case "/universe/categories/4":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"category_id":4,"name":"Material"}`))
		default:
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"error":"not found"}`))
		}
	})
}

func TestGetUniverseType_ParsesChainedCalls(t *testing.T) {
	srv := httptest.NewServer(serveUniverse(t))
	defer srv.Close()

	c := newTestClient(srv)
	got, err := c.GetUniverseType(context.Background(), 34)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.TypeID != 34 {
		t.Errorf("TypeID: got %d, want 34", got.TypeID)
	}
	if got.TypeName != "Tritanium" {
		t.Errorf("TypeName: got %q, want Tritanium", got.TypeName)
	}
	if got.GroupID != 18 {
		t.Errorf("GroupID: got %d, want 18", got.GroupID)
	}
	if got.GroupName != "Mineral" {
		t.Errorf("GroupName: got %q, want Mineral", got.GroupName)
	}
	if got.CategoryID != 4 {
		t.Errorf("CategoryID: got %d, want 4", got.CategoryID)
	}
	if got.CategoryName != "Material" {
		t.Errorf("CategoryName: got %q, want Material", got.CategoryName)
	}
}

func TestGetUniverseType_NoTokenSent(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		switch r.URL.Path {
		case "/universe/types/34":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"type_id":34,"name":"Tritanium","group_id":18}`))
		case "/universe/groups/18":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"group_id":18,"name":"Mineral","category_id":4}`))
		case "/universe/categories/4":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"category_id":4,"name":"Material"}`))
		}
	}))
	defer srv.Close()

	c := newTestClient(srv)
	_, err := c.GetUniverseType(context.Background(), 34)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotAuth != "" {
		t.Errorf("expected no Authorization header for public endpoint, got %q", gotAuth)
	}
}

func TestGetUniverseType_TypeFetchError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":"type not found"}`))
	}))
	defer srv.Close()

	c := newTestClient(srv)
	_, err := c.GetUniverseType(context.Background(), 9999)
	if err == nil {
		t.Fatal("expected error when type fetch returns 404, got nil")
	}
}

func TestGetUniverseType_GroupFetchError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/universe/types/34":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"type_id":34,"name":"Tritanium","group_id":18}`))
		default:
			// group and category endpoints fail
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"error":"not found"}`))
		}
	}))
	defer srv.Close()

	c := newTestClient(srv)
	_, err := c.GetUniverseType(context.Background(), 34)
	if err == nil {
		t.Fatal("expected error when group fetch fails, got nil")
	}
}

func TestGetUniverseType_CategoryFetchError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/universe/types/34":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"type_id":34,"name":"Tritanium","group_id":18}`))
		case "/universe/groups/18":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"group_id":18,"name":"Mineral","category_id":4}`))
		default:
			// category endpoint fails
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"error":"not found"}`))
		}
	}))
	defer srv.Close()

	c := newTestClient(srv)
	_, err := c.GetUniverseType(context.Background(), 34)
	if err == nil {
		t.Fatal("expected error when category fetch fails, got nil")
	}
}
