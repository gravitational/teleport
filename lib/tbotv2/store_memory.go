package tbotv2

import (
	"context"
	"sync"
)

var MemoryStoreType = "memory"

// MemoryStore implements Writer used by Bot and Destinations as storage
type MemoryStore struct {
	mu   sync.Mutex
	data map[string][]byte
}

func (w *MemoryStore) String() string {
	return MemoryStoreType
}

func (w *MemoryStore) Write(_ context.Context, name string, data []byte) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.data == nil {
		w.data = make(map[string][]byte)
	}
	w.data[name] = data
	return nil
}

func (w *MemoryStore) Read(_ context.Context, name string) ([]byte, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.data[name], nil
}
