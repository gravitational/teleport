/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
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
	require.Empty(t, server.GetAllLabels())
	require.True(t, types.MatchLabels(server, emptyLabels))
	require.False(t, types.MatchLabels(server, map[string]string{"a": "b"}))

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

	require.True(t, types.MatchLabels(server, emptyLabels))
	require.False(t, types.MatchLabels(server, map[string]string{"a": "b"}))
	require.True(t, types.MatchLabels(server, map[string]string{"role": "database"}))
	require.True(t, types.MatchLabels(server, map[string]string{"time": "now"}))
	require.True(t, types.MatchLabels(server, map[string]string{"time": "now", "role": "database"}))
}
