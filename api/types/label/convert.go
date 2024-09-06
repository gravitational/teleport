// Copyright 2024 Gravitational, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package label

import labelv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/label/v1"

// FromMap converts a map[string][]string to a slice of *labelv1.Label.
// Each key-value pair in the input map is transformed into a label with the
// key as the name and the corresponding slice of strings as values.
func FromMap(in map[string][]string) []*labelv1.Label {
	if len(in) == 0 {
		return nil
	}

	out := make([]*labelv1.Label, 0, len(in))
	for name, values := range in {
		out = append(out, &labelv1.Label{Name: name, Values: values})
	}
	return out
}

// ToMap converts a slice of *labelv1.Label to a map[string][]string.
// Each label in the input slice contributes to the resulting map by adding
// its name as the key and appending its values to the corresponding slice
// of strings in the map.
// If there are multiple labels with the same key, their values will be concatenated.
func ToMap(labels []*labelv1.Label) map[string][]string {
	out := make(map[string][]string)
	for _, label := range labels {
		entry := out[label.GetName()]
		entry = append(entry, label.Values...)
		out[label.GetName()] = entry
	}
	return out
}
