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

package automaticupgrades

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/integrations/kube-agent-updater/pkg/maintenance"
	"github.com/gravitational/teleport/integrations/kube-agent-updater/pkg/version"
)

const testVersion = "v1.2.3"

func Test_Channels_CheckAndSetDefaults(t *testing.T) {
	t.Run("no channels", func(t *testing.T) {
		c := Channels{}
		require.NoError(t, c.CheckAndSetDefaults())
	})
	t.Run("single channel", func(t *testing.T) {
		channel := &Channel{StaticVersion: testVersion}
		c := Channels{"foo": channel}
		require.NoError(t, c.CheckAndSetDefaults())
		require.NotNil(t, channel.versionGetter)
		require.NotNil(t, channel.criticalTrigger)
	})
	t.Run("many channels", func(t *testing.T) {
		channel1 := &Channel{StaticVersion: testVersion}
		channel2 := &Channel{StaticVersion: testVersion}
		channel3 := &Channel{StaticVersion: testVersion}
		c := Channels{"foo": channel1, "bar": channel2, "baz": channel3}
		require.NoError(t, c.CheckAndSetDefaults())
		require.NotNil(t, channel1.versionGetter)
		require.NotNil(t, channel1.criticalTrigger)
		require.NotNil(t, channel2.versionGetter)
		require.NotNil(t, channel2.criticalTrigger)
		require.NotNil(t, channel3.versionGetter)
		require.NotNil(t, channel3.criticalTrigger)
	})
}

func Test_Channel_CheckAndSetDefaults(t *testing.T) {

	tests := []struct {
		name                        string
		channel                     Channel
		assertError                 require.ErrorAssertionFunc
		expectedVersionGetterType   interface{}
		expectedCriticalTriggerType interface{}
	}{
		{
			name:        "empty (invalid)",
			channel:     Channel{},
			assertError: require.Error,
		},
		{
			name: "forward URL (valid)",
			channel: Channel{
				ForwardURL: stableCloudVersionBaseURL,
			},
			assertError:                 require.NoError,
			expectedVersionGetterType:   &version.BasicHTTPVersionGetter{},
			expectedCriticalTriggerType: maintenance.BasicHTTPMaintenanceTrigger{},
		},
		{
			name: "static version (valid)",
			channel: Channel{
				StaticVersion: testVersion,
			},
			assertError:                 require.NoError,
			expectedVersionGetterType:   version.StaticGetter{},
			expectedCriticalTriggerType: maintenance.StaticTrigger{},
		},
		{
			name: "all set (invalid)",
			channel: Channel{
				ForwardURL:    stableCloudVersionBaseURL,
				StaticVersion: testVersion,
			},
			assertError: require.Error,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.assertError(t, tt.channel.CheckAndSetDefaults())
			require.IsType(t, tt.expectedVersionGetterType, tt.channel.versionGetter)
			require.IsType(t, tt.expectedCriticalTriggerType, tt.channel.criticalTrigger)
		})
	}
}
