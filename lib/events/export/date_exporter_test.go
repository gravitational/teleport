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
	"math/rand/v2"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	auditlogpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/auditlog/v1"
	"github.com/gravitational/teleport/api/internalutils/stream"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/events"
)

// TestDateExporterBasics tests the basic functionality of the date exporter, with and
// without random flake.
func TestDateExporterBasics(t *testing.T) {
	t.Parallel()
	for _, randomFlake := range []bool{false, true} {
		t.Run(fmt.Sprintf("randomFlake=%v", randomFlake), func(t *testing.T) {
			t.Parallel()
			testDateExporterBasics(t, randomFlake)
		})
	}
}

func testDateExporterBasics(t *testing.T, randomFlake bool) {
	clt := newFakeClient()
	clt.setRandomFlake(randomFlake)

	now := time.Now()

	var exportedMu sync.Mutex
	var exported []*auditlogpb.ExportEventUnstructured

	exportFn := func(ctx context.Context, event *auditlogpb.ExportEventUnstructured) error {
		exportedMu.Lock()
		defer exportedMu.Unlock()
		exported = append(exported, event)
		return nil
	}

	getExported := func() []*auditlogpb.ExportEventUnstructured {
		exportedMu.Lock()
		defer exportedMu.Unlock()
		return append([]*auditlogpb.ExportEventUnstructured(nil), exported...)
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
		for i := 0; i < 2; i++ {
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
		Concurrency:  3,
		MaxBackoff:   time.Millisecond * 600,
		PollInterval: time.Millisecond * 200,
	})
	require.NoError(t, err)
	defer exporter.Close()

	// empty event set means the exporter should become idle almost
	// immediately.
	waitIdle(t)
	require.Empty(t, getExported())

	var allEvents []*auditlogpb.ExportEventUnstructured
	var allChunks []string
	// quickly add a bunch of chunks
	for i := 0; i < 30; i++ {
		chunk := makeEventChunk(t, now, 10)
		allEvents = append(allEvents, chunk...)
		chunkID := uuid.NewString()
		allChunks = append(allChunks, chunkID)
		clt.addChunk(now.Format(time.DateOnly), chunkID, chunk)
	}

	waitIdle(t)

	require.ElementsMatch(t, allChunks, exporter.GetState().Completed)
	require.ElementsMatch(t, allEvents, getExported())

	// process a second round of chunks to cover the case of new chunks being added
	// after non-trivial idleness.

	// note that we do a lot more events here just to make absolutely certain
	// that we're hitting a decent amout of random flake.
	for i := 0; i < 30; i++ {
		chunk := makeEventChunk(t, now, 10)
		allEvents = append(allEvents, chunk...)
		chunkID := uuid.NewString()
		allChunks = append(allChunks, chunkID)
		clt.addChunk(now.Format(time.DateOnly), chunkID, chunk)
	}

	waitIdle(t)

	require.ElementsMatch(t, allChunks, exporter.GetState().Completed)
	require.ElementsMatch(t, allEvents, getExported())

	// close the exporter
	exporter.Close()
	timeout := time.After(time.Second * 30)
	select {
	case <-exporter.Done():
	case <-timeout:
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

	waitIdle(t)

	// no additional events should have been exported
	require.ElementsMatch(t, allChunks, exporter.GetState().Completed)
	require.ElementsMatch(t, allEvents, getExported())

	// new chunks should be consumed correctly
	for i := 0; i < 30; i++ {
		chunk := makeEventChunk(t, now, 10)
		allEvents = append(allEvents, chunk...)
		chunkID := uuid.NewString()
		allChunks = append(allChunks, chunkID)
		clt.addChunk(now.Format(time.DateOnly), chunkID, chunk)
	}

	waitIdle(t)

	require.ElementsMatch(t, allChunks, exporter.GetState().Completed)
	require.ElementsMatch(t, allEvents, getExported())
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

	now := time.Now()

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
		for i := 0; i < 2; i++ {
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
		Concurrency:  3, /* low concurrency to ensure that we have some in progress chunks */
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
	for i := 0; i < 10; i++ {
		chunk := makeEventChunk(t, now, 10)
		allEvents = append(allEvents, chunk...)
		chunkID := uuid.NewString()
		clt.addChunk(now.Format(time.DateOnly), chunkID, chunk)
	}

	// consume a large subset of events s.t. we have some completed
	// chunks, some in progress, and some not yet started (note that
	// to guarantee some in progress chunks, the number consumed must not
	// divide evenly by the chunk size).
	timeout := time.After(time.Second * 30)
	for i := 0; i < 47; i++ {
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

	fmt.Printf("cursors=%+v\n", state.Cursors)

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
	for i := 0; i < 53; i++ {
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

func makeEventChunk(t *testing.T, ts time.Time, n int) []*auditlogpb.ExportEventUnstructured {
	var chunk []*auditlogpb.ExportEventUnstructured
	for i := 0; i < n; i++ {
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
		chunk = append(chunk, &auditlogpb.ExportEventUnstructured{
			Event:  event,
			Cursor: strconv.Itoa(i + 1),
		})
	}

	return chunk
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

func (c *fakeClient) ExportUnstructuredEvents(ctx context.Context, req *auditlogpb.ExportUnstructuredEventsRequest) stream.Stream[*auditlogpb.ExportEventUnstructured] {
	c.mu.Lock()
	defer c.mu.Unlock()
	chunks, ok := c.data[req.Date.AsTime().Format(time.DateOnly)]
	if !ok {
		return stream.Fail[*auditlogpb.ExportEventUnstructured](trace.NotFound("date not found"))
	}

	chunk, ok := chunks[req.Chunk]
	if !ok {
		return stream.Fail[*auditlogpb.ExportEventUnstructured](trace.NotFound("chunk not found"))
	}

	var cursor int
	if req.Cursor != "" {
		var err error
		cursor, err = strconv.Atoi(req.Cursor)
		if err != nil {
			return stream.Fail[*auditlogpb.ExportEventUnstructured](trace.BadParameter("invalid cursor %q", req.Cursor))
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

func (c *fakeClient) GetEventExportChunks(ctx context.Context, req *auditlogpb.GetEventExportChunksRequest) stream.Stream[*auditlogpb.EventExportChunk] {
	c.mu.Lock()
	defer c.mu.Unlock()
	chunks, ok := c.data[req.Date.AsTime().Format(time.DateOnly)]
	if !ok {
		return stream.Empty[*auditlogpb.EventExportChunk]()
	}

	var eec []*auditlogpb.EventExportChunk
	for name := range chunks {
		eec = append(eec, &auditlogpb.EventExportChunk{
			Chunk: name,
		})
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
