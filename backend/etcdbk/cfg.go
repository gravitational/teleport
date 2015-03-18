package etcdbk

import (
	"encoding/json"
	"fmt"

	"github.com/gravitational/teleport/backend"
)

// cfg represents JSON config for etcd backlend
type cfg struct {
	Nodes []string `json:"nodes"`
	Key   string   `json:"key"`
}

// FromString initialized the backend from backend-specific string
func FromString(v string) (backend.Backend, error) {
	if len(v) == 0 {
		return nil, fmt.Errorf(`please supply a valid dictionary, e.g. {"nodes": ["http://localhost:4001]}`)
	}
	var c *cfg
	if err := json.Unmarshal([]byte(v), &c); err != nil {
		return nil, fmt.Errorf("invalid backend configuration format, err: %v", err)
	}

	return New(c.Nodes, c.Key)
}
