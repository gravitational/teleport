/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package lookup

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/coreos/go-semver/semver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/automaticupgrades"
	"github.com/gravitational/teleport/lib/automaticupgrades/constants"
	"github.com/gravitational/teleport/lib/automaticupgrades/version"
)

type mockRFD109VersionServer struct {
	t        *testing.T
	channels map[string]channelStub
}

type channelStub struct {
	// with our without the leading "v"
	version  string
	critical bool
	fail     bool
}

func (m *mockRFD109VersionServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var path string
	var writeResp func(w http.ResponseWriter, stub channelStub) error

	switch {
	case strings.HasSuffix(r.URL.Path, constants.VersionPath):
		path = strings.Trim(strings.TrimSuffix(r.URL.Path, constants.VersionPath), "/")
		writeResp = func(w http.ResponseWriter, stub channelStub) error {
			_, err := w.Write([]byte(stub.version))
			return err
		}
	case strings.HasSuffix(r.URL.Path, constants.MaintenancePath):
		path = strings.Trim(strings.TrimSuffix(r.URL.Path, constants.MaintenancePath), "/")
		writeResp = func(w http.ResponseWriter, stub channelStub) error {
			response := "no"
			if stub.critical {
				response = "yes"
			}
			_, err := w.Write([]byte(response))
			return err
		}
	default:
		assert.Fail(m.t, "unsupported path %q", r.URL.Path)
		w.WriteHeader(http.StatusNotFound)
		return
	}

	channel, ok := m.channels[path]
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		assert.Fail(m.t, "channel %q not found", path)
		return
	}
	if channel.fail {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	assert.NoError(m.t, writeResp(w, channel), "failed to write response")
}

func TestGetVersionFromChannel(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	channelName := "test-channel"

	mock := mockRFD109VersionServer{
		t: t,
		channels: map[string]channelStub{
			"broken":            {fail: true},
			"with-leading-v":    {version: "v" + testVersionHigh},
			"without-leading-v": {version: testVersionHigh},
			"low":               {version: testVersionLow},
		},
	}
	srv := httptest.NewServer(http.HandlerFunc(mock.ServeHTTP))
	t.Cleanup(srv.Close)

	testVersion, err := version.EnsureSemver(testVersionHigh)
	require.NoError(t, err)

	tests := []struct {
		name           string
		channels       automaticupgrades.Channels
		expectedResult *semver.Version
		expectError    require.ErrorAssertionFunc
	}{
		{
			name: "channel with leading v",
			channels: automaticupgrades.Channels{
				channelName: {ForwardURL: srv.URL + "/with-leading-v"},
				"default":   {ForwardURL: srv.URL + "/low"},
			},
			expectedResult: testVersion,
			expectError:    require.NoError,
		},
		{
			name: "channel without leading v",
			channels: automaticupgrades.Channels{
				channelName: {ForwardURL: srv.URL + "/without-leading-v"},
				"default":   {ForwardURL: srv.URL + "/low"},
			},
			expectedResult: testVersion,
			expectError:    require.NoError,
		},
		{
			name: "fallback to default with leading v",
			channels: automaticupgrades.Channels{
				"default": {ForwardURL: srv.URL + "/with-leading-v"},
			},
			expectedResult: testVersion,
			expectError:    require.NoError,
		},
		{
			name: "fallback to default without leading v",
			channels: automaticupgrades.Channels{
				"default": {ForwardURL: srv.URL + "/without-leading-v"},
			},
			expectedResult: testVersion,
			expectError:    require.NoError,
		},
		{
			name: "broken channel",
			channels: automaticupgrades.Channels{
				channelName: {ForwardURL: srv.URL + "/broken"},
				"default":   {ForwardURL: srv.URL + "/without-leading-v"},
			},
			expectError: require.Error,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test setup
			require.NoError(t, tt.channels.CheckAndSetDefaults())

			// Test execution
			result, err := getVersionFromChannel(ctx, tt.channels, channelName)
			tt.expectError(t, err)
			require.Equal(t, tt.expectedResult, result)
		})
	}
}

func TestGetTriggerFromChannel(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	channelName := "test-channel"

	mock := mockRFD109VersionServer{
		t: t,
		channels: map[string]channelStub{
			"broken":       {fail: true},
			"critical":     {critical: true},
			"non-critical": {critical: false},
		},
	}
	srv := httptest.NewServer(http.HandlerFunc(mock.ServeHTTP))
	t.Cleanup(srv.Close)

	tests := []struct {
		name           string
		channels       automaticupgrades.Channels
		expectedResult bool
		expectError    require.ErrorAssertionFunc
	}{
		{
			name: "critical channel",
			channels: automaticupgrades.Channels{
				channelName: {ForwardURL: srv.URL + "/critical"},
				"default":   {ForwardURL: srv.URL + "/non-critical"},
			},
			expectedResult: true,
			expectError:    require.NoError,
		},
		{
			name: "non-critical channel",
			channels: automaticupgrades.Channels{
				channelName: {ForwardURL: srv.URL + "/non-critical"},
				"default":   {ForwardURL: srv.URL + "/critical"},
			},
			expectedResult: false,
			expectError:    require.NoError,
		},
		{
			name: "fallback to default which is critical",
			channels: automaticupgrades.Channels{
				"default": {ForwardURL: srv.URL + "/critical"},
			},
			expectedResult: true,
			expectError:    require.NoError,
		},
		{
			name: "fallback to default which is non-critical",
			channels: automaticupgrades.Channels{
				"default": {ForwardURL: srv.URL + "/non-critical"},
			},
			expectedResult: false,
			expectError:    require.NoError,
		},
		{
			name: "broken channel",
			channels: automaticupgrades.Channels{
				channelName: {ForwardURL: srv.URL + "/broken"},
				"default":   {ForwardURL: srv.URL + "/critical"},
			},
			expectError: require.Error,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test setup
			require.NoError(t, tt.channels.CheckAndSetDefaults())

			// Test execution
			result, err := getTriggerFromChannel(ctx, tt.channels, channelName)
			tt.expectError(t, err)
			require.Equal(t, tt.expectedResult, result)
		})
	}
}
