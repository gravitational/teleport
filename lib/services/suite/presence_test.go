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

package suite

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
)

func TestServerLabels(t *testing.T) {
	emptyLabels := make(map[string]string)
	// empty
	server := &types.ServerV2{}
	require.Empty(t, cmp.Diff(server.GetAllLabels(), emptyLabels))
	require.Empty(t, server.GetAllLabels())
	require.Equal(t, types.MatchLabels(server, emptyLabels), true)
	require.Equal(t, types.MatchLabels(server, map[string]string{"a": "b"}), false)

	// more complex
	server = &types.ServerV2{
		Metadata: types.Metadata{
			Labels: map[string]string{
				"role": "database",
			},
		},
		Spec: types.ServerSpecV2{
			CmdLabels: map[string]types.CommandLabelV2{
				"time": {
					Period:  types.NewDuration(time.Second),
					Command: []string{"time"},
					Result:  "now",
				},
			},
		},
	}

	require.Empty(t, cmp.Diff(server.GetAllLabels(), map[string]string{
		"role": "database",
		"time": "now",
	}))

	require.Equal(t, types.MatchLabels(server, emptyLabels), true)
	require.Equal(t, types.MatchLabels(server, map[string]string{"a": "b"}), false)
	require.Equal(t, types.MatchLabels(server, map[string]string{"role": "database"}), true)
	require.Equal(t, types.MatchLabels(server, map[string]string{"time": "now"}), true)
	require.Equal(t, types.MatchLabels(server, map[string]string{"time": "now", "role": "database"}), true)
}
