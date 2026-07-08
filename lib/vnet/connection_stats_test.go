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
	"fmt"
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
	c := newStatsCollector(clockwork.NewFakeClock(), nil)
	key := testStatsKey("app.root.example.com")

	// A connection that establishes and later ends without an error.
	att := c.begin(context.Background(), key, 1234)
	_, err := att.instrument(nopConnector)()
	require.NoError(t, err)
	att.finish(nil)

	// A connection that establishes and later ends with an error is still
	// counted as successful, it's not a failure to establish.
	att = c.begin(context.Background(), key, 1234)
	_, err = att.instrument(nopConnector)()
	require.NoError(t, err)
	att.finish(trace.Errorf("conn reset"))

	// A connection that fails before establishing.
	att = c.begin(context.Background(), key, 1234)
	att.finish(trace.Errorf("proxy dial failed"))

	// A connection canceled before establishing is not a failure of the
	// target.
	att = c.begin(context.Background(), key, 1234)
	att.finish(trace.Wrap(context.Canceled))

	stats := c.snapshot().GetStats()
	require.Len(t, stats, 1)
	require.Equal(t, uint64(2), stats[0].GetSuccessfulConnections())
	require.Equal(t, uint64(1), stats[0].GetFailedConnections())
}

func TestStatsCollector_trackWithExplicitSuccess(t *testing.T) {
	c := newStatsCollector(clockwork.NewFakeClock(), nil)
	key := testStatsKey("ssh.root.example.com")

	// track alone does not count establishment, an error after the downstream
	// conn was accepted (e.g. a failed SSH handshake) is still a failure.
	att := c.begin(context.Background(), key, 1234)
	_, err := att.track(nopConnector)()
	require.NoError(t, err)
	att.finish(trace.Errorf("ssh handshake failed"))

	// With an explicit success call the attempt counts as established.
	att = c.begin(context.Background(), key, 1234)
	_, err = att.track(nopConnector)()
	require.NoError(t, err)
	att.success()
	att.finish(nil)

	stats := c.snapshot().GetStats()
	require.Len(t, stats, 1)
	require.Equal(t, uint64(1), stats[0].GetSuccessfulConnections())
	require.Equal(t, uint64(1), stats[0].GetFailedConnections())
}

func TestStatsCollector_bytesAndThroughput(t *testing.T) {
	c := newStatsCollector(clockwork.NewFakeClock(), nil)
	key := testStatsKey("app.root.example.com")

	att := c.begin(context.Background(), key, 1234)
	conn, err := att.instrument(nopConnector)()
	require.NoError(t, err)

	// Bytes read from the downstream conn head to the target (tx), bytes
	// written to it came from the target (rx).
	_, err = conn.Read(make([]byte, 100))
	require.NoError(t, err)
	_, err = conn.Write(make([]byte, 40))
	require.NoError(t, err)

	c.sample(time.Second)
	stats := c.snapshot().GetStats()
	require.Len(t, stats, 1)
	require.Equal(t, uint64(100), stats[0].GetBytesTx())
	require.Equal(t, uint64(40), stats[0].GetBytesRx())
	require.Equal(t, uint64(100), stats[0].GetBytesTxPerSec())
	require.Equal(t, uint64(40), stats[0].GetBytesRxPerSec())

	// An idle interval zeroes the throughput but keeps the totals.
	c.sample(time.Second)
	stats = c.snapshot().GetStats()
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
	stats = c.snapshot().GetStats()
	require.Equal(t, uint64(110), stats[0].GetBytesTx())
	require.Equal(t, uint64(40), stats[0].GetBytesRx())

	att.finish(nil)
	require.Equal(t, uint64(1), c.snapshot().GetStats()[0].GetSuccessfulConnections())
}

func TestStatsCollector_recordLifecycle(t *testing.T) {
	clock := clockwork.NewFakeClock()
	c := newStatsCollector(clock, nil)
	key := testStatsKey("app.root.example.com")
	ctx := contextWithPeerProcess(context.Background(), peerProcess{PID: 1, ExePath: "/usr/bin/psql"})

	att := c.begin(ctx, key, 8080)
	conn, err := att.instrument(nopConnector)()
	require.NoError(t, err)
	_, err = conn.Read(make([]byte, 100))
	require.NoError(t, err)

	// While active the record carries live byte counts and no end time.
	records := c.snapshot().GetConnections()
	require.Len(t, records, 1)
	rec := records[0]
	require.Equal(t, vnetv1.ConnectionRecordState_CONNECTION_RECORD_STATE_ACTIVE, rec.GetState())
	require.Equal(t, "/usr/bin/psql", rec.GetClientProcessPath())
	require.Equal(t, uint32(8080), rec.GetLocalPort())
	// The target is a single-port app, so the dialed port is not part of its
	// identity.
	require.Zero(t, rec.GetPort())
	require.Equal(t, uint64(100), rec.GetBytesTx())
	require.Nil(t, rec.GetEndedAt())

	// Once the conn is closed and the attempt finished, the record is done and
	// its byte counts are final.
	clock.Advance(time.Second)
	require.NoError(t, conn.Close())
	att.finish(nil)

	records = c.snapshot().GetConnections()
	require.Len(t, records, 1)
	rec = records[0]
	require.Equal(t, vnetv1.ConnectionRecordState_CONNECTION_RECORD_STATE_DONE, rec.GetState())
	require.Equal(t, uint64(100), rec.GetBytesTx())
	require.NotNil(t, rec.GetEndedAt())
	require.Empty(t, rec.GetErrorMessage())
}

func TestStatsCollector_recordStates(t *testing.T) {
	c := newStatsCollector(clockwork.NewFakeClock(), nil)
	key := testStatsKey("app.root.example.com")
	ctx := context.Background()

	// Established, then ended with a mid-stream error: done, but the error is
	// kept.
	att := c.begin(ctx, key, 1234)
	_, err := att.instrument(nopConnector)()
	require.NoError(t, err)
	att.finish(trace.Errorf("conn reset"))

	// Never established: failed, with the reason.
	att = c.begin(ctx, key, 1234)
	att.finish(trace.Errorf("proxy dial failed"))

	// Canceled before establishing: not worth a record.
	att = c.begin(ctx, key, 1234)
	att.finish(trace.Wrap(context.Canceled))

	records := c.snapshot().GetConnections()
	require.Len(t, records, 2)
	// Newest first.
	require.Equal(t, vnetv1.ConnectionRecordState_CONNECTION_RECORD_STATE_FAILED, records[0].GetState())
	require.Contains(t, records[0].GetErrorMessage(), "proxy dial failed")
	require.Equal(t, vnetv1.ConnectionRecordState_CONNECTION_RECORD_STATE_DONE, records[1].GetState())
	require.Contains(t, records[1].GetErrorMessage(), "conn reset")
}

func TestStatsCollector_recordEviction(t *testing.T) {
	ctx := context.Background()

	t.Run("per target", func(t *testing.T) {
		c := newStatsCollector(clockwork.NewFakeClock(), nil)
		key := testStatsKey("app.root.example.com")
		for range maxRecordsPerTarget + 5 {
			att := c.begin(ctx, key, 1234)
			att.finish(trace.Errorf("nope"))
		}
		records := c.snapshot().GetConnections()
		require.Len(t, records, maxRecordsPerTarget)
		// The oldest records were evicted, the newest is first.
		require.Equal(t, uint64(maxRecordsPerTarget+5), records[0].GetId())
		require.Equal(t, uint64(6), records[len(records)-1].GetId())
	})

	t.Run("global", func(t *testing.T) {
		c := newStatsCollector(clockwork.NewFakeClock(), nil)
		// Spread records over enough targets that no per-target cap is hit, so
		// only the global cap evicts.
		targets := maxRecordsGlobal/maxRecordsPerTarget + 1
		for i := range targets {
			key := testStatsKey(fmt.Sprintf("app-%d.root.example.com", i))
			for range maxRecordsPerTarget {
				c.begin(ctx, key, 1234).finish(trace.Errorf("nope"))
			}
		}
		require.Len(t, c.snapshot().GetConnections(), maxRecordsGlobal)
	})

	t.Run("active connections are never evicted", func(t *testing.T) {
		c := newStatsCollector(clockwork.NewFakeClock(), nil)
		key := testStatsKey("app.root.example.com")

		// One active connection, then enough finished ones to overflow the cap.
		activeAtt := c.begin(ctx, key, 1234)
		_, err := activeAtt.instrument(nopConnector)()
		require.NoError(t, err)
		for range maxRecordsPerTarget + 5 {
			c.begin(ctx, key, 1234).finish(trace.Errorf("nope"))
		}

		records := c.snapshot().GetConnections()
		require.Len(t, records, maxRecordsPerTarget)
		// The active record survived, even though it is the oldest one.
		require.Equal(t, uint64(1), records[len(records)-1].GetId())
		require.Equal(t, vnetv1.ConnectionRecordState_CONNECTION_RECORD_STATE_ACTIVE, records[len(records)-1].GetState())
	})
}

func TestStatsCollector_push(t *testing.T) {
	var reports []*vnetv1.ConnectionsReport
	var reportErr error
	c := newStatsCollector(clockwork.NewFakeClock(), func(_ context.Context, report *vnetv1.ConnectionsReport) error {
		if reportErr != nil {
			return reportErr
		}
		reports = append(reports, report)
		return nil
	})
	ctx := context.Background()
	key := testStatsKey("app.root.example.com")

	// Nothing to report yet, an empty snapshot is not a change.
	c.push(ctx)
	require.Empty(t, reports)

	att := c.begin(context.Background(), key, 1234)
	_, err := att.instrument(nopConnector)()
	require.NoError(t, err)
	att.finish(nil)

	c.push(ctx)
	require.Len(t, reports, 1)

	// Unchanged statistics are not re-reported.
	c.push(ctx)
	require.Len(t, reports, 1)

	// A failed report is retried on the next push, snapshots carry absolute
	// values so nothing is lost.
	att = c.begin(context.Background(), key, 1234)
	_, err = att.instrument(nopConnector)()
	require.NoError(t, err)
	att.finish(nil)

	reportErr = trace.Errorf("client application unavailable")
	c.push(ctx)
	require.Len(t, reports, 1)

	reportErr = nil
	c.push(ctx)
	require.Len(t, reports, 2)
	require.Equal(t, uint64(2), reports[1].GetStats()[0].GetSuccessfulConnections())
	// The collection time is set on every snapshot but is not what makes a
	// snapshot count as changed.
	require.NotNil(t, reports[1].GetCollectedAt())
}

func TestStatsCollector_aggregatesPerTarget(t *testing.T) {
	c := newStatsCollector(clockwork.NewFakeClock(), nil)
	key := testStatsKey("app.root.example.com")
	otherKey := testStatsKey("other.root.example.com")

	for _, k := range []statsKey{key, key, otherKey} {
		att := c.begin(context.Background(), k, 1234)
		conn, err := att.instrument(nopConnector)()
		require.NoError(t, err)
		_, err = conn.Read(make([]byte, 25))
		require.NoError(t, err)
		att.finish(nil)
	}

	c.sample(time.Second)
	stats := c.snapshot().GetStats()
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
