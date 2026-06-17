package storage

import (
	"context"
	"fmt"
	"sort"
	"sync"

	"agentdast/pkg/types"
)

// InMemoryStore keeps scan results in process memory. It is the default backend
// for CLI and stateless use.
type InMemoryStore struct {
	mu      sync.RWMutex
	results map[string]*types.ScanResult
}

// NewInMemoryStore returns an empty in-memory store.
func NewInMemoryStore() *InMemoryStore {
	return &InMemoryStore{results: make(map[string]*types.ScanResult)}
}

func (s *InMemoryStore) SaveResult(_ context.Context, result *types.ScanResult) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.results[result.ID] = result
	return nil
}

func (s *InMemoryStore) GetResult(_ context.Context, scanID string) (*types.ScanResult, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	r, ok := s.results[scanID]
	if !ok {
		return nil, fmt.Errorf("scan %q not found", scanID)
	}
	return r, nil
}

func (s *InMemoryStore) ListResults(_ context.Context, limit, offset int) ([]*types.ScanResult, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*types.ScanResult, 0, len(s.results))
	for _, r := range s.results {
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].StartedAt.After(out[j].StartedAt) })
	if offset > len(out) {
		offset = len(out)
	}
	out = out[offset:]
	if limit > 0 && limit < len(out) {
		out = out[:limit]
	}
	return out, nil
}

func (s *InMemoryStore) DeleteResult(_ context.Context, scanID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.results, scanID)
	return nil
}

func (s *InMemoryStore) Close() error { return nil }
