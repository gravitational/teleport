package boltbk

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/gravitational/teleport/backend"
	"github.com/gravitational/trace"
)

// cfg represents JSON config for bolt backlend
type cfg struct {
	Path string `json:"path"`
}

// FromString initialized the backend from backend-specific string
func FromString(v string) (backend.Backend, error) {
	if len(v) == 0 {
		return nil, fmt.Errorf(
			`please supply a valid dictionary, e.g. {"path": "/opt/bolt.db"}`)
	}
	var c *cfg
	if err := json.Unmarshal([]byte(v), &c); err != nil {
		return nil, fmt.Errorf("invalid backend configuration format, err: %v", err)
	}
	path, err := filepath.Abs(c.Path)
	if err != nil {
		return nil, trace.Wrap(err, "failed to convert path")
	}
	dir := filepath.Dir(path)
	s, err := os.Stat(dir)
	if err != nil {
		return nil, fmt.Errorf("failed to stat directory '%v': %v", dir, err)
	}
	if !s.IsDir() {
		return nil, fmt.Errorf("path %v should be a valid directory '%v'", dir)
	}
	return New(path)
}
