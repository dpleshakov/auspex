package esi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
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

// --- PostUniverseNames ---

func TestPostUniverseNames_ReturnsEntries(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/universe/names/" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`[{"id":60003760,"name":"Jita IV - Moon 4 - Caldari Navy Assembly Plant","category":"station"}]`))
	}))
	defer srv.Close()

	c := newTestClient(srv)
	entries, err := c.PostUniverseNames(context.Background(), []int64{60003760})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].ID != 60003760 {
		t.Errorf("ID: got %d, want 60003760", entries[0].ID)
	}
	if entries[0].Name != "Jita IV - Moon 4 - Caldari Navy Assembly Plant" {
		t.Errorf("Name: got %q", entries[0].Name)
	}
	if entries[0].Category != "station" {
		t.Errorf("Category: got %q, want station", entries[0].Category)
	}
}

func TestPostUniverseNames_SendsJSONBody(t *testing.T) {
	var gotBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`[]`))
	}))
	defer srv.Close()

	c := newTestClient(srv)
	_, err := c.PostUniverseNames(context.Background(), []int64{111, 222})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var ids []int64
	if err := json.Unmarshal(gotBody, &ids); err != nil {
		t.Fatalf("body is not valid JSON array: %v", err)
	}
	if len(ids) != 2 || ids[0] != 111 || ids[1] != 222 {
		t.Errorf("body: got %v, want [111 222]", ids)
	}
}

func TestPostUniverseNames_EmptyInput_ReturnsNil(t *testing.T) {
	// No server needed — empty input should short-circuit without an HTTP call.
	c := newTestClient(httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("unexpected HTTP call for empty input")
	})))

	entries, err := c.PostUniverseNames(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if entries != nil {
		t.Errorf("expected nil for empty input, got %v", entries)
	}
}

func TestPostUniverseNames_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"bad request"}`))
	}))
	defer srv.Close()

	c := newTestClient(srv)
	_, err := c.PostUniverseNames(context.Background(), []int64{1})
	if err == nil {
		t.Fatal("expected error for 400 response, got nil")
	}
}

// --- GetUniverseStructure ---

func TestGetUniverseStructure_ParsesResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/universe/structures/1234567890123/" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"name":"Perimeter - Tranquility Trading Tower","solar_system_id":30000144}`))
	}))
	defer srv.Close()

	c := newTestClient(srv)
	s, err := c.GetUniverseStructure(context.Background(), 1234567890123, "tok")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.Name != "Perimeter - Tranquility Trading Tower" {
		t.Errorf("Name: got %q", s.Name)
	}
	if s.SolarSystemID != 30000144 {
		t.Errorf("SolarSystemID: got %d, want 30000144", s.SolarSystemID)
	}
}

func TestGetUniverseStructure_Returns403AsForbidden(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"error":"forbidden"}`))
	}))
	defer srv.Close()

	c := newTestClient(srv)
	_, err := c.GetUniverseStructure(context.Background(), 1000000000001, "tok")
	if !errors.Is(err, ErrForbidden) {
		t.Errorf("expected ErrForbidden, got %v", err)
	}
}

func TestGetUniverseStructure_SendsAuthToken(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"name":"Test","solar_system_id":30000001}`))
	}))
	defer srv.Close()

	c := newTestClient(srv)
	_, err := c.GetUniverseStructure(context.Background(), 1000000000001, "mytoken")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotAuth != "Bearer mytoken" {
		t.Errorf("Authorization: got %q, want %q", gotAuth, "Bearer mytoken")
	}
}

// --- GetUniverseSystem ---

func TestGetUniverseSystem_ReturnsName(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/universe/systems/30000142/" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"name":"Jita","system_id":30000142}`))
	}))
	defer srv.Close()

	c := newTestClient(srv)
	name, err := c.GetUniverseSystem(context.Background(), 30000142)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if name != "Jita" {
		t.Errorf("name: got %q, want Jita", name)
	}
}

func TestGetUniverseSystem_NoTokenSent(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"name":"Jita"}`))
	}))
	defer srv.Close()

	c := newTestClient(srv)
	_, err := c.GetUniverseSystem(context.Background(), 30000142)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotAuth != "" {
		t.Errorf("expected no Authorization header, got %q", gotAuth)
	}
}

func TestGetUniverseSystem_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":"not found"}`))
	}))
	defer srv.Close()

	c := newTestClient(srv)
	_, err := c.GetUniverseSystem(context.Background(), 99999)
	if err == nil {
		t.Fatal("expected error for 404 response, got nil")
	}
}
