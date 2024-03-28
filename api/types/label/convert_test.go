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

import (
	"testing"

	"github.com/stretchr/testify/require"

	labelv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/label/v1"
)

func TestFromMap(t *testing.T) {
	tests := []struct {
		name string
		in   map[string][]string
		want []*labelv1.Label
	}{
		{
			name: "empty map",
			in:   map[string][]string{},
			want: nil,
		},
		{
			name: "map with one entry",
			in:   map[string][]string{"key1": {"value1", "value2"}},
			want: []*labelv1.Label{{Name: "key1", Values: []string{"value1", "value2"}}},
		},
		{
			name: "map with multiple entries",
			in: map[string][]string{
				"key1": {"value1", "value2"},
				"key2": {"value3"},
			},
			want: []*labelv1.Label{
				{Name: "key1", Values: []string{"value1", "value2"}},
				{Name: "key2", Values: []string{"value3"}},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FromMap(tt.in)
			require.ElementsMatch(t, tt.want, got)
		})
	}
}

func TestToMap(t *testing.T) {
	tests := []struct {
		name   string
		labels []*labelv1.Label
		want   map[string][]string
	}{
		{
			name:   "empty slice",
			labels: []*labelv1.Label{},
			want:   map[string][]string{},
		},
		{
			name: "slice with one label",
			labels: []*labelv1.Label{
				{Name: "key1", Values: []string{"value1", "value2"}},
			},
			want: map[string][]string{"key1": {"value1", "value2"}},
		},
		{
			name: "slice with multiple labels",
			labels: []*labelv1.Label{
				{Name: "key1", Values: []string{"value1", "value2"}},
				{Name: "key2", Values: []string{"value3"}},
			},
			want: map[string][]string{"key1": {"value1", "value2"}, "key2": {"value3"}},
		},
		{
			name: "slice with multiple labels with same key",
			labels: []*labelv1.Label{
				{Name: "key1", Values: []string{"value1", "value2"}},
				{Name: "key1", Values: []string{"value3"}},
			},
			want: map[string][]string{"key1": {"value1", "value2", "value3"}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ToMap(tt.labels)
			require.Equal(t, tt.want, got)
		})
	}
}
