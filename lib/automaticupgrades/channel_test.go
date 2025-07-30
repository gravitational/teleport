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
	"context"
	"testing"

	"github.com/coreos/go-semver/semver"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/automaticupgrades/constants"
	"github.com/gravitational/teleport/lib/automaticupgrades/maintenance"
	"github.com/gravitational/teleport/lib/automaticupgrades/version"
)

const testVersion = "v1.2.3"

func Test_Channels_CheckAndSetDefaults(t *testing.T) {
	customChannelURL := "https://foo.example.com/bar"
	t.Run("no channels", func(t *testing.T) {
		// When we start without channels, two channels must be created:
		// - "default"
		// - "stable/cloud"
		c := Channels{}
		require.NoError(t, c.CheckAndSetDefaults())
		require.Len(t, c, 2)
	})
	t.Run("single channel", func(t *testing.T) {
		// When we start with a channel, we must keep it and create "default"
		// and "stable/cloud".
		// The channel we passed must also get initialized.
		channel := &Channel{StaticVersion: testVersion}
		c := Channels{"foo": channel}
		require.NoError(t, c.CheckAndSetDefaults())
		require.Len(t, c, 3)
		require.NotNil(t, channel.versionGetter)
		require.NotNil(t, channel.criticalTrigger)
	})
	t.Run("many channels", func(t *testing.T) {
		// When we start with many channels, we must keep them and create "default"
		// and "stable/cloud".
		// The channels passed must also get initialized.
		channel1 := &Channel{StaticVersion: testVersion}
		channel2 := &Channel{StaticVersion: testVersion}
		channel3 := &Channel{StaticVersion: testVersion}
		c := Channels{"foo": channel1, "bar": channel2, "baz": channel3}
		require.NoError(t, c.CheckAndSetDefaults())
		require.Len(t, c, 5)
		require.NotNil(t, channel1.versionGetter)
		require.NotNil(t, channel1.criticalTrigger)
		require.NotNil(t, channel2.versionGetter)
		require.NotNil(t, channel2.criticalTrigger)
		require.NotNil(t, channel3.versionGetter)
		require.NotNil(t, channel3.criticalTrigger)
	})
	t.Run("stable/cloud set but not default", func(t *testing.T) {
		// When "stable/cloud" is set but not "default", we must use "stable/cloud" as "default".
		c := Channels{
			DefaultCloudChannelName: &Channel{ForwardURL: stableCloudVersionBaseURL},
		}
		require.NoError(t, c.CheckAndSetDefaults())
		require.Len(t, c, 2)
		defaultChannel, ok := c[DefaultChannelName]
		require.True(t, ok)
		require.Equal(t, stableCloudVersionBaseURL, defaultChannel.ForwardURL)
		stableCloudChannel, ok := c[DefaultCloudChannelName]
		require.True(t, ok)
		require.Equal(t, stableCloudVersionBaseURL, stableCloudChannel.ForwardURL)
	})
	t.Run("default set but not stable/cloud", func(t *testing.T) {
		// When "default" is set but not "stable/cloud", we must use "default" as "stable/cloud".
		c := Channels{
			DefaultChannelName: &Channel{ForwardURL: stableCloudVersionBaseURL},
		}
		require.NoError(t, c.CheckAndSetDefaults())
		require.Len(t, c, 2)
		defaultChannel, ok := c[DefaultChannelName]
		require.True(t, ok)
		require.Equal(t, stableCloudVersionBaseURL, defaultChannel.ForwardURL)
		stableCloudChannel, ok := c[DefaultCloudChannelName]
		require.True(t, ok)
		require.Equal(t, stableCloudVersionBaseURL, stableCloudChannel.ForwardURL)
	})
	t.Run("stable/cloud and default set", func(t *testing.T) {
		// When both "stable/cloud" and "default" are set we must not change them.
		c := Channels{
			DefaultChannelName:      &Channel{ForwardURL: customChannelURL},
			DefaultCloudChannelName: &Channel{ForwardURL: customChannelURL},
		}
		require.NoError(t, c.CheckAndSetDefaults())
		require.Len(t, c, 2)
		stableCloudChannel, ok := c[DefaultCloudChannelName]
		require.True(t, ok)
		require.Equal(t, customChannelURL, stableCloudChannel.ForwardURL)
		defaultChannel, ok := c[DefaultChannelName]
		require.True(t, ok)
		require.Equal(t, customChannelURL, defaultChannel.ForwardURL)
	})
}

func Test_Channels_DefaultChannel(t *testing.T) {
	channels := make(Channels)
	require.NoError(t, channels.CheckAndSetDefaults())

	defaultChannel, err := NewDefaultChannel()
	require.NoError(t, err)

	customDefaultChannel := &Channel{ForwardURL: "asdf"}
	tests := []struct {
		desc     string
		channels Channels
		want     *Channel
	}{
		{
			desc: "nil channels",
			want: defaultChannel,
		},
		{
			desc:     "default channels",
			channels: channels,
			want:     defaultChannel,
		},
		{
			desc: "configured channels",
			channels: Channels{
				DefaultChannelName: customDefaultChannel,
			},
			want: customDefaultChannel,
		},
	}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			got, err := test.channels.DefaultChannel()
			require.NoError(t, err)
			require.Equal(t, test.want, got)
		})
	}
}

func Test_Channel_CheckAndSetDefaults(t *testing.T) {

	tests := []struct {
		name                        string
		channel                     *Channel
		assertError                 require.ErrorAssertionFunc
		expectedVersionGetterType   interface{}
		expectedCriticalTriggerType interface{}
	}{
		{
			name:        "empty (invalid)",
			channel:     &Channel{},
			assertError: require.Error,
		},
		{
			name: "forward URL (valid)",
			channel: &Channel{
				ForwardURL: stableCloudVersionBaseURL,
			},
			assertError:                 require.NoError,
			expectedVersionGetterType:   version.BasicHTTPVersionGetter{},
			expectedCriticalTriggerType: maintenance.BasicHTTPMaintenanceTrigger{},
		},
		{
			name: "static version (valid)",
			channel: &Channel{
				StaticVersion: testVersion,
			},
			assertError:                 require.NoError,
			expectedVersionGetterType:   version.StaticGetter{},
			expectedCriticalTriggerType: maintenance.StaticTrigger{},
		},
		{
			name: "all set (invalid)",
			channel: &Channel{
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

func Test_Channel_GetVersion(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name            string
		targetVersion   string
		expectedVersion *semver.Version
		assertErr       require.ErrorAssertionFunc
		assertCheckErr  require.ErrorAssertionFunc
	}{
		{
			name:            "normal version",
			targetVersion:   "v1.2.3",
			expectedVersion: semver.Must(version.EnsureSemver("v1.2.3")),
			assertErr:       require.NoError,
		},
		{
			name:            "no version",
			targetVersion:   constants.NoVersion,
			expectedVersion: nil,
			assertErr:       require.Error,
		},
		{
			name:            "version too high",
			targetVersion:   "v99.1.1",
			expectedVersion: teleport.SemVer(),
			assertErr:       require.NoError,
		},
		{
			name:           "version invalid",
			targetVersion:  "foobar",
			assertCheckErr: require.Error,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ch := Channel{StaticVersion: tt.targetVersion}
			if tt.assertCheckErr != nil {
				tt.assertCheckErr(t, ch.CheckAndSetDefaults())
			} else {
				require.NoError(t, ch.CheckAndSetDefaults())
				result, err := ch.GetVersion(ctx)

				tt.assertErr(t, err)
				require.Equal(t, tt.expectedVersion, result)
			}
		})
	}
}

func TestNewDefaultChannel(t *testing.T) {
	channel, err := NewDefaultChannel()
	require.NoError(t, err)
	// Default channel must return teleport version
	require.Equal(t, teleport.Version, channel.StaticVersion)
	require.False(t, channel.Critical)
	// And the default channel must be initialized
	require.NotNil(t, channel.versionGetter)
	require.NotNil(t, channel.criticalTrigger)
}
