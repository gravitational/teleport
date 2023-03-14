/*
Copyright 2022 Gravitational, Inc.

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

package types

import (
	"encoding/json"

	"github.com/gravitational/trace"
	saml2 "github.com/russellhaering/gosaml2"
)

// AssertionInfo is an alias for saml2.AssertionInfo with additional methods, required for serialization to/from protobuf.
// With those we can reference it with an option like so: `(gogoproto.customtype) = "AssertionInfo"`
type AssertionInfo saml2.AssertionInfo

func (a *AssertionInfo) Size() int {
	bytes, err := json.Marshal(a)
	if err != nil {
		return 0
	}
	return len(bytes)
}

func (a *AssertionInfo) Unmarshal(bytes []byte) error {
	return trace.Wrap(json.Unmarshal(bytes, a))
}

func (a *AssertionInfo) MarshalTo(bytes []byte) (int, error) {
	out, err := json.Marshal(a)
	if err != nil {
		return 0, trace.Wrap(err)
	}

	if len(out) > cap(bytes) {
		return 0, trace.BadParameter("capacity too low: %v, need %v", cap(bytes), len(out))
	}

	copy(bytes, out)

	return len(out), nil
}
