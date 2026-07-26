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

package common

import (
	"context"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
)

// TestClusterConn_HoldersShareOneConnection verifies that
// concurrent holders share one dialed connection and the last release closes it.
func TestClusterConn_HoldersShareOneConnection(t *testing.T) {
	t.Parallel()

	dialer := &fakeClusterDialer{}
	conn := &clusterConn{dialer: dialer}

	cc1, release1, err := conn.Acquire(t.Context())
	require.NoError(t, err)
	cc2, release2, err := conn.Acquire(t.Context())
	require.NoError(t, err)
	require.Same(t, cc1, cc2)
	require.Len(t, dialer.conns, 1)

	release1()
	require.Equal(t, 0, dialer.conns[0].closes, "the connection must stay open while held")
	release2()
	require.Equal(t, 1, dialer.conns[0].closes, "the last release must close the connection")
}

// TestClusterConn_RedialAfterRelease verifies that
// a released connection is not reused. The next acquire dials a new one.
func TestClusterConn_RedialAfterRelease(t *testing.T) {
	t.Parallel()

	dialer := &fakeClusterDialer{}
	conn := &clusterConn{dialer: dialer}

	_, release, err := conn.Acquire(t.Context())
	require.NoError(t, err)
	release()

	_, release, err = conn.Acquire(t.Context())
	require.NoError(t, err)
	release()

	require.Len(t, dialer.conns, 2)
	require.Equal(t, 1, dialer.conns[0].closes)
	require.Equal(t, 1, dialer.conns[1].closes)
}

// TestClusterConn_ReleaseIsIdempotent verifies that
// a double release does not close the connection under the other holder.
func TestClusterConn_ReleaseIsIdempotent(t *testing.T) {
	t.Parallel()

	dialer := &fakeClusterDialer{}
	conn := &clusterConn{dialer: dialer}

	_, release1, err := conn.Acquire(t.Context())
	require.NoError(t, err)
	_, release2, err := conn.Acquire(t.Context())
	require.NoError(t, err)

	release1()
	release1()
	require.Equal(t, 0, dialer.conns[0].closes, "a double release must not close the connection under the other holder")

	release2()
	require.Equal(t, 1, dialer.conns[0].closes)
}

// TestClusterConn_DialErrorLeavesNoHolders verifies that
// a failed acquire does not count as a holder and the next acquire dials again.
func TestClusterConn_DialErrorLeavesNoHolders(t *testing.T) {
	t.Parallel()

	dialer := &fakeClusterDialer{err: trace.ConnectionProblem(nil, "dial failed")}
	conn := &clusterConn{dialer: dialer}

	_, _, err := conn.Acquire(t.Context())
	require.Error(t, err)

	dialer.err = nil
	cc, release, err := conn.Acquire(t.Context())
	require.NoError(t, err)
	require.NotNil(t, cc)
	release()
	require.Equal(t, 1, dialer.conns[0].closes, "the failed acquire must not count as a holder")
}

type fakeClusterDialer struct {
	// conns are the connections dialed so far.
	conns []*fakeKubeCertClient
	err   error
}

func (f *fakeClusterDialer) DialCluster(ctx context.Context) (kubeCertClient, error) {
	if f.err != nil {
		return nil, trace.Wrap(f.err)
	}
	conn := &fakeKubeCertClient{}
	f.conns = append(f.conns, conn)
	return conn, nil
}
