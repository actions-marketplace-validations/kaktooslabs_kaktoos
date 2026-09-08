package variable

import (
	"sync"
)

// Store manages the variable store for a single scenario execution context.
// It provides thread-safe access and methods for seeding and retrieving values.
type Store struct {
	mu    sync.RWMutex
	items map[string]string
}

// NewStore creates and returns a new, empty variable store.
// This store must be used exclusively for a single RunScenario call to prevent leakage.
func NewStore() *Store {
	return &Store{
		items: make(map[string]string),
	}
}

// Seed populates the store with initial values (e.g., from environment variables or initial context).
func (s *Store) Seed(initialValues map[string]string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for k, v := range initialValues {
		s.items[k] = v
	}
}

// Set stores a new variable or updates an existing one.
// This is called directly by the runner/extractor when a value is processed.
func (s *Store) Set(key string, value string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.items[key] = value
}

// Get retrieves a variable's value by key. Returns an empty string and false if the key does not exist.
func (s *Store) Get(key string) (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	value, ok := s.items[key]
	return value, ok
}

// GetAll returns a map of all currently stored variables.
func (s *Store) GetAll() map[string]string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	// Return a copy to prevent external modification
	copyMap := make(map[string]string)
	for k, v := range s.items {
		copyMap[k] = v
	}
	return copyMap
}
