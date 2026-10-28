package storage

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"sync"
)

// memoryStorage keeps objects in a map. For tests and local runs without S3.
type memoryStorage struct {
	mu      sync.RWMutex
	objects map[string][]byte
}

func newMemory() *memoryStorage { return &memoryStorage{objects: map[string][]byte{}} }

func (m *memoryStorage) Put(_ context.Context, key string, body io.Reader, _ int64, _ string) error {
	b, err := io.ReadAll(body)
	if err != nil {
		return fmt.Errorf("storage: read body: %w", err)
	}
	m.mu.Lock()
	m.objects[key] = b
	m.mu.Unlock()
	return nil
}

func (m *memoryStorage) Get(_ context.Context, key string) (io.ReadCloser, error) {
	m.mu.RLock()
	b, ok := m.objects[key]
	m.mu.RUnlock()
	if !ok {
		return nil, ErrNotFound
	}
	return io.NopCloser(bytes.NewReader(b)), nil
}

func (m *memoryStorage) Delete(_ context.Context, key string) error {
	m.mu.Lock()
	delete(m.objects, key)
	m.mu.Unlock()
	return nil
}
