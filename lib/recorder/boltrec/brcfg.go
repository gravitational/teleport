package boltrec

import (
	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/gravitational/trace"
	"github.com/gravitational/teleport/lib/recorder"
	"github.com/gravitational/teleport/lib/utils"
)

// cfg represents JSON config for bolt backlend
type cfg struct {
	Path string `json:"path"`
}

// FromString initialized the backend from backend-specific string
func FromObject(in interface{}) (recorder.Recorder, error) {
	var c *cfg
	if err := utils.ObjectToStruct(in, &c); err != nil {
		return nil, trace.Wrap(err)
	}
	if len(c.Path) == 0 {
		return nil, trace.Errorf(
			`please supply a valid dictionary, e.g. {"path": "/opt/bolt.db"}`)
	}
	return New(c.Path)
}
