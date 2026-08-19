/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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
	"io"
	"net"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
)

// closeTrackingConn is a net.Conn that records whether Close was called.
type closeTrackingConn struct {
	net.Conn
	closed atomic.Bool
}

func (c *closeTrackingConn) Close() error {
	c.closed.Store(true)
	return nil
}

// TestSPDYFailedUpgrade ensures that when a SPDY upgrade request fails
// (RBAC rejection, pod doesn't exist, etc) the connection is closed.
func TestSPDYFailedUpgrade(t *testing.T) {
	conn := &closeTrackingConn{}
	rt := &SpdyRoundTripper{conn: conn}

	resp := &http.Response{
		StatusCode: http.StatusForbidden,
		Header:     http.Header{},
		Body: io.NopCloser(strings.NewReader(
			`{"kind":"Status","apiVersion":"v1","status":"Failure","message":"forbidden","code":403}`,
		)),
	}

	upgraded, err := rt.NewConnection(resp)
	require.Error(t, err, "a non-101 response must be rejected")
	require.Nil(t, upgraded)
	require.True(t, conn.closed.Load(), "dialed connection must be closed when the SPDY upgrade is rejected")
}
