package utils

import (
	"encoding/json"
	"fmt"

	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/gravitational/trace"
)

func ObjectToStruct(in interface{}, out interface{}) error {
	bytes, err := json.Marshal(in)
	if err != nil {
		return trace.Wrap(err, fmt.Sprintf("failed to marshal: %v", in))
	}
	if err := json.Unmarshal([]byte(bytes), out); err != nil {
		return trace.Wrap(err, fmt.Sprintf("failed to unmarshal %v into %T", in, out))
	}
	return nil
}
