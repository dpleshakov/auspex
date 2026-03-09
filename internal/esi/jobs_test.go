package esi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestGetCharacterJobs_FiltersByStatus(t *testing.T) {
	payload := `[
		{"job_id":1,"blueprint_id":100,"installer_id":42,"activity_id":4,"status":"active","start_date":"2026-02-20T10:00:00Z","end_date":"2026-02-22T10:00:00Z"},
		{"job_id":2,"blueprint_id":200,"installer_id":42,"activity_id":4,"status":"ready","start_date":"2026-02-19T10:00:00Z","end_date":"2026-02-21T10:00:00Z"},
		{"job_id":3,"blueprint_id":300,"installer_id":42,"activity_id":4,"status":"delivered","start_date":"2026-02-18T10:00:00Z","end_date":"2026-02-20T10:00:00Z"},
		{"job_id":4,"blueprint_id":400,"installer_id":42,"activity_id":4,"status":"canceled","start_date":"2026-02-17T10:00:00Z","end_date":"2026-02-19T10:00:00Z"}
	]`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(payload))
	}))
	defer srv.Close()

	c := newTestClient(srv)
	jobs, _, err := c.GetCharacterJobs(context.Background(), 42, "tok")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(jobs) != 2 {
		t.Errorf("expected 2 jobs (active+ready only), got %d", len(jobs))
	}
}

func TestGetCharacterJobs_FiltersByActivity(t *testing.T) {
	payload := `[
		{"job_id":1,"blueprint_id":100,"installer_id":42,"activity_id":4,"status":"active","start_date":"2026-02-20T10:00:00Z","end_date":"2026-02-22T10:00:00Z"},
		{"job_id":2,"blueprint_id":200,"installer_id":42,"activity_id":3,"status":"active","start_date":"2026-02-20T10:00:00Z","end_date":"2026-02-22T10:00:00Z"},
		{"job_id":3,"blueprint_id":300,"installer_id":42,"activity_id":5,"status":"active","start_date":"2026-02-20T10:00:00Z","end_date":"2026-02-22T10:00:00Z"},
		{"job_id":4,"blueprint_id":400,"installer_id":42,"activity_id":1,"status":"active","start_date":"2026-02-20T10:00:00Z","end_date":"2026-02-22T10:00:00Z"},
		{"job_id":5,"blueprint_id":500,"installer_id":42,"activity_id":8,"status":"active","start_date":"2026-02-20T10:00:00Z","end_date":"2026-02-22T10:00:00Z"}
	]`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(payload))
	}))
	defer srv.Close()

	c := newTestClient(srv)
	jobs, _, err := c.GetCharacterJobs(context.Background(), 42, "tok")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// activity_id 4 (ME), 3 (TE), 5 (copying) kept; 1 (manufacturing) and 8 (invention) skipped.
	if len(jobs) != 3 {
		t.Errorf("expected 3 jobs (ME+TE+copying), got %d", len(jobs))
	}
}

func TestGetCharacterJobs_ActivityMapping(t *testing.T) {
	payload := `[
		{"job_id":1,"blueprint_id":100,"installer_id":42,"activity_id":4,"status":"active","start_date":"2026-02-20T10:00:00Z","end_date":"2026-02-22T10:00:00Z"},
		{"job_id":2,"blueprint_id":200,"installer_id":42,"activity_id":3,"status":"active","start_date":"2026-02-20T10:00:00Z","end_date":"2026-02-22T10:00:00Z"},
		{"job_id":3,"blueprint_id":300,"installer_id":42,"activity_id":5,"status":"active","start_date":"2026-02-20T10:00:00Z","end_date":"2026-02-22T10:00:00Z"}
	]`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(payload))
	}))
	defer srv.Close()

	c := newTestClient(srv)
	jobs, _, err := c.GetCharacterJobs(context.Background(), 42, "tok")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := map[int64]string{1: "me_research", 2: "te_research", 3: "copying"}
	for _, j := range jobs {
		wantActivity, ok := want[j.JobID]
		if !ok {
			t.Errorf("unexpected job_id %d", j.JobID)
			continue
		}
		if j.Activity != wantActivity {
			t.Errorf("job %d: activity got %q, want %q", j.JobID, j.Activity, wantActivity)
		}
	}
}

func TestGetCharacterJobs_ParsesDates(t *testing.T) {
	payload := `[
		{"job_id":1,"blueprint_id":100,"installer_id":42,"activity_id":4,"status":"active","start_date":"2026-02-20T10:00:00Z","end_date":"2026-02-22T10:00:00Z"}
	]`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(payload))
	}))
	defer srv.Close()

	c := newTestClient(srv)
	jobs, _, err := c.GetCharacterJobs(context.Background(), 42, "tok")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(jobs) != 1 {
		t.Fatalf("expected 1 job, got %d", len(jobs))
	}

	wantStart := time.Date(2026, 2, 20, 10, 0, 0, 0, time.UTC)
	wantEnd := time.Date(2026, 2, 22, 10, 0, 0, 0, time.UTC)
	if !jobs[0].StartDate.Equal(wantStart) {
		t.Errorf("start_date: got %v, want %v", jobs[0].StartDate, wantStart)
	}
	if !jobs[0].EndDate.Equal(wantEnd) {
		t.Errorf("end_date: got %v, want %v", jobs[0].EndDate, wantEnd)
	}
}

func TestGetCharacterJobs_UsesCorrectURL(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`[]`))
	}))
	defer srv.Close()

	c := newTestClient(srv)
	_, _, err := c.GetCharacterJobs(context.Background(), 42, "tok")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(gotPath, "/characters/42/industry/jobs") {
		t.Errorf("unexpected URL path: %q", gotPath)
	}
}

func TestGetCorporationJobs_FiltersByStatus(t *testing.T) {
	payload := `[
		{"job_id":1,"blueprint_id":100,"installer_id":42,"activity_id":4,"status":"active","start_date":"2026-02-20T10:00:00Z","end_date":"2026-02-22T10:00:00Z"},
		{"job_id":2,"blueprint_id":200,"installer_id":42,"activity_id":4,"status":"delivered","start_date":"2026-02-19T10:00:00Z","end_date":"2026-02-21T10:00:00Z"}
	]`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(payload))
	}))
	defer srv.Close()

	c := newTestClient(srv)
	jobs, _, err := c.GetCorporationJobs(context.Background(), 99999, "tok")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(jobs) != 1 {
		t.Errorf("expected 1 active job, got %d", len(jobs))
	}
}

func TestGetCorporationJobs_UsesCorrectURL(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`[]`))
	}))
	defer srv.Close()

	c := newTestClient(srv)
	_, _, err := c.GetCorporationJobs(context.Background(), 99999, "tok")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(gotPath, "/corporations/99999/industry/jobs") {
		t.Errorf("unexpected URL path: %q", gotPath)
	}
}

func TestGetCharacterJobs_EmptyList(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`[]`))
	}))
	defer srv.Close()

	c := newTestClient(srv)
	jobs, _, err := c.GetCharacterJobs(context.Background(), 42, "tok")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(jobs) != 0 {
		t.Errorf("expected empty slice, got %d items", len(jobs))
	}
}

func TestGetCorporationJobs_EmptyList(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`[]`))
	}))
	defer srv.Close()

	c := newTestClient(srv)
	jobs, _, err := c.GetCorporationJobs(context.Background(), 99999, "tok")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(jobs) != 0 {
		t.Errorf("expected empty slice, got %d items", len(jobs))
	}
}

// --- fixture-based full ESI response tests ---

func TestGetCorporationJobs_FullESIResponse(t *testing.T) {
	data, err := os.ReadFile("testdata/corporation_jobs.json")
	if err != nil {
		t.Fatalf("reading fixture: %v", err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(data)
	}))
	defer srv.Close()

	c := newTestClient(srv)
	jobs, _, err := c.GetCorporationJobs(context.Background(), 99999, "tok")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Fixture has 2 active jobs (activities 5 + 3); both survive the filter.
	if len(jobs) != 2 {
		t.Fatalf("expected 2 jobs, got %d", len(jobs))
	}
	j := jobs[0]
	if j.JobID != 644990911 {
		t.Errorf("JobID: got %d, want 644990911", j.JobID)
	}
	if j.BlueprintID != 1052548714404 {
		t.Errorf("BlueprintID: got %d, want 1052548714404", j.BlueprintID)
	}
	if j.Activity != "copying" {
		t.Errorf("Activity: got %q, want copying", j.Activity)
	}
	if j.Status != "active" {
		t.Errorf("Status: got %q, want active", j.Status)
	}
	wantStart := time.Date(2026, 3, 6, 18, 33, 47, 0, time.UTC)
	if !j.StartDate.Equal(wantStart) {
		t.Errorf("StartDate: got %v, want %v", j.StartDate, wantStart)
	}
	wantEnd := time.Date(2026, 3, 17, 18, 33, 47, 0, time.UTC)
	if !j.EndDate.Equal(wantEnd) {
		t.Errorf("EndDate: got %v, want %v", j.EndDate, wantEnd)
	}
	if jobs[1].JobID != 644990823 {
		t.Errorf("jobs[1].JobID: got %d, want 644990823", jobs[1].JobID)
	}
	if jobs[1].Activity != "te_research" {
		t.Errorf("jobs[1].Activity: got %q, want te_research", jobs[1].Activity)
	}
}

func TestGetCharacterJobs_FullESIResponse(t *testing.T) {
	data, err := os.ReadFile("testdata/character_jobs_mixed.json")
	if err != nil {
		t.Fatalf("reading fixture: %v", err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(data)
	}))
	defer srv.Close()

	c := newTestClient(srv)
	jobs, _, err := c.GetCharacterJobs(context.Background(), 42, "tok")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Fixture: 1 active me_research + 1 delivered manufacturing (filtered) + 1 ready copying.
	if len(jobs) != 2 {
		t.Fatalf("expected 2 jobs (manufacturing+delivered filtered), got %d", len(jobs))
	}
	if jobs[0].JobID != 700000001 {
		t.Errorf("jobs[0].JobID: got %d, want 700000001", jobs[0].JobID)
	}
	if jobs[0].Activity != "me_research" {
		t.Errorf("jobs[0].Activity: got %q, want me_research", jobs[0].Activity)
	}
	if jobs[0].Status != "active" {
		t.Errorf("jobs[0].Status: got %q, want active", jobs[0].Status)
	}
	if jobs[1].JobID != 700000003 {
		t.Errorf("jobs[1].JobID: got %d, want 700000003", jobs[1].JobID)
	}
	if jobs[1].Activity != "copying" {
		t.Errorf("jobs[1].Activity: got %q, want copying", jobs[1].Activity)
	}
	if jobs[1].Status != "ready" {
		t.Errorf("jobs[1].Status: got %q, want ready", jobs[1].Status)
	}
}

// --- pagination ---

func TestGetCharacterJobs_MultiPage(t *testing.T) {
	page1, err := os.ReadFile("testdata/character_jobs_page1.json")
	if err != nil {
		t.Fatalf("reading page1 fixture: %v", err)
	}
	page2, err := os.ReadFile("testdata/character_jobs_page2.json")
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
	jobs, _, err := c.GetCharacterJobs(context.Background(), 42, "tok")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(jobs) != 2 {
		t.Fatalf("expected 2 jobs, got %d", len(jobs))
	}
	if jobs[0].JobID != 645493269 {
		t.Errorf("jobs[0].JobID: got %d, want 645493269", jobs[0].JobID)
	}
	if jobs[1].JobID != 645490001 {
		t.Errorf("jobs[1].JobID: got %d, want 645490001", jobs[1].JobID)
	}
	if got := requestCount.Load(); got != 2 {
		t.Errorf("expected 2 HTTP requests (page 1 + page 2), got %d", got)
	}
}

func TestGetCharacterJobs_XPagesAbsent(t *testing.T) {
	var requestCount atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)
		// No X-Pages header — single-page response.
		_, _ = w.Write([]byte(`[{"job_id":1,"blueprint_id":100,"installer_id":42,"activity_id":5,"status":"active","start_date":"2026-03-09T15:36:04Z","end_date":"2026-03-09T16:18:18Z"}]`))
	}))
	defer srv.Close()

	c := newTestClient(srv)
	jobs, _, err := c.GetCharacterJobs(context.Background(), 42, "tok")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(jobs) != 1 {
		t.Errorf("expected 1 job, got %d", len(jobs))
	}
	if got := requestCount.Load(); got != 1 {
		t.Errorf("expected exactly 1 HTTP request, got %d", got)
	}
}

func TestGetCorporationJobs_MultiPage(t *testing.T) {
	page1, err := os.ReadFile("testdata/corporation_jobs_page1.json")
	if err != nil {
		t.Fatalf("reading page1 fixture: %v", err)
	}
	page2, err := os.ReadFile("testdata/corporation_jobs_page2.json")
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
	jobs, _, err := c.GetCorporationJobs(context.Background(), 99999, "tok")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(jobs) != 2 {
		t.Fatalf("expected 2 jobs, got %d", len(jobs))
	}
	if jobs[0].JobID != 644990911 {
		t.Errorf("jobs[0].JobID: got %d, want 644990911", jobs[0].JobID)
	}
	if jobs[1].JobID != 644990823 {
		t.Errorf("jobs[1].JobID: got %d, want 644990823", jobs[1].JobID)
	}
	if got := requestCount.Load(); got != 2 {
		t.Errorf("expected 2 HTTP requests (page 1 + page 2), got %d", got)
	}
}
