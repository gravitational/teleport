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

package maintenance

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	basichttp2 "github.com/gravitational/teleport/lib/automaticupgrades/basichttp"
	"github.com/gravitational/teleport/lib/automaticupgrades/constants"
)

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

const basicHTTPTestPath = "/v1/cloud-stable"

func Test_basicHTTPMaintenanceClient_Get(t *testing.T) {
	mock := NewServerMock(basicHTTPTestPath + "/" + constants.MaintenancePath)
	t.Cleanup(mock.Srv.Close)
	serverURL, err := url.Parse(mock.Srv.URL)
	serverURL.Path = basicHTTPTestPath
	require.NoError(t, err)
	ctx := context.Background()

	tests := []struct {
		name       string
		statusCode int
		response   string
		expected   bool
		assertErr  require.ErrorAssertionFunc
	}{
		{
			name:       "all good - no maintenance",
			statusCode: http.StatusOK,
			response:   "no",
			expected:   false,
			assertErr:  require.NoError,
		},
		{
			name:       "all good - maintenance",
			statusCode: http.StatusOK,
			response:   "yes",
			expected:   true,
			assertErr:  require.NoError,
		},
		{
			name:       "all good with newline",
			statusCode: http.StatusOK,
			response:   "yes\n",
			expected:   true,
			assertErr:  require.NoError,
		},
		{
			name:       "invalid response",
			statusCode: http.StatusOK,
			response:   "hello",
			expected:   false,
			assertErr:  require.Error,
		},
		{
			name:       "empty",
			statusCode: http.StatusOK,
			response:   "",
			expected:   false,
			assertErr: func(t2 require.TestingT, err2 error, _ ...interface{}) {
				require.IsType(t2, &trace.BadParameterError{}, trace.Unwrap(err2))
			},
		},
		{
			name:       "non-200 response",
			statusCode: http.StatusInternalServerError,
			response:   "ERROR - SOMETHING WENT WRONG",
			expected:   false,
			assertErr:  require.Error,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := &basicHTTPMaintenanceClient{
				baseURL: serverURL,
				client:  &basichttp2.Client{Client: mock.Srv.Client()},
			}
			mock.SetResponse(t, tt.statusCode, tt.response)
			result, err := b.Get(ctx)
			tt.assertErr(t, err)
			require.Equal(t, tt.expected, result)
		})
	}
}
