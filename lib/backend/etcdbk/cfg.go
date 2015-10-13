package etcdbk

import (
	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/gravitational/trace"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/utils"
)

// cfg represents JSON config for etcd backlend
type cfg struct {
	Nodes []string `json:"nodes"`
	Key   string   `json:"key"`
}

// FromString initialized the backend from backend-specific string
func FromObject(in interface{}) (backend.Backend, error) {
	var c *cfg
	if err := utils.ObjectToStruct(in, &c); err != nil {
		return nil, trace.Wrap(err)
	}
	if len(c.Nodes) == 0 {
		return nil, trace.Errorf(`please supply a valid dictionary, e.g. {"nodes": ["http://localhost:4001]}`)
	}
	return New(c.Nodes, c.Key)
}
