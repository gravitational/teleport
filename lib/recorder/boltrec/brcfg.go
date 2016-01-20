/*
Copyright 2015 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package boltrec

import (
	"github.com/gravitational/teleport/lib/recorder"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
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
