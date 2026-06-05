package accessgraph

import (
	"context"
	"fmt"
	"log/slog"
	"slices"
	"strconv"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/gravitational/trace"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	auditlogpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/auditlog/v1"
	"github.com/gravitational/teleport/api/internalutils/stream"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/e/lib/accessgraph/grpctest"
	accessgraphv1 "github.com/gravitational/teleport/gen/proto/go/accessgraph/v1alpha"
	"github.com/gravitational/teleport/lib/events"
)

var (
	synctestStart = time.Date(2000, time.January, 1, 0, 0, 0, 0, time.UTC)
	testStartDate = synctestStart.AddDate(0, -1, 0)
)

type auditLogServerStream grpc.BidiStreamingServer[accessgraphv1.AuditLogStreamRequest, accessgraphv1.AuditLogStreamResponse]

func newTestStreams(ctx context.Context) (auditLogStream, auditLogServerStream) {
	return grpctest.NewStreams[accessgraphv1.AuditLogStreamRequest, accessgraphv1.AuditLogStreamResponse](ctx)
}

func newTestAuditLogExporter(stream auditLogStream, eventSource events.AuditLogSessionStreamer) *auditLogExporter {
	return &auditLogExporter{
		log:    slog.New(slog.DiscardHandler),
		client: eventSource,
		stream: stream,
	}
}

func newTAGServerMock(t *testing.T, configAndState []*accessgraphv1.AuditLogStreamResponse, receiveCount int, cancel context.CancelFunc) *tagServerMock {
	return &tagServerMock{
		configAndState: configAndState,
		receiveCount:   receiveCount,
		cancel:         cancel,
	}
}

type tagServerMock struct {
	accessgraphv1.UnimplementedAccessGraphServiceServer
	mu sync.Mutex

	// input
	configAndState []*accessgraphv1.AuditLogStreamResponse
	receiveCount   int
	cancel         context.CancelFunc

	// output
	receivedReqs []*accessgraphv1.AuditLogStreamRequest
}

func (s *tagServerMock) AuditLogStream(stream auditLogServerStream) error {
	// Receive and send config (typically negotiated)
	if _, err := stream.Recv(); err != nil {
		return err
	}
	if err := stream.Send(s.configAndState[0]); err != nil {
		return err
	}

	// Send resume state (typically persisted in DB)
	if err := stream.Send(s.configAndState[1]); err != nil {
		return err
	}

	// Receive receiveCount audit log event batches, then cancel (context).
	for range s.receiveCount {
		req, err := stream.Recv()
		if err != nil {
			return err
		}
		s.mu.Lock()
		s.receivedReqs = append(s.receivedReqs, req)
		s.mu.Unlock()
	}
	s.cancel()
	return nil
}

func (s *tagServerMock) receivedRequests() []*accessgraphv1.AuditLogStreamRequest {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.receivedReqs)
}

func newConfigAndState() []*accessgraphv1.AuditLogStreamResponse {
	config := accessgraphv1.AuditLogStreamResponse_builder{
		AuditLogConfig: accessgraphv1.AuditLogConfig_builder{
			StartDate: timestamppb.New(testStartDate),
		}.Build(),
	}.Build()
	noResumeState := &accessgraphv1.AuditLogStreamResponse{
		State: &accessgraphv1.AuditLogStreamResponse_NoResumeState{},
	}
	return []*accessgraphv1.AuditLogStreamResponse{config, noResumeState}
}

func requireProtoEq(t *testing.T, want, got proto.Message, msg string) {
	t.Helper()
	diff := cmp.Diff(want, got, protocmp.Transform())
	if len(diff) > 0 {
		t.Errorf("%sProtos are not equal, diff:\n%s", msg, diff)
	}
}

func requireRequestsEqual(t *testing.T, want, got []*accessgraphv1.AuditLogStreamRequest) {
	t.Helper()
	require.Len(t, got, len(want))
	for i := range want {
		requireProtoEq(t, want[i], got[i], fmt.Sprintf("Index %d\n", i))
	}
}

// --- AuditLog SEARCH export testing

func Test_AuditLogExport_Search_Backfill(t *testing.T) {
	batches := []testEventBatch{
		newTestEventBatch("a", &testEvent{id: 0}, &testEvent{id: 1}), // startkey: a
		newTestEventBatch("b", &testEvent{id: 10}, &testEvent{id: 11}, &testEvent{id: 12}),
	}
	want := []*accessgraphv1.AuditLogStreamRequest{
		searchEventRequest(batches[0].events, "a", ""), // startkey: a, lastEventID: ""
		searchEventRequest(batches[1].events, "b", ""),
	}

	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		clientStream, serverStream := newTestStreams(ctx)
		server := newTAGServerMock(t, newConfigAndState(), 2, cancel) // cancel context after 2 requests
		go server.AuditLogStream(serverStream)
		mock := &searchEventsMock{batches: batches}
		exporter := newTestAuditLogExporter(clientStream, mock)
		go exporter.start(ctx, AuditLogConfig{Enabled: true})

		synctest.Wait()
		got := server.receivedRequests()
		requireRequestsEqual(t, want, got)

		<-ctx.Done() // wait for server mock to terminate, after 2 requests received
	})
}

func Test_AuditLogExport_ConnectedMetric(t *testing.T) {
	setAccessGraphConnected(accessGraphMetricStreamAuditLog, false)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	clientStream, serverStream := newTestStreams(ctx)
	server := newTAGServerMock(t, newConfigAndState(), 1, cancel)
	serverErrC := make(chan error, 1)
	go func() {
		serverErrC <- server.AuditLogStream(serverStream)
	}()

	mock := &searchEventsMock{
		batches: []testEventBatch{newTestEventBatch("")},
	}
	exporter := newTestAuditLogExporter(clientStream, mock)
	exporterErrC := make(chan error, 1)
	go func() {
		exporterErrC <- exporter.start(ctx, AuditLogConfig{Enabled: true})
	}()

	require.Eventually(t, func() bool {
		return testutil.ToFloat64(accessGraphConnected.WithLabelValues(accessGraphMetricStreamAuditLog)) == 1
	}, 10*time.Second, 100*time.Millisecond, "expected audit log connected metric to be set")

	cancel()
	require.Eventually(t, func() bool {
		return testutil.ToFloat64(accessGraphConnected.WithLabelValues(accessGraphMetricStreamAuditLog)) == 0
	}, 10*time.Second, 100*time.Millisecond, "expected audit log connected metric to be reset")

	select {
	case <-serverErrC:
	case <-time.After(10 * time.Second):
		t.Fatal("timed out waiting for audit log server stream to stop")
	}
	select {
	case <-exporterErrC:
	case <-time.After(10 * time.Second):
		t.Fatal("timed out waiting for audit log exporter to stop")
	}
}

func Test_AuditLogExport_Search_OneWait(t *testing.T) {
	batches := []testEventBatch{
		newTestEventBatch("a", &testEvent{id: 0}), // startkey: a
		newTestEventBatch("", &testEvent{id: 10}),
		newTestEventBatch("b", &testEvent{id: 10}, &testEvent{id: 11}),
		newTestEventBatch("c", &testEvent{id: 20}),
	}
	want := []*accessgraphv1.AuditLogStreamRequest{
		searchEventRequest(batches[0].events, "a", ""), // startkey: a, lastEventID: ""
		searchEventRequest(batches[1].events, "a", "10"),
		searchEventRequest(batches[2].events[1:], "b", ""),
		searchEventRequest(batches[3].events, "c", ""),
	}

	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		clientStream, serverStream := newTestStreams(ctx)
		server := newTAGServerMock(t, newConfigAndState(), len(batches), cancel) // cancel context after 4 requests
		go server.AuditLogStream(serverStream)
		mock := &searchEventsMock{batches: batches}
		exporter := newTestAuditLogExporter(clientStream, mock)
		go exporter.start(ctx, AuditLogConfig{Enabled: true})

		synctest.Wait()                                              // wait until all goroutines are blocked
		requireRequestsEqual(t, want[:2], server.receivedRequests()) // require 2 requests through back-fill (until empty startKey)

		time.Sleep(time.Minute)                                      // advance clock by 1 minute to trigger new search in real time mode
		synctest.Wait()                                              // block until all timers within 1 minute have been processed
		requireRequestsEqual(t, want[:4], server.receivedRequests()) // require 2 new requests, 4 in total

		<-ctx.Done() // wait for server mock to terminate
	})
}

func Test_AuditLogExport_Search_TwoWaitNoneEmpty(t *testing.T) {
	batches := []testEventBatch{
		newTestEventBatch("a", &testEvent{id: 0}), // startkey: a
		newTestEventBatch("", &testEvent{id: 10}),
		newTestEventBatch("", &testEvent{id: 10}, &testEvent{id: 11}),
		newTestEventBatch("b", &testEvent{id: 10}, &testEvent{id: 11}, &testEvent{id: 12}),
		newTestEventBatch("c", &testEvent{id: 20}),
	}
	want := []*accessgraphv1.AuditLogStreamRequest{
		searchEventRequest(batches[0].events, "a", ""), // startkey: a, lastEventID: ""
		searchEventRequest(batches[1].events, "a", "10"),
		searchEventRequest(batches[2].events[1:], "a", "11"),
		searchEventRequest(batches[3].events[2:], "b", ""),
		searchEventRequest(batches[4].events, "c", ""),
	}

	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		clientStream, serverStream := newTestStreams(ctx)
		server := newTAGServerMock(t, newConfigAndState(), len(batches), cancel) // cancel context after 5 requests
		go server.AuditLogStream(serverStream)
		mock := &searchEventsMock{batches: batches}
		exporter := newTestAuditLogExporter(clientStream, mock)
		go exporter.start(ctx, AuditLogConfig{Enabled: true})

		synctest.Wait()
		requireRequestsEqual(t, want[:2], server.receivedRequests()) // require 2 requests through back-fill (until empty startKey)

		time.Sleep(time.Minute) // advance clock by 1 minute to trigger new search in real time mode
		synctest.Wait()
		requireRequestsEqual(t, want[:3], server.receivedRequests()) // require 1 new request, 3 in total

		time.Sleep(time.Minute) // advance clock by another minute to trigger next real-time search and receive last two requests
		synctest.Wait()
		requireRequestsEqual(t, want[:5], server.receivedRequests()) // require 2 new requests, 5 in total

		<-ctx.Done() // wait for server mock to terminate
	})
}

func Test_AuditLogExport_Search_TwoWaitOneEmpty(t *testing.T) {
	batches := []testEventBatch{
		newTestEventBatch("a", &testEvent{id: 0}), // startkey: a
		newTestEventBatch("", &testEvent{id: 10}),
		newTestEventBatch("", &testEvent{id: 10}), // no new events!
		newTestEventBatch("b", &testEvent{id: 10}, &testEvent{id: 11}),
		newTestEventBatch("c", &testEvent{id: 20}),
	}
	want := []*accessgraphv1.AuditLogStreamRequest{
		searchEventRequest(batches[0].events, "a", ""), // startkey: a, lastEventID: ""
		searchEventRequest(batches[1].events, "a", "10"),
		// skip batch[2] as it has no events
		searchEventRequest(batches[3].events[1:], "b", ""),
		searchEventRequest(batches[4].events, "c", ""),
	}
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		clientStream, serverStream := newTestStreams(ctx)
		server := newTAGServerMock(t, newConfigAndState(), len(batches)-1, cancel) // cancel context after 4 requests
		go server.AuditLogStream(serverStream)
		mock := &searchEventsMock{batches: batches}
		exporter := newTestAuditLogExporter(clientStream, mock)
		go exporter.start(ctx, AuditLogConfig{Enabled: true})

		synctest.Wait()
		requireRequestsEqual(t, want[:2], server.receivedRequests()) // require 2 requests through back-fill (until empty startKey)

		time.Sleep(time.Minute) // advance clock by 1 minute to trigger new search in real time mode
		synctest.Wait()
		requireRequestsEqual(t, want[:2], server.receivedRequests()) // require no new request, still 2 in total

		time.Sleep(time.Minute) // advance clock by another minute to trigger next real-time search and receive last two requests
		synctest.Wait()
		requireRequestsEqual(t, want[:4], server.receivedRequests()) // require 2 new requests, 4 in total

		<-ctx.Done() // wait for server mock to terminate
	})
}

func Test_AuditLogExport_Search_WithResume(t *testing.T) {
	batches := []testEventBatch{
		newTestEventBatch("a", &testEvent{id: 0}, &testEvent{id: 1}), // startkey of next batch: a
		newTestEventBatch("b", &testEvent{id: 10}),
		newTestEventBatch("", &testEvent{id: 20}),
	}
	want := []*accessgraphv1.AuditLogStreamRequest{
		searchEventRequest(batches[0].events[1:], "a", ""), // first event is skipped because of resume state
		searchEventRequest(batches[1].events, "b", ""),
		searchEventRequest(batches[2].events, "b", "20"),
	}
	wantSearch := []events.SearchEventsRequest{
		{From: testStartDate, To: synctestStart, StartKey: "X"},
		{From: testStartDate, To: synctestStart, StartKey: "a"},
		{From: testStartDate, To: synctestStart, StartKey: "b"},
	}

	configAndState := newConfigAndState()
	searchState := &accessgraphv1.AuditLogStreamResponse_SearchResumeState{SearchResumeState: searchResumeState("X", "0")}
	configAndState[1] = &accessgraphv1.AuditLogStreamResponse{State: searchState}
	synctest.Test(t, func(t *testing.T) { // use synctest for time.Now() fixture
		ctx, cancel := context.WithCancel(t.Context())
		clientStream, serverStream := newTestStreams(ctx)
		server := newTAGServerMock(t, configAndState, len(batches), cancel)
		go server.AuditLogStream(serverStream)
		mock := &searchEventsMock{batches: batches}
		exporter := newTestAuditLogExporter(clientStream, mock)
		go exporter.start(ctx, AuditLogConfig{Enabled: true})

		synctest.Wait()
		requireRequestsEqual(t, want, server.receivedRequests())
		require.Equal(t, wantSearch, mock.receivedRequests())

		<-ctx.Done() // wait until context canceled by mock server
	})
}

// --- AuditLog SEARCH export testing utilities

type searchEventsMock struct {
	events.AuditLogSessionStreamer
	batches []testEventBatch
	index   int

	mu           sync.Mutex
	receivedReqs []events.SearchEventsRequest
}

type testEventBatch struct {
	events  []apievents.AuditEvent
	nextKey string
}

func (t *searchEventsMock) SearchEvents(ctx context.Context, req events.SearchEventsRequest) ([]apievents.AuditEvent, string, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.receivedReqs = append(t.receivedReqs, req)

	batch := t.batches[t.index]
	t.index = (t.index + 1) % len(t.batches)
	return batch.events, batch.nextKey, nil
}

func (t *searchEventsMock) GetEventExportChunks(ctx context.Context, req *auditlogpb.GetEventExportChunksRequest) stream.Stream[*auditlogpb.EventExportChunk] {
	// called to check if exporter is bulk or search, see auditLogExporter.isBulkExporter method.
	return stream.Fail[*auditlogpb.EventExportChunk](trace.NotImplemented("searchEventsMock does not implement GetEventExportChunks"))
}

func (t *searchEventsMock) Close() error {
	return nil
}

func (t *searchEventsMock) receivedRequests() []events.SearchEventsRequest {
	t.mu.Lock()
	defer t.mu.Unlock()
	return slices.Clone(t.receivedReqs)
}

func newTestEventBatch(nextKey string, events ...apievents.AuditEvent) testEventBatch {
	return testEventBatch{
		nextKey: nextKey,
		events:  events,
	}
}

func searchEventRequest(events []apievents.AuditEvent, startKey, lastID string) *accessgraphv1.AuditLogStreamRequest {
	return accessgraphv1.AuditLogStreamRequest_builder{
		Events: &accessgraphv1.AuditLogEvents{
			Events: toUnstructured(events),
			ResumeState: &accessgraphv1.AuditLogEvents_SearchResumeState{
				SearchResumeState: searchResumeState(startKey, lastID),
			},
		},
	}.Build()
}

func toUnstructured(events []apievents.AuditEvent) []*auditlogpb.EventUnstructured {
	result := make([]*auditlogpb.EventUnstructured, len(events))
	for i, event := range events {
		result[i] = auditlogpb.EventUnstructured_builder{
			Id:   event.GetID(),
			Type: event.GetType(),
			Time: timestamppb.New(event.GetTime()),
			Unstructured: &structpb.Struct{
				Fields: map[string]*structpb.Value{
					"AuditEvent": structpb.NewNullValue(),
				},
			},
		}.Build()
	}
	return result
}

func searchResumeState(startKey, lastID string) *accessgraphv1.SearchResumeState {
	return accessgraphv1.SearchResumeState_builder{
		StartKey:    startKey,
		LastEventId: lastID,
	}.Build()
}

type testEvent struct {
	apievents.AuditEvent
	id int
}

func (t *testEvent) GetID() string {
	return fmt.Sprintf("%d", t.id)
}

func (t *testEvent) GetType() string {
	return "testEvent"
}

func (t *testEvent) GetIndex() int64 {
	return 0
}

func (t *testEvent) GetTime() time.Time {
	return testStartDate
}

// --- AuditLog BULK export testing

func Test_AuditLogExport_Bulk_Backfill(t *testing.T) {
	day1 := testStartDate
	day2 := day1.AddDate(0, 0, 1)

	eventsMock := newBulkEventsMock()
	events1 := eventsMock.addChunk(day1, "chunk1", 3)
	events2 := eventsMock.addChunk(day1, "chunk2", 5)
	events3 := eventsMock.addChunk(day1, "chunk3", 2)
	events4 := eventsMock.addChunk(day1, "chunk4", 1)
	events5 := eventsMock.addChunk(day2, "chunk5", 7)
	events6 := eventsMock.addChunk(day2, "chunk6", 0)

	want := []*accessgraphv1.AuditLogStreamRequest{
		bulkEventRequest(events1, day1, "chunk1", "", true), // resume state: chunk1, cursor="", completed=true
		bulkEventRequest(events2, day1, "chunk2", "", true),
		bulkEventRequest(events3, day1, "chunk3", "", true),
		bulkEventRequest(events4, day1, "chunk4", "", true),
		bulkEventRequest(events5, day2, "chunk5", "", true),
		bulkEventRequest(events6, day2, "chunk6", "", true),
	}

	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		clientStream, serverStream := newTestStreams(ctx)
		server := newTAGServerMock(t, newConfigAndState(), len(want), cancel) // cancel context after wanted number of requests
		go server.AuditLogStream(serverStream)
		exporter := newTestAuditLogExporter(clientStream, eventsMock)
		go exporter.start(ctx, AuditLogConfig{Enabled: true})

		synctest.Wait()
		requireBulkRequestsSame(t, want[:4], server.receivedRequests()) // require all 4 chunks from day 1 as requests

		time.Sleep(15 * time.Second) // Advance time by 15 seconds; day 2 processing should complete (there is jitter in polling intervals in bulk exporter so we cannot be precise)
		synctest.Wait()
		requireBulkRequestsSame(t, want, server.receivedRequests()) // require all 4 day 1 chunks as requests

		<-ctx.Done() // wait until context canceled by mock server
	})
}

func Test_AuditLogExport_Bulk_BackfillManyDates(t *testing.T) {
	start := testStartDate

	eventsMock := newBulkEventsMock()
	events0 := eventsMock.addChunk(start.AddDate(0, 0, 0), "chunk0", 1)
	events1 := eventsMock.addChunk(start.AddDate(0, 0, 1), "chunk1", 1)
	events2 := eventsMock.addChunk(start.AddDate(0, 0, 2), "chunk2", 1)
	events3 := eventsMock.addChunk(start.AddDate(0, 0, 3), "chunk3", 2)
	events4 := eventsMock.addChunk(start.AddDate(0, 0, 4), "chunk4", 1)
	events5 := eventsMock.addChunk(start.AddDate(0, 0, 5), "chunk5", 7)
	events6 := eventsMock.addChunk(start.AddDate(0, 0, 6), "chunk6", 0)

	want := []*accessgraphv1.AuditLogStreamRequest{
		bulkEventRequest(events0, start.AddDate(0, 0, 0), "chunk0", "", true), // resume state: chunk0, cursor="", completed=true
		bulkEventRequest(events1, start.AddDate(0, 0, 1), "chunk1", "", true),
		bulkEventRequest(events2, start.AddDate(0, 0, 2), "chunk2", "", true),
		bulkEventRequest(events3, start.AddDate(0, 0, 3), "chunk3", "", true),
		bulkEventRequest(events4, start.AddDate(0, 0, 4), "chunk4", "", true),
		bulkEventRequest(events5, start.AddDate(0, 0, 5), "chunk5", "", true),
		bulkEventRequest(events6, start.AddDate(0, 0, 6), "chunk6", "", true),
	}

	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		clientStream, serverStream := newTestStreams(ctx)
		server := newTAGServerMock(t, newConfigAndState(), len(want), cancel) // cancel context after wanted number of requests
		go server.AuditLogStream(serverStream)
		exporter := newTestAuditLogExporter(clientStream, eventsMock)
		go exporter.start(ctx, AuditLogConfig{Enabled: true})

		synctest.Wait()
		requireRequestsEqual(t, want[:1], server.receivedRequests()) // require the only chunk from day 1 as request before time is advance

		time.Sleep(45 * time.Second) // Advance time by 45 seconds; each day waits 15s before submitting the batch, since we run event exporter with concurrency = 3, we must wait for 3 batches to complete leading to 45s for 7 days
		synctest.Wait()
		requireBulkRequestsSame(t, want, server.receivedRequests()) // require all 4 day 1 chunks as requests

		<-ctx.Done() // wait until context canceled by mock server
	})
}

func Test_AuditLogExport_Bulk_SyncActiveDates(t *testing.T) {
	eventsMock := newBulkEventsMock()
	events1 := eventsMock.addChunk(testStartDate, "chunk1", 1)

	want := []*accessgraphv1.AuditLogStreamRequest{
		bulkEventRequest(events1, testStartDate, "chunk1", "", true),
		bulkSync(synctestStart),
	}

	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		clientStream, serverStream := newTestStreams(ctx)
		// cancel context after 2 requests; second request must be a bulk state sync
		server := newTAGServerMock(t, newConfigAndState(), 2, cancel)
		go server.AuditLogStream(serverStream)
		exporter := newTestAuditLogExporter(clientStream, eventsMock)
		go exporter.start(ctx, AuditLogConfig{Enabled: true})

		synctest.Wait()
		requireRequestsEqual(t, want[:1], server.receivedRequests()) // require the only chunk from day 1 as request before time is advance

		time.Sleep(time.Minute) // Advance time by 1 minute to trigger the first sync active dates request
		synctest.Wait()
		requireRequestsEqual(t, want, server.receivedRequests())

		<-ctx.Done() // wait until context canceled by mock server
	})
}

func Test_AuditLogExport_Bulk_SlowChunks(t *testing.T) {
	// Stream 3 events with 7s delay after each
	eventsMock := newDelayedBulkEventsMock(7 * time.Second)
	events1 := eventsMock.addChunk(testStartDate, "chunk1", 3)
	want := []*accessgraphv1.AuditLogStreamRequest{
		bulkEventRequest(events1[0:1], testStartDate, "chunk1", "0", false), // cursor=0 completed=false
		bulkEventRequest(events1[1:2], testStartDate, "chunk1", "1", false), // cursor=1 completed=false
		bulkEventRequest(events1[2:], testStartDate, "chunk1", "", true),    // cursor=2 completed=true
	}

	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		clientStream, serverStream := newTestStreams(ctx)
		// cancel context after 2 requests; second request must be a bulk state sync
		server := newTAGServerMock(t, newConfigAndState(), 3, cancel)
		go server.AuditLogStream(serverStream)
		exporter := newTestAuditLogExporter(clientStream, eventsMock)
		go exporter.start(ctx, AuditLogConfig{Enabled: true})
		synctest.Wait()
		got := server.receivedRequests()
		require.Empty(t, got)

		time.Sleep(5 * time.Second)
		synctest.Wait()
		got = server.receivedRequests()
		requireRequestsEqual(t, want[:1], got[:1])

		time.Sleep(5 * time.Second)
		synctest.Wait()
		got = server.receivedRequests()
		requireRequestsEqual(t, want[:2], got[:2])

		<-ctx.Done() // wait until context canceled by mock server
	})
}

func Test_AuditLogExport_Bulk_ManySlowChunks(t *testing.T) {
	// Stream 3 events with 7s delay after each
	eventsMock := newDelayedBulkEventsMock(7 * time.Second)
	for i := range 100 {
		eventsMock.addChunk(testStartDate, fmt.Sprintf("chunk%d", i), 10)
	}

	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		clientStream, serverStream := newTestStreams(ctx)
		// cancel context after 2 requests; second request must be a bulk state sync
		server := newTAGServerMock(t, newConfigAndState(), 1000, cancel)
		go server.AuditLogStream(serverStream)
		exporter := newTestAuditLogExporter(clientStream, eventsMock)
		go exporter.start(ctx, AuditLogConfig{Enabled: true})

		<-ctx.Done() // wait until context canceled by mock server

		require.Len(t, server.receivedRequests(), 1000) // extra requests for sync active dates
	})
}

// --- AuditLog BULK export testing utilities

type bulkEventsMock struct {
	events.AuditLogSessionStreamer
	mu sync.Mutex

	delay time.Duration
	data  map[string]map[string][]*auditlogpb.ExportEventUnstructured
}

func newBulkEventsMock() *bulkEventsMock {
	return &bulkEventsMock{
		data: map[string]map[string][]*auditlogpb.ExportEventUnstructured{},
	}
}

func newDelayedBulkEventsMock(delay time.Duration) *bulkEventsMock {
	return &bulkEventsMock{
		delay: delay,
		data:  map[string]map[string][]*auditlogpb.ExportEventUnstructured{},
	}
}

func (b *bulkEventsMock) addChunk(chunkDate time.Time, chunkID string, eventCnt int) []*auditlogpb.EventUnstructured {
	date := chunkDate.Format(time.DateOnly)
	events := make([]*auditlogpb.ExportEventUnstructured, eventCnt)
	result := make([]*auditlogpb.EventUnstructured, eventCnt)
	for i := range eventCnt {
		cursor := strconv.Itoa(i)
		event := auditlogpb.EventUnstructured_builder{Id: fmt.Sprintf("%s-%s-%s", date, chunkID, cursor)}.Build()
		result[i] = event
		events[i] = auditlogpb.ExportEventUnstructured_builder{Event: event, Cursor: cursor}.Build()
	}

	b.mu.Lock()
	defer b.mu.Unlock()
	if _, ok := b.data[date]; !ok {
		b.data[date] = make(map[string][]*auditlogpb.ExportEventUnstructured)
	}
	b.data[date][chunkID] = events
	return result
}

func (b *bulkEventsMock) GetEventExportChunks(ctx context.Context, req *auditlogpb.GetEventExportChunksRequest) stream.Stream[*auditlogpb.EventExportChunk] {
	b.mu.Lock()
	defer b.mu.Unlock()
	chunks, ok := b.data[req.GetDate().AsTime().Format(time.DateOnly)]
	if !ok {
		return stream.Empty[*auditlogpb.EventExportChunk]()
	}
	var chunkIDs []*auditlogpb.EventExportChunk
	for chunkID := range chunks {
		chunkIDPB := auditlogpb.EventExportChunk_builder{
			Chunk: chunkID,
		}.Build()
		chunkIDs = append(chunkIDs, chunkIDPB)
	}
	return stream.Slice(chunkIDs)
}

func (b *bulkEventsMock) ExportUnstructuredEvents(ctx context.Context, req *auditlogpb.ExportUnstructuredEventsRequest) stream.Stream[*auditlogpb.ExportEventUnstructured] {
	b.mu.Lock()
	defer b.mu.Unlock()
	date := req.GetDate().AsTime().Format(time.DateOnly)
	chunks, ok := b.data[date]
	if !ok {
		return stream.Fail[*auditlogpb.ExportEventUnstructured](trace.NotFound("date not found: %q", date))
	}

	chunk, ok := chunks[req.GetChunk()]
	if !ok {
		return stream.Fail[*auditlogpb.ExportEventUnstructured](trace.NotFound("chunk not found: %q", req.GetChunk()))
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
	if b.delay <= 0 {
		return stream.Slice(chunk)
	}
	return slowStream(ctx, chunk, b.delay)
}

func (b *bulkEventsMock) Close() error {
	return nil
}

type slowSlice[T any] struct {
	ctx   context.Context
	items []T
	idx   int

	delay time.Duration
}

func (s *slowSlice[T]) Next() bool {
	s.idx++
	if s.idx > 0 && s.idx < len(s.items) { // don't sleep for fist item or when done
		select {
		case <-time.After(s.delay):
		case <-s.ctx.Done():
			return false
		}
	}
	return s.idx < len(s.items)
}

func (s *slowSlice[T]) Item() T {
	return s.items[s.idx]
}

func (s *slowSlice[T]) Done() error {
	return nil
}

func slowStream[T any](ctx context.Context, items []T, delay time.Duration) stream.Stream[T] {
	return &slowSlice[T]{
		ctx:   ctx,
		delay: delay,
		items: items,
		idx:   -1,
	}
}

func bulkSync(dates ...time.Time) *accessgraphv1.AuditLogStreamRequest {
	pbDates := make([]*timestamppb.Timestamp, len(dates))
	for i, date := range dates {
		pbDates[i] = timestamppb.New(date)
	}
	return accessgraphv1.AuditLogStreamRequest_builder{
		BulkSync: accessgraphv1.BulkResumeStateSync_builder{
			ActiveDates: pbDates,
		}.Build(),
	}.Build()
}

func bulkEventRequest(events []*auditlogpb.EventUnstructured, date time.Time, chunkID, cursor string, completed bool) *accessgraphv1.AuditLogStreamRequest {
	return accessgraphv1.AuditLogStreamRequest_builder{
		Events: accessgraphv1.AuditLogEvents_builder{
			Events: events,
			BulkResumeStateUpdate: accessgraphv1.BulkResumeStateUpdate_builder{
				Date:      timestamppb.New(date), // TODO
				Chunk:     chunkID,
				Cursor:    cursor,
				Completed: completed,
			}.Build(),
		}.Build(),
	}.Build()
}

func requireBulkRequestsSame(t *testing.T, want, got []*accessgraphv1.AuditLogStreamRequest) {
	t.Helper()
	require.Len(t, got, len(want))
	wantByID := make(map[string]*accessgraphv1.AuditLogStreamRequest)
	for _, req := range want {
		reqID := bulkEventRequestID(t, req)
		wantByID[reqID] = req
	}

	for _, req := range got {
		reqID := bulkEventRequestID(t, req)
		wantReq, ok := wantByID[reqID]
		if !ok {
			t.Errorf("Unexpected request: %v", req)
		}
		requireProtoEq(t, wantReq, req, "")
	}
}

func bulkEventRequestID(t *testing.T, req *accessgraphv1.AuditLogStreamRequest) string {
	t.Helper()
	events := req.GetEvents()
	require.NotNil(t, events, "No events action request\n  action:%v\n  type:%T", req.GetAction(), req.GetAction())
	state := events.GetBulkResumeStateUpdate()
	require.NotNil(t, state, "No bulk resume state\n  state:%v\n  type:%T", events.GetResumeState(), events.GetResumeState())
	return fmt.Sprintf("%s-%s-%s-%t", state.GetDate().AsTime().Format(time.RFC3339), state.GetChunk(), state.GetCursor(), state.GetCompleted())
}
