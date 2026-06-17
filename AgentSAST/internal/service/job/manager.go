// Package job orchestrates SAST runs in the background and supports cancel.
package job

import (
	"context"
	"sync"
)

// manager tracks cancel functions for in-flight jobs.
type manager struct {
	mu      sync.Mutex
	cancels map[string]context.CancelFunc
}

func newManager() *manager {
	return &manager{cancels: make(map[string]context.CancelFunc)}
}

func (m *manager) add(id string, cancel context.CancelFunc) {
	m.mu.Lock()
	m.cancels[id] = cancel
	m.mu.Unlock()
}

func (m *manager) remove(id string) {
	m.mu.Lock()
	delete(m.cancels, id)
	m.mu.Unlock()
}

// cancel signals the job; returns false if it is not tracked (already finished).
func (m *manager) cancel(id string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	if c, ok := m.cancels[id]; ok {
		c()
		return true
	}
	return false
}
