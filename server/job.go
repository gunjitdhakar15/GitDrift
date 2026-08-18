package main

import (
	"sync"
	"time"

	"gitdrift/pkg/analyzer"
)

// JobStage tracks the lifecycle of an analysis job.
type JobStage string

const (
	StageQueued  JobStage = "queued"
	StageCloning JobStage = "cloning"
	StageDone    JobStage = "done"
	StageFailed  JobStage = "failed"
)

// Job represents a single remote-repository analysis.
type Job struct {
	ID        string           `json:"id"`
	URL       string           `json:"url"`
	Depth     int              `json:"depth"`
	Stage     JobStage         `json:"stage"`
	Progress  string           `json:"progress,omitempty"`
	Error     string           `json:"error,omitempty"`
	CreatedAt time.Time        `json:"created_at"`
	Result    *analyzer.Report `json:"result,omitempty"`
}

// JobStore is a concurrency-safe in-memory job registry with TTL cleanup.
type JobStore struct {
	mu   sync.RWMutex
	jobs map[string]*Job
	max  int
	ttl  time.Duration
	seq  int
}

func NewJobStore(max int, ttl time.Duration) *JobStore {
	s := &JobStore{
		jobs: make(map[string]*Job),
		max:  max,
		ttl:  ttl,
	}
	go s.reaper()
	return s
}

func (s *JobStore) Create(url string, depth int) (*Job, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.jobs) >= s.max {
		return nil, errMaxJobs
	}

	s.seq++
	id := time.Now().Format("20060102-150405") + "-" + itoa(s.seq)
	job := &Job{
		ID:        id,
		URL:       url,
		Depth:     depth,
		Stage:     StageQueued,
		CreatedAt: time.Now(),
	}
	s.jobs[id] = job
	return job, nil
}

func (s *JobStore) Get(id string) (*Job, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	job, ok := s.jobs[id]
	return job, ok
}

func (s *JobStore) Update(id string, fn func(*Job)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if job, ok := s.jobs[id]; ok {
		fn(job)
	}
}

// reaper periodically evicts jobs older than the TTL to bound memory.
func (s *JobStore) reaper() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		s.mu.Lock()
		for id, job := range s.jobs {
			if time.Since(job.CreatedAt) > s.ttl {
				delete(s.jobs, id)
			}
		}
		s.mu.Unlock()
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [10]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
