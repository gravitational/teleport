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
	"cmp"
	"context"
	"errors"
	"net"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/jonboulle/clockwork"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	vnetv1 "github.com/gravitational/teleport/gen/proto/go/teleport/lib/vnet/v1"
	"github.com/gravitational/teleport/lib/utils"
)

// statsSamplingInterval is how often the collector folds bytes transferred
// over active connections into the cumulative per-target counters and
// recomputes throughput.
const statsSamplingInterval = time.Second

const (
	// maxRecordsPerTarget is how many connection records are kept for a single
	// target before the oldest finished one is evicted.
	maxRecordsPerTarget = 50
	// maxRecordsGlobal is how many connection records are kept in total before
	// the oldest finished one is evicted.
	maxRecordsGlobal = 1000
)

// statsKey identifies a single target for connection statistics aggregation.
// Two connections with the same key are aggregated into the same counters.
type statsKey struct {
	kind        vnetv1.ConnectionKind
	profile     string
	leafCluster string
	displayName string
	// port is the target port of the connection. Only set for multi-port TCP
	// apps, zero otherwise, so that single-port targets aggregate into a
	// single entry no matter the port dialed.
	port uint16
}

// targetAgg accumulates connection statistics for a single target. All
// counters are absolute values accumulated since VNet started, except the
// throughput values which cover the most recent sampling interval.
type targetAgg struct {
	successfulConns uint64
	failedConns     uint64
	bytesTx         uint64
	bytesRx         uint64
	bytesTxPerSec   uint64
	bytesRxPerSec   uint64
}

// trackedConn is an established connection whose transferred bytes are
// tracked. lastWritten/lastRead hold the byte counts that have already been
// folded into the cumulative per-target counters at the previous sample.
type trackedConn struct {
	key         statsKey
	tc          *utils.TrackingConn
	lastWritten uint64
	lastRead    uint64
	// record is the connection record this conn belongs to, if one was created.
	// It is cleared once the conn is closed and its final byte counts have been
	// copied over to the record.
	record *connRecord
}

// connRecord tracks a single individual connection through its lifecycle.
type connRecord struct {
	id        uint64
	key       statsKey
	localPort uint16
	process   string
	startedAt time.Time
	// endedAt is zero while the connection is active.
	endedAt time.Time
	// tracked is set while the connection is active, so byte counts can be read
	// live from it. Once the conn is closed the final counts are copied to
	// bytesTx/bytesRx and this is cleared.
	tracked          *trackedConn
	bytesTx, bytesRx uint64
	state            vnetv1.ConnectionRecordState
	errMsg           string
}

// bytes returns the bytes transferred over the connection, read live from the
// tracked conn while the connection is still active.
func (r *connRecord) bytes() (tx, rx uint64) {
	if r.tracked != nil {
		written, read := r.tracked.tc.Stat()
		return read, written
	}
	return r.bytesTx, r.bytesRx
}

// reportConnectionsFunc reports a snapshot of VNet connection activity to the
// client application.
type reportConnectionsFunc func(ctx context.Context, report *vnetv1.ConnectionsReport) error

// statsCollector aggregates per-target connection statistics and keeps a capped
// window of individual connection records for all connections handled by VNet
// in the admin process. TCP handlers report connection attempts through
// [statsCollector.begin], and a sampler periodically folds bytes transferred
// over active connections into the cumulative counters via [statsCollector.run]
// and reports fresh snapshots to the client application.
type statsCollector struct {
	clock clockwork.Clock
	// report, if set, is called with a fresh snapshot whenever VNet connection
	// activity changed since the last successful report.
	report reportConnectionsFunc
	// lastReported is the last successfully reported snapshot. It is only
	// accessed from the [statsCollector.run] goroutine.
	lastReported *vnetv1.ConnectionsReport
	// nextRecordID hands out connection record IDs. It is incremented when a
	// connection attempt starts, so IDs order records by connection start time.
	// IDs of attempts that never produce a record are skipped.
	nextRecordID atomic.Uint64

	// mu guards agg, active, records and recordsPerTarget.
	mu     sync.Mutex
	agg    map[statsKey]*targetAgg
	active map[*trackedConn]struct{}
	// records holds the connection records currently kept, in creation order.
	records []*connRecord
	// recordsPerTarget counts the records in records per target, to enforce the
	// per-target cap without scanning.
	recordsPerTarget map[statsKey]int
}

func newStatsCollector(clock clockwork.Clock, report reportConnectionsFunc) *statsCollector {
	return &statsCollector{
		clock:            clock,
		report:           report,
		agg:              make(map[statsKey]*targetAgg),
		active:           make(map[*trackedConn]struct{}),
		recordsPerTarget: make(map[statsKey]int),
	}
}

// begin starts tracking a single connection attempt to the target identified
// by key. localPort is the port the client application dialed. Callers must
// call [connAttempt.finish] with the handler's final error once the attempt
// has ended.
func (c *statsCollector) begin(ctx context.Context, key statsKey, localPort uint16) *connAttempt {
	return &connAttempt{
		c:         c,
		id:        c.nextRecordID.Add(1),
		key:       key,
		localPort: localPort,
		process:   clientProcessPathFromContext(ctx),
		startedAt: c.clock.Now(),
	}
}

// connAttempt tracks a single connection attempt from start to finish. It is
// not safe for concurrent use: handlers call the connector, [success], and
// [finish] sequentially in a single goroutine.
type connAttempt struct {
	c           *statsCollector
	id          uint64
	key         statsKey
	localPort   uint16
	process     string
	startedAt   time.Time
	established bool
	// tracked is the downstream conn, set once it has been accepted.
	tracked *trackedConn
	// record is the record for this connection, set once the attempt resolved
	// into either an established or a failed connection.
	record *connRecord
}

// instrument wraps connector so that when it succeeds the attempt is counted
// as a successful connection and bytes transferred over the returned conn are
// tracked. Suitable for handlers that establish the upstream connection
// before calling the downstream connector, so that connector success implies
// the connection to the target has been fully established.
func (a *connAttempt) instrument(connector func() (net.Conn, error)) func() (net.Conn, error) {
	track := a.track(connector)
	return func() (net.Conn, error) {
		conn, err := track()
		if err != nil {
			return nil, err
		}
		a.success()
		return conn, nil
	}
}

// track wraps connector so that bytes transferred over the returned conn are
// tracked, without counting the attempt as successful. Suitable for handlers
// that establish the connection to the target only after accepting the
// downstream connection, which must call [connAttempt.success] separately
// once the connection is fully established.
func (a *connAttempt) track(connector func() (net.Conn, error)) func() (net.Conn, error) {
	return func() (net.Conn, error) {
		conn, err := connector()
		if err != nil {
			return nil, err
		}
		trackedConn, tracked := a.c.trackConn(a.key, conn)
		a.tracked = tracked
		return trackedConn, nil
	}
}

// success counts the attempt as a successfully established connection and
// records it as active. It is idempotent.
func (a *connAttempt) success() {
	if a.established {
		return
	}
	a.established = true
	a.c.mu.Lock()
	defer a.c.mu.Unlock()
	a.c.aggLocked(a.key).successfulConns++

	a.record = a.newRecord(vnetv1.ConnectionRecordState_CONNECTION_RECORD_STATE_ACTIVE)
	// Point the record and the conn at each other so the record can report live
	// byte counts and the conn can finalize them when it's closed.
	a.record.tracked = a.tracked
	if a.tracked != nil {
		a.tracked.record = a.record
	}
	a.c.addRecordLocked(a.record)
}

// finish must be called with the handler's final error after the attempt has
// ended.
//
// A connection that was established is recorded as done. A connection that was
// never established is counted and recorded as failed, unless it ended with a
// benign context cancelation: the client going away is not a failure of the
// target, and is not worth a record.
func (a *connAttempt) finish(err error) {
	a.c.mu.Lock()
	defer a.c.mu.Unlock()
	now := a.c.clock.Now()

	if a.established {
		// An established connection counts as a success no matter how it ended.
		// Whatever error it ends with is a teardown error, one side closing the
		// socket: an EOF, a reset from the client going away (e.g. iperf3 killed
		// with Ctrl-C), a canceled context. None of it is a failure of the
		// target, and which teardown errors even surface is not deterministic
		// (the proxy filters an EOF but not a reset), so it is not recorded.
		// Errors are only recorded for connections that never established.
		if a.record == nil {
			return
		}
		a.record.endedAt = now
		a.record.state = vnetv1.ConnectionRecordState_CONNECTION_RECORD_STATE_DONE
		return
	}

	if err == nil || errors.Is(err, context.Canceled) {
		return
	}
	a.c.aggLocked(a.key).failedConns++
	a.record = a.newRecord(vnetv1.ConnectionRecordState_CONNECTION_RECORD_STATE_FAILED)
	a.record.endedAt = now
	a.record.errMsg = err.Error()
	if a.tracked != nil {
		// The downstream conn was accepted before the attempt failed, and has
		// been closed by now, so its counts are final.
		a.record.bytesTx, a.record.bytesRx = a.tracked.lastRead, a.tracked.lastWritten
	}
	a.c.addRecordLocked(a.record)
}

func (a *connAttempt) newRecord(state vnetv1.ConnectionRecordState) *connRecord {
	return &connRecord{
		id:        a.id,
		key:       a.key,
		localPort: a.localPort,
		process:   a.process,
		startedAt: a.startedAt,
		state:     state,
	}
}

// trackConn wraps conn so bytes transferred over it are periodically folded
// into the cumulative counters for key, until the conn is closed.
func (c *statsCollector) trackConn(key statsKey, conn net.Conn) (net.Conn, *trackedConn) {
	tracked := &trackedConn{key: key, tc: utils.NewTrackingConn(conn)}
	c.mu.Lock()
	c.active[tracked] = struct{}{}
	c.mu.Unlock()
	return &statsConn{
		TrackingConn: tracked.tc,
		onClose:      func() { c.closeTracked(tracked) },
	}, tracked
}

// closeTracked folds any remaining bytes transferred over the conn into the
// cumulative counters and into its record, and stops tracking it.
func (c *statsCollector) closeTracked(t *trackedConn) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.foldLocked(t)
	delete(c.active, t)
	if t.record != nil {
		t.record.bytesTx, t.record.bytesRx = t.lastRead, t.lastWritten
		t.record.tracked = nil
		t.record = nil
	}
}

// addRecordLocked appends a new record and evicts old ones if it pushed the
// collector over either cap.
func (c *statsCollector) addRecordLocked(rec *connRecord) {
	c.records = append(c.records, rec)
	c.recordsPerTarget[rec.key]++
	// Never evict the record just added, it is the newest one.
	if c.recordsPerTarget[rec.key] > maxRecordsPerTarget {
		c.evictOldestLocked(func(r *connRecord) bool { return r != rec && r.key == rec.key })
	}
	if len(c.records) > maxRecordsGlobal {
		c.evictOldestLocked(func(r *connRecord) bool { return r != rec })
	}
}

// evictOldestLocked removes the oldest finished record matching match. Active
// connections are never evicted, so a cap may be exceeded while more than that
// many connections to a target are open at once.
func (c *statsCollector) evictOldestLocked(match func(*connRecord) bool) {
	for i, rec := range c.records {
		if rec.state == vnetv1.ConnectionRecordState_CONNECTION_RECORD_STATE_ACTIVE || !match(rec) {
			continue
		}
		c.records = slices.Delete(c.records, i, i+1)
		c.recordsPerTarget[rec.key]--
		if c.recordsPerTarget[rec.key] == 0 {
			delete(c.recordsPerTarget, rec.key)
		}
		return
	}
}

// foldLocked folds bytes transferred over the conn since the previous sample
// into the cumulative per-target counters and returns the deltas.
//
// The downstream conn connects the client application to VNet: bytes VNet
// reads from it head to the target (TX) and bytes VNet writes to it came from
// the target (RX).
func (c *statsCollector) foldLocked(t *trackedConn) (dTx, dRx uint64) {
	written, read := t.tc.Stat()
	dTx = read - t.lastRead
	dRx = written - t.lastWritten
	t.lastRead, t.lastWritten = read, written
	agg := c.aggLocked(t.key)
	agg.bytesTx += dTx
	agg.bytesRx += dRx
	return dTx, dRx
}

func (c *statsCollector) aggLocked(key statsKey) *targetAgg {
	agg, ok := c.agg[key]
	if !ok {
		agg = &targetAgg{}
		c.agg[key] = agg
	}
	return agg
}

// sample folds bytes transferred over all active connections into the
// cumulative counters and recomputes per-target throughput over the elapsed
// interval.
func (c *statsCollector) sample(interval time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	type delta struct{ tx, rx uint64 }
	deltas := make(map[statsKey]delta, len(c.active))
	for t := range c.active {
		dTx, dRx := c.foldLocked(t)
		d := deltas[t.key]
		d.tx += dTx
		d.rx += dRx
		deltas[t.key] = d
	}
	secs := interval.Seconds()
	for key, agg := range c.agg {
		d := deltas[key]
		agg.bytesTxPerSec = uint64(float64(d.tx) / secs)
		agg.bytesRxPerSec = uint64(float64(d.rx) / secs)
	}
}

// snapshot returns the current snapshot of VNet connection activity. The
// statistics and the records are taken under the same lock, so they always
// describe the same instant.
func (c *statsCollector) snapshot() *vnetv1.ConnectionsReport {
	c.mu.Lock()
	defer c.mu.Unlock()
	return vnetv1.ConnectionsReport_builder{
		Stats:       c.snapshotStatsLocked(),
		Connections: c.snapshotRecordsLocked(),
		CollectedAt: timestamppb.New(c.clock.Now()),
	}.Build()
}

// snapshotRecordsLocked returns the connection records currently kept, ordered
// newest-first. Records for active connections carry live byte counts.
func (c *statsCollector) snapshotRecordsLocked() []*vnetv1.ConnectionRecord {
	records := make([]*vnetv1.ConnectionRecord, 0, len(c.records))
	for _, rec := range c.records {
		bytesTx, bytesRx := rec.bytes()
		b := vnetv1.ConnectionRecord_builder{
			Id:                rec.id,
			Kind:              rec.key.kind,
			Profile:           rec.key.profile,
			LeafCluster:       rec.key.leafCluster,
			DisplayName:       rec.key.displayName,
			Port:              uint32(rec.key.port),
			LocalPort:         uint32(rec.localPort),
			ClientProcessPath: rec.process,
			StartedAt:         timestamppb.New(rec.startedAt),
			BytesTx:           bytesTx,
			BytesRx:           bytesRx,
			State:             rec.state,
			ErrorMessage:      rec.errMsg,
		}
		if !rec.endedAt.IsZero() {
			b.EndedAt = timestamppb.New(rec.endedAt)
		}
		records = append(records, b.Build())
	}
	// IDs are handed out when a connection starts, so ordering by descending ID
	// puts the most recently started connection first.
	slices.SortFunc(records, func(a, b *vnetv1.ConnectionRecord) int {
		return cmp.Compare(b.GetId(), a.GetId())
	})
	return records
}

// snapshotStatsLocked returns the current statistics for all targets connected
// to since VNet started, in a stable order.
func (c *statsCollector) snapshotStatsLocked() []*vnetv1.ConnectionStat {
	stats := make([]*vnetv1.ConnectionStat, 0, len(c.agg))
	for key, agg := range c.agg {
		stats = append(stats, vnetv1.ConnectionStat_builder{
			Kind:                  key.kind,
			Profile:               key.profile,
			LeafCluster:           key.leafCluster,
			DisplayName:           key.displayName,
			Port:                  uint32(key.port),
			SuccessfulConnections: agg.successfulConns,
			FailedConnections:     agg.failedConns,
			BytesTx:               agg.bytesTx,
			BytesRx:               agg.bytesRx,
			BytesTxPerSec:         agg.bytesTxPerSec,
			BytesRxPerSec:         agg.bytesRxPerSec,
		}.Build())
	}
	slices.SortFunc(stats, func(a, b *vnetv1.ConnectionStat) int {
		return cmp.Or(
			cmp.Compare(a.GetKind(), b.GetKind()),
			strings.Compare(a.GetProfile(), b.GetProfile()),
			strings.Compare(a.GetLeafCluster(), b.GetLeafCluster()),
			strings.Compare(a.GetDisplayName(), b.GetDisplayName()),
			cmp.Compare(a.GetPort(), b.GetPort()),
		)
	})
	return stats
}

// run periodically samples all active connections to keep the cumulative
// counters and throughput up to date, and reports fresh snapshots to the
// client application. It returns when ctx is canceled.
func (c *statsCollector) run(ctx context.Context) {
	ticker := c.clock.NewTicker(statsSamplingInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.Chan():
			c.sample(statsSamplingInterval)
			c.push(ctx)
		}
	}
}

// push reports the current snapshot if VNet connection activity changed since
// the last successful report. Snapshots are complete and self-contained so a
// failed report loses no data, the next successful one catches the client
// application up.
func (c *statsCollector) push(ctx context.Context) {
	if c.report == nil {
		return
	}
	report := c.snapshot()
	if reportsEqual(report, c.lastReported) {
		return
	}
	if err := c.report(ctx, report); err != nil {
		log.DebugContext(ctx, "Failed to report connections to the client application", "error", err)
		return
	}
	c.lastReported = report
}

// reportsEqual reports whether two snapshots hold the same connection activity.
// The collection time is deliberately not compared: it advances on every sample
// and would make every snapshot look like a change. A nil report is treated as
// an empty one, so an empty snapshot compares equal to "nothing reported yet".
func reportsEqual(a, b *vnetv1.ConnectionsReport) bool {
	return slices.EqualFunc(a.GetStats(), b.GetStats(), func(a, b *vnetv1.ConnectionStat) bool {
		return proto.Equal(a, b)
	}) && slices.EqualFunc(a.GetConnections(), b.GetConnections(), func(a, b *vnetv1.ConnectionRecord) bool {
		return proto.Equal(a, b)
	})
}

// statsConn wraps a tracked conn to stop tracking it when it's closed.
type statsConn struct {
	*utils.TrackingConn
	closeOnce sync.Once
	onClose   func()
}

func (c *statsConn) Close() error {
	err := c.TrackingConn.Close()
	c.closeOnce.Do(c.onClose)
	return err
}

// statsDisplayName returns addr if set, otherwise the fallback (the resource
// name), matching the display address reported on the connection callbacks.
func statsDisplayName(addr, fallback string) string {
	if addr != "" {
		return addr
	}
	return fallback
}
