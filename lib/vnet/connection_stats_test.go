// Teleport
// Copyright (C) 2026 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package vnet

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	vnetv1 "github.com/gravitational/teleport/gen/proto/go/teleport/lib/vnet/v1"
)

// nopConn is a fake net.Conn where reads and writes always fully succeed, so
// tests have exact control over the number of bytes transferred.
type nopConn struct{ net.Conn }

func (nopConn) Read(b []byte) (int, error)  { return len(b), nil }
func (nopConn) Write(b []byte) (int, error) { return len(b), nil }
func (nopConn) Close() error                { return nil }

func testStatsKey(displayName string) statsKey {
	return statsKey{
		kind:        vnetv1.ConnectionKind_CONNECTION_KIND_APP,
		profile:     "root.example.com",
		displayName: displayName,
	}
}

func nopConnector() (net.Conn, error) { return nopConn{}, nil }

func TestStatsCollector_successfulAndFailedConnections(t *testing.T) {
	c := newStatsCollector(clockwork.NewFakeClock())
	key := testStatsKey("app.root.example.com")

	// A connection that establishes and later ends without an error.
	att := c.begin(key)
	_, err := att.instrument(nopConnector)()
	require.NoError(t, err)
	att.finish(nil)

	// A connection that establishes and later ends with an error is still
	// counted as successful, it's not a failure to establish.
	att = c.begin(key)
	_, err = att.instrument(nopConnector)()
	require.NoError(t, err)
	att.finish(trace.Errorf("conn reset"))

	// A connection that fails before establishing.
	att = c.begin(key)
	att.finish(trace.Errorf("proxy dial failed"))

	// A connection canceled before establishing is not a failure of the
	// target.
	att = c.begin(key)
	att.finish(trace.Wrap(context.Canceled))

	stats := c.snapshot()
	require.Len(t, stats, 1)
	require.Equal(t, uint64(2), stats[0].GetSuccessfulConnections())
	require.Equal(t, uint64(1), stats[0].GetFailedConnections())
}

func TestStatsCollector_trackWithExplicitSuccess(t *testing.T) {
	c := newStatsCollector(clockwork.NewFakeClock())
	key := testStatsKey("ssh.root.example.com")

	// track alone does not count establishment, an error after the downstream
	// conn was accepted (e.g. a failed SSH handshake) is still a failure.
	att := c.begin(key)
	_, err := att.track(nopConnector)()
	require.NoError(t, err)
	att.finish(trace.Errorf("ssh handshake failed"))

	// With an explicit success call the attempt counts as established.
	att = c.begin(key)
	_, err = att.track(nopConnector)()
	require.NoError(t, err)
	att.success()
	att.finish(nil)

	stats := c.snapshot()
	require.Len(t, stats, 1)
	require.Equal(t, uint64(1), stats[0].GetSuccessfulConnections())
	require.Equal(t, uint64(1), stats[0].GetFailedConnections())
}

func TestStatsCollector_bytesAndThroughput(t *testing.T) {
	c := newStatsCollector(clockwork.NewFakeClock())
	key := testStatsKey("app.root.example.com")

	att := c.begin(key)
	conn, err := att.instrument(nopConnector)()
	require.NoError(t, err)

	// Bytes read from the downstream conn head to the target (tx), bytes
	// written to it came from the target (rx).
	_, err = conn.Read(make([]byte, 100))
	require.NoError(t, err)
	_, err = conn.Write(make([]byte, 40))
	require.NoError(t, err)

	c.sample(time.Second)
	stats := c.snapshot()
	require.Len(t, stats, 1)
	require.Equal(t, uint64(100), stats[0].GetBytesTx())
	require.Equal(t, uint64(40), stats[0].GetBytesRx())
	require.Equal(t, uint64(100), stats[0].GetBytesTxPerSec())
	require.Equal(t, uint64(40), stats[0].GetBytesRxPerSec())

	// An idle interval zeroes the throughput but keeps the totals.
	c.sample(time.Second)
	stats = c.snapshot()
	require.Equal(t, uint64(100), stats[0].GetBytesTx())
	require.Equal(t, uint64(40), stats[0].GetBytesRx())
	require.Zero(t, stats[0].GetBytesTxPerSec())
	require.Zero(t, stats[0].GetBytesRxPerSec())

	// Closing the conn folds any bytes transferred since the last sample and
	// stops tracking it.
	_, err = conn.Read(make([]byte, 10))
	require.NoError(t, err)
	require.NoError(t, conn.Close())
	require.Empty(t, c.active)

	c.sample(time.Second)
	stats = c.snapshot()
	require.Equal(t, uint64(110), stats[0].GetBytesTx())
	require.Equal(t, uint64(40), stats[0].GetBytesRx())

	att.finish(nil)
	require.Equal(t, uint64(1), c.snapshot()[0].GetSuccessfulConnections())
}

func TestStatsCollector_aggregatesPerTarget(t *testing.T) {
	c := newStatsCollector(clockwork.NewFakeClock())
	key := testStatsKey("app.root.example.com")
	otherKey := testStatsKey("other.root.example.com")

	for _, k := range []statsKey{key, key, otherKey} {
		att := c.begin(k)
		conn, err := att.instrument(nopConnector)()
		require.NoError(t, err)
		_, err = conn.Read(make([]byte, 25))
		require.NoError(t, err)
		att.finish(nil)
	}

	c.sample(time.Second)
	stats := c.snapshot()
	require.Len(t, stats, 2)
	// Snapshot order is stable, sorted by display name within the same kind
	// and profile.
	require.Equal(t, "app.root.example.com", stats[0].GetDisplayName())
	require.Equal(t, uint64(2), stats[0].GetSuccessfulConnections())
	require.Equal(t, uint64(50), stats[0].GetBytesTx())
	require.Equal(t, "other.root.example.com", stats[1].GetDisplayName())
	require.Equal(t, uint64(1), stats[1].GetSuccessfulConnections())
	require.Equal(t, uint64(25), stats[1].GetBytesTx())
}
