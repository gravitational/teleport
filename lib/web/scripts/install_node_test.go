/*
Copyright 2015-2022 Gravitational, Inc.

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
package scripts

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils"
)

func TestMarshalLabelsYAML(t *testing.T) {
	for _, tt := range []struct {
		name     string
		labels   types.Labels
		expected []string
	}{
		{
			name:     "empty",
			labels:   types.Labels{},
			expected: []string{"{}"},
		},
		{
			name: "wildcard to wildcard",
			labels: types.Labels{
				types.Wildcard: utils.Strings{types.Wildcard},
			},
			expected: []string{`'*': '*'`},
		},
		{
			name: "multiple labels",
			labels: types.Labels{
				"dev":     utils.Strings{types.Wildcard},
				"product": utils.Strings{"scripts"},
			},
			expected: []string{`dev: '*'`, `product: scripts`},
		},
	} {
		got, err := MarshalLabelsYAML(tt.labels)
		require.NoError(t, err)

		require.Equal(t, tt.expected, got)
	}
}
