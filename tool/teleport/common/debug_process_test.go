// Teleport
// Copyright (C) 2026 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package common

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	debugclient "github.com/gravitational/teleport/lib/client/debug"
)

func TestBuildServiceGroups(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()
	info := debugclient.ProcessInfo{
		ServiceDebugInfo: map[string]debugclient.ServiceDebugInfo{
			"auth.expiry": {
				ServiceName:   "auth.expiry",
				HasInfo:       true,
				ServiceConfig: "auth_service: {}",
				RunningSince:  now,
			},
			"auth.broadcast": {
				ServiceName:  "auth.broadcast",
				IsCritical:   true,
				RunningSince: now.Add(time.Second),
			},
			"proxy.web": {
				ServiceName:  "proxy.web",
				RunningSince: now.Add(2 * time.Second),
			},
		},
	}

	t.Run("groups by top-level service by default", func(t *testing.T) {
		groups := buildServiceGroups(info, processInfoOutputOptions{})
		require.Len(t, groups, 2)
		require.Equal(t, "auth", groups[0].name)
		require.Equal(t, []string{"auth.broadcast", "auth.expiry"}, groups[0].services)
		require.True(t, groups[0].hasConfig)
		require.True(t, groups[0].critical)
		require.Equal(t, "proxy", groups[1].name)
	})

	t.Run("filter by top-level service", func(t *testing.T) {
		groups := buildServiceGroups(info, processInfoOutputOptions{serviceFilter: "auth"})
		require.Len(t, groups, 1)
		require.Equal(t, "auth", groups[0].name)
		require.Equal(t, []string{"auth.broadcast", "auth.expiry"}, groups[0].services)
	})

	t.Run("filter by exact subservice", func(t *testing.T) {
		groups := buildServiceGroups(info, processInfoOutputOptions{serviceFilter: "auth.expiry"})
		require.Len(t, groups, 1)
		require.Equal(t, "auth.expiry", groups[0].name)
		require.Equal(t, []string{"auth.expiry"}, groups[0].services)
	})
}
