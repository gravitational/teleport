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

package proxy

import (
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/integration/helpers"
	"github.com/gravitational/teleport/lib/automaticupgrades"
	"github.com/gravitational/teleport/lib/automaticupgrades/constants"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/utils"
)

func createProxyWithChannels(t *testing.T, channels automaticupgrades.Channels) string {
	require.NoError(t, channels.CheckAndSetDefaults())
	testDir := t.TempDir()

	cfg := helpers.InstanceConfig{
		ClusterName: "root.example.com",
		HostID:      uuid.New().String(),
		NodeName:    helpers.Loopback,
		Logger:      utils.NewSlogLoggerForTests(),
	}
	cfg.Listeners = helpers.SingleProxyPortSetup(t, &cfg.Fds)
	rc := helpers.NewInstance(t, cfg)

	var err error
	rcConf := servicecfg.MakeDefaultConfig()
	rcConf.DataDir = filepath.Join(testDir, "data")
	rcConf.Auth.Enabled = true
	rcConf.Proxy.Enabled = true
	rcConf.SSH.Enabled = false
	rcConf.Proxy.DisableWebInterface = true
	rcConf.Version = "v3"
	rcConf.Proxy.AutomaticUpgradesChannels = channels

	err = rc.CreateEx(t, nil, rcConf)
	require.NoError(t, err)
	err = rc.Start()
	require.NoError(t, err)
	t.Cleanup(func() {
		assert.NoError(t, rc.StopAll())
	})

	return cfg.Listeners.Web
}

// ServerMock is a HTTP server whose response can be controlled from the tests.
// This is used to mock external dependencies like s3 buckets or a remote HTTP server.
type ServerMock struct {
	Srv *httptest.Server

	t        *testing.T
	code     int
	response string
	path     string
}

// SetResponse sets the ServerMock's response.
func (m *ServerMock) SetResponse(t *testing.T, code int, response string) {
	m.t = t
	m.code = code
	m.response = response
}

// ServeHTTP implements the http.Handler interface.
func (m *ServerMock) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	require.Equal(m.t, m.path, r.URL.Path)
	w.WriteHeader(m.code)
	_, err := io.WriteString(w, m.response)
	require.NoError(m.t, err)
}

// NewServerMock builds a [ServerMock] that only
// responds to requests for the given path.
func NewServerMock(path string) *ServerMock {
	mock := ServerMock{path: path}
	mock.Srv = httptest.NewServer(http.HandlerFunc(mock.ServeHTTP))
	return &mock
}

func TestVersionServer(t *testing.T) {
	// Test setup
	ctx := t.Context()

	testVersion := "v12.2.6"
	testVersionMajorTooHigh := "v99.1.3"

	staticChannel := "static/ok"
	staticHighChannel := "static/high"
	staticNoVersionChannel := "static/none"
	forwardChannel := "forward/ok"
	forwardHighChannel := "forward/high"
	forwardNoVersionChannel := "forward/none"
	forwardPath := "/version-server/"

	upstreamServer := NewServerMock(forwardPath + constants.VersionPath)
	upstreamServer.SetResponse(t, http.StatusOK, testVersion)
	upstreamHighServer := NewServerMock(forwardPath + constants.VersionPath)
	upstreamHighServer.SetResponse(t, http.StatusOK, testVersionMajorTooHigh)
	upstreamNoVersionServer := NewServerMock(forwardPath + constants.VersionPath)
	upstreamNoVersionServer.SetResponse(t, http.StatusOK, constants.NoVersion)

	channels := automaticupgrades.Channels{
		staticChannel: {
			StaticVersion: testVersion,
		},
		staticHighChannel: {
			StaticVersion: testVersionMajorTooHigh,
		},
		staticNoVersionChannel: {
			StaticVersion: constants.NoVersion,
		},
		forwardChannel: {
			ForwardURL: upstreamServer.Srv.URL + forwardPath,
		},
		forwardHighChannel: {
			ForwardURL: upstreamHighServer.Srv.URL + forwardPath,
		},
		forwardNoVersionChannel: {
			ForwardURL: upstreamNoVersionServer.Srv.URL + forwardPath,
		},
	}

	proxyAddr := createProxyWithChannels(t, channels)

	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	httpClient := http.Client{Transport: tr}

	// Test execution
	tests := []struct {
		name             string
		channel          string
		expectedResponse string
	}{
		{
			name:             "static version OK",
			channel:          staticChannel,
			expectedResponse: testVersion,
		},
		{
			name:             "static version too high",
			channel:          staticHighChannel,
			expectedResponse: "v" + teleport.Version,
		},
		{
			name:             "static version none",
			channel:          staticNoVersionChannel,
			expectedResponse: constants.NoVersion,
		},
		{
			name:             "forward version OK",
			channel:          forwardChannel,
			expectedResponse: testVersion,
		},
		{
			name:             "forward version too high",
			channel:          forwardHighChannel,
			expectedResponse: "v" + teleport.Version,
		},
		{
			name:             "forward version none",
			channel:          forwardNoVersionChannel,
			expectedResponse: constants.NoVersion,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			channelUrl, err := url.Parse(
				fmt.Sprintf("https://%s/v1/webapi/automaticupgrades/channel/%s/version", proxyAddr, tt.channel),
			)
			require.NoError(t, err)

			req, err := http.NewRequestWithContext(ctx, http.MethodGet, channelUrl.String(), nil)
			require.NoError(t, err)
			res, err := httpClient.Do(req)
			require.NoError(t, err)
			defer res.Body.Close()

			body, err := io.ReadAll(res.Body)
			require.NoError(t, err)

			require.Equal(t, http.StatusOK, res.StatusCode)
			require.Equal(t, tt.expectedResponse, string(body))
		})
	}
}
func TestDefaultVersionServer(t *testing.T) {
	// Test setup
	ctx := t.Context()

	channels := automaticupgrades.Channels{}

	proxyAddr := createProxyWithChannels(t, channels)

	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	httpClient := http.Client{Transport: tr}

	// Test execution
	tests := []struct {
		name             string
		channel          string
		expectedResponse string
	}{
		{
			name:             "default channel is served",
			channel:          automaticupgrades.DefaultChannelName,
			expectedResponse: "v" + teleport.Version,
		},
		{
			name:             "cloud default channel is served",
			channel:          automaticupgrades.DefaultCloudChannelName,
			expectedResponse: "v" + teleport.Version,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			channelUrl, err := url.Parse(
				fmt.Sprintf("https://%s/v1/webapi/automaticupgrades/channel/%s/version", proxyAddr, tt.channel),
			)
			require.NoError(t, err)

			req, err := http.NewRequestWithContext(ctx, http.MethodGet, channelUrl.String(), nil)
			require.NoError(t, err)
			res, err := httpClient.Do(req)
			require.NoError(t, err)
			defer res.Body.Close()

			body, err := io.ReadAll(res.Body)
			require.NoError(t, err)

			require.Equal(t, http.StatusOK, res.StatusCode)
			require.Equal(t, tt.expectedResponse, string(body))
		})
	}
}
