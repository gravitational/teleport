// Copyright 2026 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package types

import "github.com/gravitational/trace"

// EnrollPairingFilter encodes filter params for enroll pairing watchers.
type EnrollPairingFilter struct {
	Name string
}

const enrollPairingFilterKeyName = "name"

// IntoMap copies EnrollPairingFilter values into a map.
func (f *EnrollPairingFilter) IntoMap() map[string]string {
	m := make(map[string]string)
	if f.Name != "" {
		m[enrollPairingFilterKeyName] = f.Name
	}
	return m
}

// FromMap copies values from a map into this EnrollPairingFilter value.
func (f *EnrollPairingFilter) FromMap(m map[string]string) error {
	for key, val := range m {
		switch key {
		case enrollPairingFilterKeyName:
			f.Name = val
		default:
			return trace.BadParameter("unknown filter key %s", key)
		}
	}
	return nil
}

// Match returns true if the given pairing name passes the filter. An empty
// filter matches everything.
func (f *EnrollPairingFilter) Match(name string) bool {
	if f.Name != "" && name != f.Name {
		return false
	}
	return true
}
