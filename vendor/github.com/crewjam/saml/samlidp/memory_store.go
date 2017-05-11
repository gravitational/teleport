package samlidp

import (
	"encoding/json"
	"strings"
	"sync"
)

// MemoryStore is an implementation of Store that resides completely
// in memory.
type MemoryStore struct {
	mu   sync.RWMutex
	data map[string]string
}

// Get fetches the data stored in `key` and unmarshals it into `value`.
func (s *MemoryStore) Get(key string, value interface{}) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	v, ok := s.data[key]
	if !ok {
		return ErrNotFound
	}
	return json.Unmarshal([]byte(v), value)
}

// Put marshals `value` and stores it in `key`.
func (s *MemoryStore) Put(key string, value interface{}) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.data == nil {
		s.data = map[string]string{}
	}

	buf, err := json.Marshal(value)
	if err != nil {
		return err
	}
	s.data[key] = string(buf)
	return nil
}

// Delete removes `key`
func (s *MemoryStore) Delete(key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.data, key)
	return nil
}

// List returns all the keys that start with `prefix`. The prefix is
// stripped from each returned value. So if keys are ["aa", "ab", "cd"]
// then List("a") would produce []string{"a", "b"}
func (s *MemoryStore) List(prefix string) ([]string, error) {
	rv := []string{}
	for k := range s.data {
		if strings.HasPrefix(k, prefix) {
			rv = append(rv, strings.TrimPrefix(k, prefix))
		}
	}
	return rv, nil
}
