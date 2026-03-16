// Package memory provides a thread-safe in-process key-value store for
// agent session state. One Memory instance is created per session and shared
// across all tool calls and sub-agent invocations within that session.
package memory

import (
	"fmt"
	"strings"
	"sync"
)

// Memory is a thread-safe in-process key-value store.
type Memory struct {
	mu    sync.RWMutex
	store map[string]string
}

// New returns an empty Memory ready for use.
func New() *Memory {
	return &Memory{store: make(map[string]string)}
}

// Set stores value under key, overwriting any previous value.
func (m *Memory) Set(key, value string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.store[key] = value
}

// Get retrieves the value for key. Returns ("", false) if the key is absent.
func (m *Memory) Get(key string) (string, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	v, ok := m.store[key]
	return v, ok
}

// All returns a snapshot of all stored key-value pairs.
func (m *Memory) All() map[string]string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make(map[string]string, len(m.store))
	for k, v := range m.store {
		out[k] = v
	}
	return out
}

// Delete removes key. No-op if the key does not exist.
func (m *Memory) Delete(key string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.store, key)
}

// String returns all entries as a human-readable "key: value" listing.
func (m *Memory) String() string {
	all := m.All()
	if len(all) == 0 {
		return "(memory is empty)"
	}
	var sb strings.Builder
	for k, v := range all {
		fmt.Fprintf(&sb, "%s: %s\n", k, v)
	}
	return strings.TrimRight(sb.String(), "\n")
}
