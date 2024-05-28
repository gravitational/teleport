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
