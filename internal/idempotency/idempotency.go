package idempotency

import (
	"sync"
	"time"
)

type Status string

const (
	StatusInProgress Status = "in_progress"
	StatusCompleted  Status = "completed"
)

type StepSummary struct{ Name, Status, Duration, Error string }
type Record struct {
	Key, ExecutionID, ScenarioName, ExecStatus, Duration, Error string
	Status                                                      Status
	StartedAt, FinishedAt                                       time.Time
	Steps                                                       []StepSummary
}
type Store interface {
	Get(string) (*Record, error)
	Set(string, Record) error
}
type memoryStore struct {
	mu sync.RWMutex
	m  map[string]Record
}

func NewMemoryStore() Store { return &memoryStore{m: map[string]Record{}} }
func (s *memoryStore) Get(k string) (*Record, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.m[k]
	if !ok {
		return nil, nil
	}
	v.Steps = append([]StepSummary(nil), v.Steps...)
	return &v, nil
}
func (s *memoryStore) Set(k string, v Record) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	v.Steps = append([]StepSummary(nil), v.Steps...)
	s.m[k] = v
	return nil
}
