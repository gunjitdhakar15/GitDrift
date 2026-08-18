package main

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"time"
)

// Handler wires the job store and analysis engine to HTTP routes.
type Handler struct {
	store  *JobStore
	engine *Engine
	ui     fs.FS
	// analyze is injectable for tests.
	analyze func(job *Job) error
}

func NewHandler(store *JobStore, engine *Engine, ui fs.FS) *Handler {
	h := &Handler{store: store, engine: engine, ui: ui}
	h.analyze = h.remoteAnalyze
	return h
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.Method == http.MethodGet && r.URL.Path == "/":
		h.handleIndex(w, r)
	case r.Method == http.MethodGet && (r.URL.Path == "/healthz" || r.URL.Path == "/api/health"):
		h.handleHealth(w, r)
	case r.Method == http.MethodPost && r.URL.Path == "/api/analyze":
		h.handleAnalyze(w, r)
	case r.Method == http.MethodGet && r.URL.Path == "/api/status":
		h.handleStatus(w, r)
	case r.Method == http.MethodGet && r.URL.Path == "/api/report":
		h.handleReport(w, r)
	default:
		http.NotFound(w, r)
	}
}

// handleHealth answers lightweight probes used by hosting platforms and
// keep-alive monitors to prevent free-tier cold starts.
func (h *Handler) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, map[string]string{
		"status": "ok",
		"app":    "gitdrift",
		"time":   time.Now().UTC().Format(time.RFC3339),
	})
}

func (h *Handler) handleIndex(w http.ResponseWriter, r *http.Request) {
	data, err := fs.ReadFile(h.ui, "webui/index.html")
	if err != nil {
		http.Error(w, "ui missing", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(data)
}

type analyzeRequest struct {
	URL   string `json:"url"`
	Depth int    `json:"depth"`
}

func (h *Handler) handleAnalyze(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var req analyzeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONErr(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if req.Depth < 1 || req.Depth > 1000 {
		req.Depth = 100
	}

	cloneURL, err := ValidateRepoURL(req.URL)
	if err != nil {
		writeJSONErr(w, http.StatusBadRequest, err.Error())
		return
	}

	job, err := h.store.Create(req.URL, req.Depth)
	if err != nil {
		writeJSONErr(w, http.StatusTooManyRequests, err.Error())
		return
	}

	go h.run(job, cloneURL)

	writeJSON(w, http.StatusAccepted, job)
}

// run executes the analysis async and mutates the job in place.
func (h *Handler) run(job *Job, _ string) {
	if err := h.analyze(job); err != nil {
		h.store.Update(job.ID, func(j *Job) {
			j.Stage = StageFailed
			j.Error = err.Error()
		})
		return
	}
	h.store.Update(job.ID, func(j *Job) {
		j.Stage = StageDone
		j.Progress = ""
	})
}

// remoteAnalyze performs the clone + analysis pipeline, updating progress.
func (h *Handler) remoteAnalyze(job *Job) error {
	h.store.Update(job.ID, func(j *Job) {
		j.Stage = StageCloning
		j.Progress = "cloning repository (shallow)"
	})

	report, err := h.engine.Analyze(job.URL, job.Depth)
	if err != nil {
		return err
	}

	h.store.Update(job.ID, func(j *Job) {
		job.Result = report
	})
	return nil
}

func (h *Handler) handleStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	id := r.URL.Query().Get("id")
	job, ok := h.store.Get(id)
	if !ok {
		writeJSONErr(w, http.StatusNotFound, "job not found")
		return
	}
	writeJSON(w, http.StatusOK, job)
}

func (h *Handler) handleReport(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	job, ok := h.store.Get(id)
	if !ok {
		writeJSONErr(w, http.StatusNotFound, "job not found")
		return
	}
	if job.Stage != StageDone || job.Result == nil {
		writeJSONErr(w, http.StatusConflict, "job not finished")
		return
	}

	if r.URL.Query().Get("json") == "1" {
		w.Header().Set("Content-Type", "application/json")
		if err := job.Result.WriteJSON(w); err != nil {
			http.Error(w, "serialize failed", http.StatusInternalServerError)
		}
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := job.Result.WriteHTMLTo(w); err != nil {
		http.Error(w, "render failed", http.StatusInternalServerError)
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		fmt.Fprintf(w, `{"error":"encode failed"}`)
	}
}

func writeJSONErr(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
