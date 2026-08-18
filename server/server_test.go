package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"gitdrift/pkg/analyzer"
)

func TestValidateRepoURL(t *testing.T) {
	cases := []struct {
		input string
		want  string
		err   bool
	}{
		{"https://github.com/gin-gonic/gin", "https://github.com/gin-gonic/gin.git", false},
		{"https://github.com/gin-gonic/gin.git", "https://github.com/gin-gonic/gin.git", false},
		{"http://github.com/gin-gonic/gin", "https://github.com/gin-gonic/gin.git", false},
		{"https://github.com/gin-gonic/gin/tree/master/context.go", "https://github.com/gin-gonic/gin.git", false},
		{"git@github.com:gin-gonic/gin", "https://github.com/gin-gonic/gin.git", false},
		{"https://github.com/gin-gonic", "", true},
		{"https://gitlab.com/foo/bar", "", true},
		{"not-a-url", "", true},
		{"", "", true},
		{"https://example.com/foo/bar", "", true},
	}
	for _, c := range cases {
		got, err := ValidateRepoURL(c.input)
		if c.err {
			if err == nil {
				t.Errorf("ValidateRepoURL(%q): expected error, got %q", c.input, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("ValidateRepoURL(%q): unexpected error %v", c.input, err)
			continue
		}
		if got != c.want {
			t.Errorf("ValidateRepoURL(%q) = %q, want %q", c.input, got, c.want)
		}
	}
}

func TestJobStoreLifecycle(t *testing.T) {
	store := NewJobStore(10, time.Minute)

	job, err := store.Create("https://github.com/gin-gonic/gin", 100)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if job.Stage != StageQueued {
		t.Errorf("stage = %q, want queued", job.Stage)
	}

	store.Update(job.ID, func(j *Job) { j.Stage = StageDone })
	got, ok := store.Get(job.ID)
	if !ok {
		t.Fatal("job missing after update")
	}
	if got.Stage != StageDone {
		t.Errorf("stage = %q, want done", got.Stage)
	}
}

func TestHandleAnalyzeValidation(t *testing.T) {
	h := NewHandler(NewJobStore(10, time.Minute), &Engine{}, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/analyze",
		strings.NewReader(`{"url":"https://gitlab.com/foo/bar","depth":50}`))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("bad JSON response: %v", err)
	}
	if body["error"] == "" {
		t.Error("expected error message in response")
	}
}

func TestHandleStatusNotFound(t *testing.T) {
	h := NewHandler(NewJobStore(10, time.Minute), &Engine{}, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/status?id=nope", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

// fakeAnalyzer completes jobs without touching the network.
func fakeAnalyzer(job *Job) error {
	job.Result = &analyzer.Report{
		GeneratedAt: time.Now(),
		Repo:        "fake",
		Stats:       analyzer.RepoStats{TotalFiles: 42, TotalLines: 1337, Languages: map[string]int{".go": 42}},
		Hotspots:    []analyzer.HotspotFile{{Path: "main.go", CommitCount: 7}},
		Todos:       []analyzer.TodoItem{{File: "main.go", Line: 3, Tag: "TODO", Content: "fix"}},
	}
	return nil
}

func TestAnalyzeStatusReportFlow(t *testing.T) {
	store := NewJobStore(10, time.Minute)
	h := NewHandler(store, &Engine{CloneTimeout: time.Second}, nil)
	h.analyze = fakeAnalyzer

	resp := httptest.NewRecorder()
	h.ServeHTTP(resp, httptest.NewRequest(http.MethodPost, "/api/analyze",
		strings.NewReader(`{"url":"https://github.com/gin-gonic/gin","depth":50}`)))
	if resp.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d: %s", resp.Code, resp.Body.String())
	}
	var job Job
	if err := json.Unmarshal(resp.Body.Bytes(), &job); err != nil {
		t.Fatalf("parse analyze response: %v", err)
	}

	// wait for the async fake analyzer
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		got, _ := store.Get(job.ID)
		if got != nil && got.Stage == StageDone {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	// status endpoint returns the finished job
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/status?id="+job.ID, nil))
	var status Job
	if err := json.Unmarshal(rec.Body.Bytes(), &status); err != nil {
		t.Fatalf("parse status: %v", err)
	}
	if status.Stage != StageDone {
		t.Fatalf("expected done, got %q", status.Stage)
	}

	// report endpoint serves the HTML dashboard
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/report?id="+job.ID, nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("report status = %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "text/html") {
		t.Errorf("report content-type = %q", ct)
	}
	if !strings.Contains(rec.Body.String(), "<html") {
		t.Error("report body missing html")
	}

	// JSON variant
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/report?id="+job.ID+"&json=1", nil))
	var report analyzer.Report
	if err := json.Unmarshal(rec.Body.Bytes(), &report); err != nil {
		t.Fatalf("parse report json: %v", err)
	}
	if report.Stats.TotalFiles != 42 {
		t.Errorf("TotalFiles = %d, want 42", report.Stats.TotalFiles)
	}
}
