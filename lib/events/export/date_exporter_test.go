/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

package export

import (
	"context"
	"fmt"
	"iter"
	"math/rand/v2"
	"slices"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	auditlogpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/auditlog/v1"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/itertools/stream"
)

// TestDateExporterBasics tests the basic functionality of the date exporter, with and
// without random flake.
func TestDateExporterBasics(t *testing.T) {
	t.Parallel()
	for _, batch := range []bool{false, true} {
		for _, randomFlake := range []bool{false, true} {
			t.Run(fmt.Sprintf("randomFlake=%v_batch=%v", randomFlake, batch), func(t *testing.T) {
				t.Parallel()
				testDateExporterBasics(t, randomFlake, batch)
			})
		}
	}
}

func testDateExporterBasics(t *testing.T, randomFlake bool, batch bool) {
	clt := newFakeClient()
	clt.setRandomFlake(randomFlake)

	now := time.Now().UTC()

	var exportedMu sync.Mutex
	var exported []*auditlogpb.ExportEventUnstructured
	var batchExported []*auditlogpb.EventUnstructured

	exportFn := func(ctx context.Context, event *auditlogpb.ExportEventUnstructured) error {
		exportedMu.Lock()
		defer exportedMu.Unlock()
		exported = append(exported, event)
		return nil
	}

	batchExportFn := func(ctx context.Context, events []*auditlogpb.EventUnstructured, resumeState BulkExportResumeState) error {
		exportedMu.Lock()
		defer exportedMu.Unlock()
		batchExported = append(batchExported, events...)
		return nil
	}

	getExported := func() []*auditlogpb.ExportEventUnstructured {
		exportedMu.Lock()
		defer exportedMu.Unlock()
		return slices.Clone(exported)
	}

	getBatchExported := func() []*auditlogpb.EventUnstructured {
		exportedMu.Lock()
		defer exportedMu.Unlock()
		return slices.Clone(batchExported)
	}

	idleCh := make(chan struct{})

	onIdleFn := func(ctx context.Context) {
		select {
		case idleCh <- struct{}{}:
		default:
		}
	}

	waitIdle := func(t *testing.T) {
		// wait for two ticks of idleness (first tick may correspond to a cycle that was finishing
		// as the new events were being added, second cycle will have a happens-after relationship to
		// this function being called).
		timeout := time.After(time.Second * 30)
		for range 2 {
			select {
			case <-idleCh:
			case <-timeout:
				require.FailNow(t, "timeout waiting for exporter to become idle")
			}
		}
	}
	cfg := DateExporterConfig{
		Client:       clt,
		Date:         now,
		OnIdle:       onIdleFn,
		Concurrency:  3,
		MaxBackoff:   time.Millisecond * 600,
		PollInterval: time.Millisecond * 200,
	}
	if batch {
		cfg.BatchExport = &BatchExportConfig{
			Callback: batchExportFn,
			MaxDelay: time.Microsecond,
		}
	} else {
		cfg.Export = exportFn
	}
	exporter, err := NewDateExporter(cfg)
	require.NoError(t, err)
	defer exporter.Close()

	// empty event set means the exporter should become idle almost
	// immediately.
	waitIdle(t)
	require.Empty(t, getExported())
	require.Empty(t, getBatchExported())

	var allEvents []*auditlogpb.ExportEventUnstructured
	var allBatchedEvents []*auditlogpb.EventUnstructured
	var allChunks []string
	// quickly add a bunch of chunks
	for range 30 {
		chunk, batchedEvents := makeEventChunk(t, now, 10)
		allEvents = append(allEvents, chunk...)
		allBatchedEvents = append(allBatchedEvents, batchedEvents...)
		chunkID := uuid.NewString()
		allChunks = append(allChunks, chunkID)
		clt.addChunk(now.Format(time.DateOnly), chunkID, chunk)
	}

	waitIdle(t)

	require.ElementsMatch(t, allChunks, exporter.GetState().Completed)
	if batch {
		require.ElementsMatch(t, allBatchedEvents, getBatchExported())
	} else {
		require.ElementsMatch(t, allEvents, getExported())
	}

	// process a second round of chunks to cover the case of new chunks being added
	// after non-trivial idleness.

	// note that we do a lot more events here just to make absolutely certain
	// that we're hitting a decent amount of random flake.
	for range 30 {
		chunk, batchedEvents := makeEventChunk(t, now, 10)
		allEvents = append(allEvents, chunk...)
		allBatchedEvents = append(allBatchedEvents, batchedEvents...)
		chunkID := uuid.NewString()
		allChunks = append(allChunks, chunkID)
		clt.addChunk(now.Format(time.DateOnly), chunkID, chunk)
	}

	waitIdle(t)

	require.ElementsMatch(t, allChunks, exporter.GetState().Completed)
	if batch {
		require.ElementsMatch(t, allBatchedEvents, getBatchExported())
	} else {
		require.ElementsMatch(t, allEvents, getExported())
	}

	// close the exporter
	exporter.Close()
	timeout := time.After(time.Second * 30)
	select {
	case <-exporter.Done():
	case <-timeout:
		require.FailNow(t, "timeout waiting for exporter to close")
	}

	// get the final state of the exporter
	cfg.PreviousState = exporter.GetState()

	// recreate exporter with state from previous run
	exporter, err = NewDateExporter(cfg)
	require.NoError(t, err)
	defer exporter.Close()

	waitIdle(t)

	// no additional events should have been exported
	if batch {
		require.ElementsMatch(t, allBatchedEvents, getBatchExported())
	} else {
		require.ElementsMatch(t, allEvents, getExported())
	}
	// new chunks should be consumed correctly
	for range 30 {
		chunk, batchedEvents := makeEventChunk(t, now, 10)
		allEvents = append(allEvents, chunk...)
		allBatchedEvents = append(allBatchedEvents, batchedEvents...)
		chunkID := uuid.NewString()
		allChunks = append(allChunks, chunkID)
		clt.addChunk(now.Format(time.DateOnly), chunkID, chunk)
	}

	waitIdle(t)

	require.ElementsMatch(t, allChunks, exporter.GetState().Completed)
	if batch {
		require.ElementsMatch(t, allBatchedEvents, getBatchExported())
	} else {
		require.ElementsMatch(t, allEvents, getExported())
	}
}

// TestDateExporterResume verifies non-trivial exporter resumption behavior, with and without
// random flake.
func TestDateExporterResume(t *testing.T) {
	t.Parallel()
	for _, randomFlake := range []bool{false, true} {
		t.Run(fmt.Sprintf("randomFlake=%v", randomFlake), func(t *testing.T) {
			t.Parallel()
			testDateExporterResume(t, randomFlake)
		})
	}
}

func testDateExporterResume(t *testing.T, randomFlake bool) {
	clt := newFakeClient()
	clt.setRandomFlake(randomFlake)

	now := time.Now().UTC()

	// export via unbuffered channel so that we can easily block/unblock export from
	// the main test routine.
	exportCH := make(chan *auditlogpb.ExportEventUnstructured)

	exportFn := func(ctx context.Context, event *auditlogpb.ExportEventUnstructured) error {
		select {
		case exportCH <- event:
		case <-ctx.Done():
			return trace.Wrap(ctx.Err())
		}
		return nil
	}

	idleCh := make(chan struct{})

	onIdleFn := func(ctx context.Context) {
		select {
		case idleCh <- struct{}{}:
		default:
		}
	}

	waitIdle := func(t *testing.T) {
		// wait for two ticks of idleness (first tick may correspond to a cycle that was finishing
		// as the new events were being added, second cycle will have a happens-after relationship to
		// this function being called).
		timeout := time.After(time.Second * 30)
		for range 2 {
			select {
			case <-idleCh:
			case <-timeout:
				require.FailNow(t, "timeout waiting for exporter to become idle")
			}
		}
	}

	exporter, err := NewDateExporter(DateExporterConfig{
		Client:       clt,
		Date:         now,
		Export:       exportFn,
		OnIdle:       onIdleFn,
		Concurrency:  3, // low concurrency to ensure that we have some in progress chunks
		MaxBackoff:   time.Millisecond * 600,
		PollInterval: time.Millisecond * 200,
	})
	require.NoError(t, err)
	defer exporter.Close()

	// empty event set means the exporter should become idle almost
	// immediately.
	waitIdle(t)

	var allEvents, gotEvents []*auditlogpb.ExportEventUnstructured
	// quickly add a bunch of chunks
	for range 10 {
		chunk, _ := makeEventChunk(t, now, 10)
		allEvents = append(allEvents, chunk...)
		chunkID := uuid.NewString()
		clt.addChunk(now.Format(time.DateOnly), chunkID, chunk)
	}

	// consume a large subset of events s.t. we have some completed
	// chunks, some in progress, and some not yet started (note that
	// to guarantee some in progress chunks, the number consumed must not
	// divide evenly by the chunk size).
	timeout := time.After(time.Second * 30)
	for i := range 47 {
		select {
		case evt := <-exportCH:
			gotEvents = append(gotEvents, evt)
		case <-timeout:
			require.FailNowf(t, "timeout waiting for event", "iteration=%d", i)
		}
	}

	// close the exporter and wait for it to finish so that
	// we can get the correct final state.
	exporter.Close()
	select {
	case <-exporter.Done():
	case <-time.After(time.Second * 30):
		require.FailNow(t, "timeout waiting for exporter to close")
	}

	// get the final state of the exporter
	state := exporter.GetState()

	// recreate exporter with state from previous run
	exporter, err = NewDateExporter(DateExporterConfig{
		Client:        clt,
		Date:          now,
		Export:        exportFn,
		OnIdle:        onIdleFn,
		PreviousState: state,
		Concurrency:   3,
		MaxBackoff:    time.Millisecond * 600,
		PollInterval:  time.Millisecond * 200,
	})
	require.NoError(t, err)
	defer exporter.Close()

	// consume remaining events
	for i := range 53 {
		select {
		case evt := <-exportCH:
			gotEvents = append(gotEvents, evt)
		case <-timeout:
			require.FailNowf(t, "timeout waiting for event", "iteration=%d", i)
		}
	}
	require.ElementsMatch(t, allEvents, gotEvents)

	// ensure that exporter becomes idle
	waitIdle(t)
}

// TestDateExporterReleasesStreamOnExportFailure verifies that an export stream is
// released when the export callback fails, rather than remaining open across retries.
func TestDateExporterReleasesStreamOnExportFailure(t *testing.T) {
	t.Parallel()
	for _, batch := range []bool{false, true} {
		t.Run(fmt.Sprintf("batch=%t", batch), func(t *testing.T) {
			t.Parallel()
			synctest.Test(t, func(t *testing.T) {
				clt := &streamTrackingClient{fakeClient: newFakeClient()}

				// the fake client keys chunks by UTC date, matching the exporter's timestamp conversion.
				now := time.Now().UTC()

				// use a chunk large enough that the stream always has events remaining
				// when the export callback fails.
				chunk, _ := makeEventChunk(t, now, 100)
				clt.addChunk(now.Format(time.DateOnly), uuid.NewString(), chunk)

				var failures atomic.Int64
				cfg := DateExporterConfig{
					Client:     clt,
					Date:       now,
					MaxBackoff: time.Minute,
				}
				if batch {
					cfg.BatchExport = &BatchExportConfig{
						Callback: func(ctx context.Context, events []*auditlogpb.EventUnstructured, resumeState BulkExportResumeState) error {
							failures.Add(1)
							return trace.Errorf("batch export failed as test condition")
						},
						// export a single event per batch so that the callback fails early in the stream.
						MaxSize: 1,
					}
				} else {
					cfg.Export = func(ctx context.Context, event *auditlogpb.ExportEventUnstructured) error {
						failures.Add(1)
						return trace.Errorf("export failed as test condition")
					}
				}

				exporter, err := NewDateExporter(cfg)
				require.NoError(t, err)
				defer exporter.Close()

				// each sleep covers at least one full backoff, and Wait lets the
				// resulting attempt run until every goroutine is blocked again.
				for range 5 {
					time.Sleep(cfg.MaxBackoff)
					synctest.Wait()
				}

				// with every goroutine blocked, the only streams still open are
				// those abandoned by failed attempts.
				require.GreaterOrEqual(t, failures.Load(), int64(5))
				require.Zero(t, clt.activeStreams.Load())
			})
		})
	}
}

// streamTrackingClient wraps fakeClient to track the number of export streams
// that have been started but not yet finished.
type streamTrackingClient struct {
	*fakeClient
	activeStreams atomic.Int64
}

func (c *streamTrackingClient) ExportUnstructuredEvents(ctx context.Context, req *auditlogpb.ExportUnstructuredEventsRequest) iter.Seq2[*auditlogpb.ExportEventUnstructured, error] {
	events := c.fakeClient.ExportUnstructuredEvents(ctx, req)
	return func(yield func(*auditlogpb.ExportEventUnstructured, error) bool) {
		c.activeStreams.Add(1)
		defer c.activeStreams.Add(-1)
		events(yield)
	}
}

func makeEventChunk(t *testing.T, ts time.Time, n int) ([]*auditlogpb.ExportEventUnstructured, []*auditlogpb.EventUnstructured) {
	var batchedEvents []*auditlogpb.EventUnstructured
	var chunk []*auditlogpb.ExportEventUnstructured
	for i := range n {
		baseEvent := apievents.UserLogin{
			Method:       events.LoginMethodSAML,
			Status:       apievents.Status{Success: true},
			UserMetadata: apievents.UserMetadata{User: "alice@example.com"},
			Metadata: apievents.Metadata{
				ID:   uuid.NewString(),
				Type: events.UserLoginEvent,
				Time: ts.Add(time.Duration(i)),
			},
		}

		event, err := apievents.ToUnstructured(&baseEvent)
		require.NoError(t, err)
		chunk = append(chunk, auditlogpb.ExportEventUnstructured_builder{
			Event:  event,
			Cursor: strconv.Itoa(i + 1),
		}.Build())
		batchedEvents = append(batchedEvents, event)
	}

	return chunk, batchedEvents
}

type fakeClient struct {
	mu          sync.Mutex
	data        map[string]map[string][]*auditlogpb.ExportEventUnstructured
	randomFlake bool
}

func newFakeClient() *fakeClient {
	return &fakeClient{
		data: make(map[string]map[string][]*auditlogpb.ExportEventUnstructured),
	}
}

func (c *fakeClient) setRandomFlake(flake bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.randomFlake = flake
}

func (c *fakeClient) addChunk(date string, chunk string, events []*auditlogpb.ExportEventUnstructured) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, ok := c.data[date]; !ok {
		c.data[date] = make(map[string][]*auditlogpb.ExportEventUnstructured)
	}
	c.data[date][chunk] = events
}

func (c *fakeClient) ExportUnstructuredEvents(ctx context.Context, req *auditlogpb.ExportUnstructuredEventsRequest) iter.Seq2[*auditlogpb.ExportEventUnstructured, error] {
	c.mu.Lock()
	defer c.mu.Unlock()
	chunks, ok := c.data[req.GetDate().AsTime().Format(time.DateOnly)]
	if !ok {
		return stream.Fail[*auditlogpb.ExportEventUnstructured](trace.NotFound("date not found"))
	}

	chunk, ok := chunks[req.GetChunk()]
	if !ok {
		return stream.Fail[*auditlogpb.ExportEventUnstructured](trace.NotFound("chunk not found"))
	}

	var cursor int
	if req.GetCursor() != "" {
		var err error
		cursor, err = strconv.Atoi(req.GetCursor())
		if err != nil {
			return stream.Fail[*auditlogpb.ExportEventUnstructured](trace.BadParameter("invalid cursor %q", req.GetCursor()))
		}
	}

	chunk = chunk[cursor:]

	// randomly truncate the chunk and append an error to simulate flake. we target a 33% failure rate
	// since event export is more frequent than chunk listing.
	var fail bool
	if c.randomFlake && rand.N(3) == 0 {
		chunk = chunk[:rand.N(len(chunk))]
		fail = true
	}

	return stream.MapErr(stream.Slice(chunk), func(err error) error {
		if fail {
			return trace.NotFound("export failed as random test condition")
		}
		return err
	})
}

func (c *fakeClient) GetEventExportChunks(ctx context.Context, req *auditlogpb.GetEventExportChunksRequest) iter.Seq2[*auditlogpb.EventExportChunk, error] {
	c.mu.Lock()
	defer c.mu.Unlock()
	chunks, ok := c.data[req.GetDate().AsTime().Format(time.DateOnly)]
	if !ok {
		return stream.Empty[*auditlogpb.EventExportChunk]()
	}

	var eec []*auditlogpb.EventExportChunk
	for name := range chunks {
		eec = append(eec, auditlogpb.EventExportChunk_builder{
			Chunk: name,
		}.Build())
	}

	// randomly truncate the chunk list and append an error to simulate flake. we target a 50% failure rate
	// since chunk listing is less frequent than event export.
	var fail bool
	if c.randomFlake && rand.N(2) == 0 {
		eec = eec[:rand.N(len(eec))]
		fail = true
	}

	return stream.MapErr(stream.Slice(eec), func(err error) error {
		if fail {
			return trace.NotFound("chunks failed as random test condition")
		}
		return err
	})
}
