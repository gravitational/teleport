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
	"testing/synctest"
	"time"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
)

// TestClusterConn_HoldersShareOneConnection verifies that
// concurrent holders share one dialed connection and it closes one linger after the last release.
func TestClusterConn_HoldersShareOneConnection(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		dialer := &fakeClusterDialer{}
		conn := &clusterConn{dialer: dialer}

		cc1, release1, err := conn.Acquire(t.Context())
		require.NoError(t, err)
		cc2, release2, err := conn.Acquire(t.Context())
		require.NoError(t, err)
		require.Same(t, cc1, cc2)
		require.Len(t, dialer.conns, 1)

		release1()
		time.Sleep(clusterConnLinger)
		synctest.Wait()
		require.Equal(t, 0, dialer.conns[0].closes, "the connection must stay open while held")

		release2()
		time.Sleep(clusterConnLinger)
		synctest.Wait()
		require.Equal(t, 1, dialer.conns[0].closes, "the connection must close one linger after the last release")
	})
}

// TestClusterConn_LingerReusesConnection verifies that
// an acquire within the linger reuses the released connection and cancels its pending close.
func TestClusterConn_LingerReusesConnection(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		dialer := &fakeClusterDialer{}
		conn := &clusterConn{dialer: dialer}

		cc1, release, err := conn.Acquire(t.Context())
		require.NoError(t, err)
		release()

		cc2, release, err := conn.Acquire(t.Context())
		require.NoError(t, err)
		require.Same(t, cc1, cc2, "an acquire within the linger must reuse the connection")
		require.Len(t, dialer.conns, 1, "an acquire within the linger must not dial")

		time.Sleep(2 * clusterConnLinger)
		synctest.Wait()
		require.Equal(t, 0, dialer.conns[0].closes, "the canceled close must not fire under the holder")

		release()
		time.Sleep(clusterConnLinger)
		synctest.Wait()
		require.Equal(t, 1, dialer.conns[0].closes)
	})
}

// TestClusterConn_RedialAfterLinger verifies that
// a connection idle past the linger closes and the next acquire dials a new one.
func TestClusterConn_RedialAfterLinger(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		dialer := &fakeClusterDialer{}
		conn := &clusterConn{dialer: dialer}

		_, release, err := conn.Acquire(t.Context())
		require.NoError(t, err)
		release()
		time.Sleep(clusterConnLinger)
		synctest.Wait()
		require.Equal(t, 1, dialer.conns[0].closes, "an idle connection must close after the linger")

		_, release, err = conn.Acquire(t.Context())
		require.NoError(t, err)
		require.Len(t, dialer.conns, 2, "an acquire after the linger must dial a new connection")

		release()
		time.Sleep(clusterConnLinger)
		synctest.Wait()
		require.Equal(t, 1, dialer.conns[1].closes)
	})
}

// TestClusterConn_ReleaseIsIdempotent verifies that
// a double release does not close the connection under the other holder.
func TestClusterConn_ReleaseIsIdempotent(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		dialer := &fakeClusterDialer{}
		conn := &clusterConn{dialer: dialer}

		_, release1, err := conn.Acquire(t.Context())
		require.NoError(t, err)
		_, release2, err := conn.Acquire(t.Context())
		require.NoError(t, err)

		release1()
		release1()
		time.Sleep(clusterConnLinger)
		synctest.Wait()
		require.Equal(t, 0, dialer.conns[0].closes, "a double release must not close the connection under the other holder")

		release2()
		time.Sleep(clusterConnLinger)
		synctest.Wait()
		require.Equal(t, 1, dialer.conns[0].closes)
	})
}

// TestClusterConn_DialErrorLeavesNoHolders verifies that
// a failed acquire does not count as a holder and the next acquire dials again.
func TestClusterConn_DialErrorLeavesNoHolders(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		dialer := &fakeClusterDialer{err: trace.ConnectionProblem(nil, "dial failed")}
		conn := &clusterConn{dialer: dialer}

		_, _, err := conn.Acquire(t.Context())
		require.Error(t, err)

		dialer.err = nil
		cc, release, err := conn.Acquire(t.Context())
		require.NoError(t, err)
		require.NotNil(t, cc)
		release()
		time.Sleep(clusterConnLinger)
		synctest.Wait()
		require.Equal(t, 1, dialer.conns[0].closes, "the failed acquire must not count as a holder")
	})
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
