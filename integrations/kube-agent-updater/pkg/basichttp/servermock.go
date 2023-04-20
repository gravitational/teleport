/*
Copyright 2023 Gravitational, Inc.

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

package basichttp

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
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

// NewServerMock builds and returns a
func NewServerMock(path string) *ServerMock {
	mock := ServerMock{path: path}
	mock.Srv = httptest.NewServer(http.HandlerFunc(mock.ServeHTTP))
	return &mock
}
