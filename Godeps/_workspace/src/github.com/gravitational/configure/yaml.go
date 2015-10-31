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
package configure

import (
	"github.com/gravitational/teleport/Godeps/_workspace/src/gopkg.in/yaml.v2"

	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/gravitational/trace"
)

// ParseYAML parses yaml-encoded byte string into the struct
// passed to the function.
func ParseYAML(data []byte, cfg interface{}) error {
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return trace.Wrap(err)
	}
	return nil
}
