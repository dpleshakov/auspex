package esi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
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
