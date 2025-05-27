package accessgraph

import (
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"net"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/protobuf/types/known/timestamppb"

	auditlogpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/auditlog/v1"
	"github.com/gravitational/teleport/api/internalutils/stream"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	accessgraphv1 "github.com/gravitational/teleport/gen/proto/go/accessgraph/v1alpha"
	"github.com/gravitational/teleport/lib/auth"
	authority "github.com/gravitational/teleport/lib/auth/testauthority"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/fixtures"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local"
)

type accessGraphServerStub struct {
	accessgraphv1.UnimplementedAccessGraphServiceServer
	mu sync.Mutex

	// input
	configAndState []*accessgraphv1.AuditLogStreamResponse
	receiveCount   int
	cancel         context.CancelFunc

	// output
	receivedAuditLogEvents []*accessgraphv1.AuditLogStreamRequest
}

type testEventSourceSearch struct {
	events.AuditLogSessionStreamer
	eventBatches []testEventBatch
	index        int

	mu               sync.Mutex
	receivedRequests []events.SearchEventsRequest
}

type testEventBatch struct {
	events  []apievents.AuditEvent
	nextKey string
}

func Test_AuditLogExport_Search_Backfill(t *testing.T) {
	// GIVEN the following events sourced from search exporter in order
	searchSource := &testEventSourceSearch{
		eventBatches: []testEventBatch{{
			nextKey: "a",
			events: []apievents.AuditEvent{
				&testEvent{id: 0, message: "batch:0 msg:0 "},
				&testEvent{id: 1, message: "batch:0 msg:1 "},
			},
		}, {
			nextKey: "b",
			events: []apievents.AuditEvent{
				&testEvent{id: 10, message: "batch:1 msg:0 "},
				&testEvent{id: 11, message: "batch:1 msg:1 "},
				&testEvent{id: 12, message: "batch:1 msg:2 "},
			},
		}},
	}
	// WHEN we start an auditlog exporting client and an access-graph server stub
	batchCount := len(searchSource.eventBatches)
	lis, err := net.Listen("tcp", "localhost:0")
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	server := newAccessGraphServerStub(t, lis, newConfigAndState(), batchCount, cancel)
	clock := clockwork.NewFakeClock()
	exporter, err := newTestAuditLogExporter(t, ctx, lis, clock, searchSource)
	require.NoError(t, err)
	exporter.start(ctx, AuditLogConfig{Enabled: true})

	// THEN we expect to track following requests in the stub server
	gotEvents := server.receivedEvents()
	require.Len(t, gotEvents, batchCount)
	for i, req := range gotEvents {
		batch := searchSource.eventBatches[i]
		gotLen := len(req.GetEvents().GetEvents())
		wantLen := len(batch.events)
		require.Equal(t, wantLen, gotLen)

		resumeState := req.GetEvents().GetSearchResumeState()
		require.Empty(t, resumeState.GetLastEventId())
		require.Equal(t, batch.nextKey, resumeState.GetStartKey())
	}
}

type wantRequest struct {
	wantLen     int
	wantKey     string
	wantFirstID string
	wantLastID  string
}

func Test_AuditLogExport_Search_OneWait(t *testing.T) {
	// GIVEN the following events sourced from search exporter in order
	searchSource := &testEventSourceSearch{
		eventBatches: []testEventBatch{{
			nextKey: "a", // caught up with present - 1 minutes wait.
			events: []apievents.AuditEvent{
				&testEvent{id: 0, message: "batch:0 msg:0 "},
			},
		}, {
			nextKey: "",
			events: []apievents.AuditEvent{
				&testEvent{id: 10, message: "batch:1 msg:0 "},
			},
		}, {
			nextKey: "b",
			events: []apievents.AuditEvent{
				&testEvent{id: 10, message: "batch:1 msg:0 "},
				&testEvent{id: 11, message: "batch:1 msg:1 "},
			},
		}, {
			nextKey: "c",
			events: []apievents.AuditEvent{
				&testEvent{id: 20, message: "batch:2 msg:0 "},
			},
		}},
	}

	// WHEN we start an auditlog exporting client and an access-graph server stub
	lis, err := net.Listen("tcp", "localhost:0")
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	server := newAccessGraphServerStub(t, lis, newConfigAndState(), 4, cancel)
	clock := clockwork.NewFakeClock()
	exporter, err := newTestAuditLogExporter(t, ctx, lis, clock, searchSource)
	require.NoError(t, err)
	go exporter.start(ctx, AuditLogConfig{Enabled: true})
	clock.BlockUntilContext(ctx, 1) // wait until the first batch real-time mode (no nextKey)
	received2 := func() bool {
		return len(server.receivedEvents()) == 2
	}
	require.Eventually(t, received2, time.Minute, time.Second)
	clock.Advance(61 * time.Second)
	<-ctx.Done() // wait until context canceled by stub server

	// THEN we expect to track following requests in the stub server
	wantRequests := []wantRequest{{
		wantKey:     "a",
		wantLen:     1,
		wantFirstID: "0",
	}, {
		wantKey:     "a",
		wantLen:     1,
		wantFirstID: "10",
		wantLastID:  "10",
	}, {
		wantKey:     "b",
		wantLen:     1,
		wantFirstID: "11",
	}, {
		wantKey:     "c",
		wantLen:     1,
		wantFirstID: "20",
	},
	}

	gotEvents := server.receivedEvents()
	require.Len(t, gotEvents, len(wantRequests), gotEvents)
	for i, req := range gotEvents {
		w := wantRequests[i]
		gotEvents := req.GetEvents().GetEvents()
		require.Len(t, gotEvents, w.wantLen)
		require.Equal(t, w.wantFirstID, gotEvents[0].GetId())
		resumeState := req.GetEvents().GetSearchResumeState()
		require.Equal(t, w.wantLastID, resumeState.GetLastEventId(), "index i: %d, startKey: %v", i, resumeState.GetStartKey())
		require.Equal(t, w.wantKey, resumeState.GetStartKey())
	}
}

func Test_AuditLogExport_Search_TwoWaitNoneEmpty(t *testing.T) {
	// GIVEN the following events sourced from search exporter in order
	searchSource := &testEventSourceSearch{
		eventBatches: []testEventBatch{{
			nextKey: "a", // caught up with present - 1 minutes wait.
			events: []apievents.AuditEvent{
				&testEvent{id: 0, message: "batch:0 msg:0 "},
			},
		}, {
			nextKey: "",
			events: []apievents.AuditEvent{
				&testEvent{id: 10, message: "batch:1 msg:0 "},
			},
		}, {
			nextKey: "",
			events: []apievents.AuditEvent{
				&testEvent{id: 10, message: "batch:1 msg:0 "},
				&testEvent{id: 11, message: "batch:1 msg:1 "},
			},
		}, {
			nextKey: "b",
			events: []apievents.AuditEvent{
				&testEvent{id: 10, message: "batch:1 msg:0 "},
				&testEvent{id: 11, message: "batch:1 msg:1 "},
				&testEvent{id: 12, message: "batch:1 msg:2 "},
			},
		}, {
			nextKey: "c",
			events: []apievents.AuditEvent{
				&testEvent{id: 20, message: "batch:2 msg:0 "},
			},
		}},
	}

	// WHEN we start an auditlog exporting client and an access-graph server stub with synchronization points.
	lis, err := net.Listen("tcp", "localhost:0")
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	server := newAccessGraphServerStub(t, lis, newConfigAndState(), 5, cancel)
	clock := clockwork.NewFakeClock()
	exporter, err := newTestAuditLogExporter(t, ctx, lis, clock, searchSource)
	require.NoError(t, err)
	go exporter.start(ctx, AuditLogConfig{Enabled: true})
	clock.BlockUntilContext(ctx, 1)
	received2 := func() bool {
		return len(server.receivedEvents()) == 2
	}
	require.Eventually(t, received2, time.Minute, time.Second)
	clock.Advance(61 * time.Second)
	clock.BlockUntilContext(ctx, 1)
	require.Len(t, server.receivedEvents(), 2)
	clock.Advance(61 * time.Second)
	<-ctx.Done() // wait until context canceled by stub server.

	// THEN we expect to track following requests in the stub server
	wantRequests := []wantRequest{{
		wantKey:     "a",
		wantLen:     1,
		wantFirstID: "0",
	}, {
		wantKey:     "a",
		wantLen:     1,
		wantFirstID: "10",
		wantLastID:  "10",
	}, {
		wantKey:     "a",
		wantLen:     1,
		wantFirstID: "11",
		wantLastID:  "11",
	}, {
		wantKey:     "b",
		wantLen:     1,
		wantFirstID: "12",
	}, {
		wantKey:     "c",
		wantLen:     1,
		wantFirstID: "20",
	},
	}

	gotEvents := server.receivedEvents()
	require.Len(t, gotEvents, len(wantRequests), gotEvents)
	for i, req := range gotEvents {
		w := wantRequests[i]
		gotEvents := req.GetEvents().GetEvents()
		require.Len(t, gotEvents, w.wantLen)
		require.Equal(t, w.wantFirstID, gotEvents[0].GetId())

		resumeState := req.GetEvents().GetSearchResumeState()
		require.Equal(t, w.wantLastID, resumeState.GetLastEventId(), "index i: %d, startKey: %v", i, resumeState.GetStartKey())
		require.Equal(t, w.wantKey, resumeState.GetStartKey())
	}
}

func Test_AuditLogExport_Search_TwoWaitOneEmpty(t *testing.T) {
	// GIVEN the following events sourced from search exporter in order
	searchSource := &testEventSourceSearch{
		eventBatches: []testEventBatch{{
			nextKey: "a", // caught up with present - 1 minutes wait.
			events: []apievents.AuditEvent{
				&testEvent{id: 0, message: "batch:0 msg:0 "},
			},
		}, {
			nextKey: "",
			events: []apievents.AuditEvent{
				&testEvent{id: 10, message: "batch:1 msg:0 "},
			},
		}, {
			nextKey: "", // No request for this search result
			events: []apievents.AuditEvent{
				&testEvent{id: 10, message: "batch:1 msg:0 "},
			},
		}, {
			nextKey: "b",
			events: []apievents.AuditEvent{
				&testEvent{id: 10, message: "batch:1 msg:0 "},
				&testEvent{id: 11, message: "batch:1 msg:1 "},
			},
		}, {
			nextKey: "c",
			events: []apievents.AuditEvent{
				&testEvent{id: 20, message: "batch:2 msg:0 "},
			},
		}},
	}

	// WHEN we start an auditlog exporting client and an access-graph server stub with synchronization points.
	lis, err := net.Listen("tcp", "localhost:0")
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	server := newAccessGraphServerStub(t, lis, newConfigAndState(), 4, cancel) // skip 1
	clock := clockwork.NewFakeClock()
	exporter, err := newTestAuditLogExporter(t, ctx, lis, clock, searchSource)
	require.NoError(t, err)
	go exporter.start(ctx, AuditLogConfig{Enabled: true})
	clock.BlockUntilContext(ctx, 1)
	received2 := func() bool {
		return len(server.receivedEvents()) == 2
	}
	require.Eventually(t, received2, time.Minute, time.Second)
	clock.Advance(61 * time.Second)
	clock.BlockUntilContext(ctx, 1)
	require.Len(t, server.receivedEvents(), 2)
	clock.Advance(61 * time.Second)
	<-ctx.Done() // wait until context canceled by stub server

	// THEN we expect to track following requests in the stub server
	wantRequests := []wantRequest{{
		wantKey:     "a",
		wantLen:     1,
		wantFirstID: "0",
	}, {
		wantKey:     "a",
		wantLen:     1,
		wantFirstID: "10",
		wantLastID:  "10",
	}, { // no new request for searchEvents with index 2
		wantKey:     "b",
		wantLen:     1,
		wantFirstID: "11",
	}, {
		wantKey:     "c",
		wantLen:     1,
		wantFirstID: "20",
	},
	}

	gotEvents := server.receivedEvents()
	require.Len(t, gotEvents, len(wantRequests), gotEvents)
	for i, req := range gotEvents {
		w := wantRequests[i]
		gotEvents := req.GetEvents().GetEvents()
		require.Len(t, gotEvents, w.wantLen)
		require.Equal(t, w.wantFirstID, gotEvents[0].GetId())

		resumeState := req.GetEvents().GetSearchResumeState()
		require.Equal(t, w.wantLastID, resumeState.GetLastEventId(), "index i: %d, startKey: %v", i, resumeState.GetStartKey())
		require.Equal(t, w.wantKey, resumeState.GetStartKey())
	}
}

func Test_AuditLogExport_Search_WithResume(t *testing.T) {
	// GIVEN the following events sourced from search exporter in order
	searchSource := &testEventSourceSearch{
		eventBatches: []testEventBatch{{
			nextKey: "a",
			events: []apievents.AuditEvent{
				&testEvent{id: 0, message: "batch:0 msg:0 "},
				&testEvent{id: 1, message: "batch:0 msg:1 "},
			},
		}},
	}
	// WHEN we start an auditlog exporting client and an access-graph server stub
	lis, err := net.Listen("tcp", "localhost:0")
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	configAndState := newConfigAndState()
	configAndState[1] = &accessgraphv1.AuditLogStreamResponse{
		State: &accessgraphv1.AuditLogStreamResponse_SearchResumeState{
			SearchResumeState: &accessgraphv1.SearchResumeState{
				LastEventId: "0",
				StartKey:    "X",
			},
		},
	}
	server := newAccessGraphServerStub(t, lis, configAndState, 1, cancel)
	clock := clockwork.NewFakeClock()
	exporter, err := newTestAuditLogExporter(t, ctx, lis, clock, searchSource)
	require.NoError(t, err)
	exporter.start(ctx, AuditLogConfig{Enabled: true})

	// THEN we expect to track following requests in the stub server
	gotEvents := server.receivedEvents()
	require.Len(t, gotEvents, 1)
	require.Len(t, gotEvents[0].GetEvents().GetEvents(), 1) // not 2, because of re
	wantResumeState := &accessgraphv1.SearchResumeState{
		LastEventId: "",
		StartKey:    "a",
	}
	require.Equal(t, wantResumeState, gotEvents[0].GetEvents().GetSearchResumeState())
	require.Equal(t, "1", gotEvents[0].GetEvents().GetEvents()[0].GetId()) // not 2, because of re
	searchRequests := searchSource.receivedSearchRequests()
	require.Equal(t, "X", searchRequests[0].StartKey)
}

func newTestAuditLogExporter(t *testing.T, ctx context.Context, lis net.Listener, clock clockwork.Clock, eventSource events.AuditLogSessionStreamer) (*auditLogExporter, error) {
	authServer := newAuthServerForAuditLogTests(t, clock, eventSource)
	addr := lis.Addr().String()
	stream, err := newTestAuditLogStream(t, ctx, addr)
	if err != nil {
		return nil, err
	}
	require.NoError(t, err)

	exporter := &auditLogExporter{
		log:    slog.New(slog.DiscardHandler),
		client: authServer,
		stream: stream,
		clock:  clock,
	}
	return exporter, nil
}

func newConfigAndState() []*accessgraphv1.AuditLogStreamResponse {
	config := &accessgraphv1.AuditLogStreamResponse{
		State: &accessgraphv1.AuditLogStreamResponse_AuditLogConfig{
			AuditLogConfig: &accessgraphv1.AuditLogConfig{
				StartDate: timestamppb.New(time.Now().Add(-time.Hour)),
			},
		},
	}
	noResumeState := &accessgraphv1.AuditLogStreamResponse{
		State: &accessgraphv1.AuditLogStreamResponse_NoResumeState{},
	}
	return []*accessgraphv1.AuditLogStreamResponse{config, noResumeState}
}

func newAccessGraphServerStub(t *testing.T, lis net.Listener, configAndState []*accessgraphv1.AuditLogStreamResponse, receiveCount int, cancel context.CancelFunc) *accessGraphServerStub {
	t.Helper()
	localTLSConfig, err := fixtures.LocalTLSConfig()
	require.NoError(t, err)
	s := grpc.NewServer(grpc.Creds(credentials.NewTLS(localTLSConfig.TLS)))
	t.Cleanup(s.GracefulStop)
	server := &accessGraphServerStub{
		configAndState: configAndState,
		receiveCount:   receiveCount,
		cancel:         cancel,
	}
	accessgraphv1.RegisterAccessGraphServiceServer(s, server)
	go s.Serve(lis)
	t.Cleanup(s.Stop)
	return server
}

func (s *accessGraphServerStub) AuditLogStream(stream accessgraphv1.AccessGraphService_AuditLogStreamServer) error {
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

	// Receive receiveCount audit log event batches.
	for range s.receiveCount {
		req, err := stream.Recv()
		if err != nil {
			return err
		}
		s.mu.Lock()
		s.receivedAuditLogEvents = append(s.receivedAuditLogEvents, req)
		s.mu.Unlock()
	}
	s.cancel()
	return nil
}

func (s *accessGraphServerStub) receivedEvents() []*accessgraphv1.AuditLogStreamRequest {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.receivedAuditLogEvents)
}

func newTestAuditLogStream(t *testing.T, ctx context.Context, addr string) (auditLogStream, error) {
	t.Helper()
	clientConfig := ServiceClientConfig{
		Addr:     addr,
		Insecure: true,
	}
	getCreds := func() (*tls.Certificate, error) { return &fixtures.LocalhostTLSCertificate, nil }
	const serviceConfig = `{"loadBalancingConfig": [{"round_robin": {}}], "healthCheckConfig": {"serviceName": ""}}`
	conn, err := NewAccessGraphClient(ctx, clientConfig, getCreds, grpc.WithDefaultServiceConfig(serviceConfig))
	require.NoError(t, err)
	client := accessgraphv1.NewAccessGraphServiceClient(conn)
	return client.AuditLogStream(ctx)
}

func newAuthServerForAuditLogTests(t *testing.T, clock clockwork.Clock, eventSource events.AuditLogSessionStreamer) *auth.Server {
	t.Helper()
	memConfig := memory.Config{Clock: clock}
	backend, err := memory.New(memConfig)
	require.NoError(t, err)
	t.Cleanup(func() {
		err := backend.Close()
		require.NoError(t, err)
	})
	spec := types.ClusterNameSpecV2{ClusterName: "localhost"}
	clusterName, err := services.NewClusterNameWithRandomID(spec)
	require.NoError(t, err)
	clusterConfigService, err := local.NewClusterConfigurationService(backend)
	require.NoError(t, err)

	authConfig := &auth.InitConfig{
		ClusterName:            clusterName,
		Backend:                backend,
		ClusterConfiguration:   clusterConfigService,
		VersionStorage:         auth.NewFakeTeleportVersion(),
		Authority:              authority.New(),
		SkipPeriodicOperations: true,
		Clock:                  clock,
		AuditLog:               eventSource,
	}
	authServer, err := auth.NewServer(authConfig)
	require.NoError(t, err)

	t.Cleanup(func() {
		err := authServer.Close()
		require.NoError(t, err)
	})

	events := local.NewEventsService(backend)
	cache, err := services.NewUnifiedResourceCache(context.Background(), services.UnifiedResourceCacheConfig{
		Clock:          clock,
		ResourceGetter: authServer,
		ResourceWatcherConfig: services.ResourceWatcherConfig{
			Component: "resource-watcher",
			Client:    events,
		},
	})
	require.NoError(t, err)

	authServer.SetUnifiedResourcesCache(cache)
	return authServer
}

func (t *testEventSourceSearch) SearchEvents(ctx context.Context, req events.SearchEventsRequest) ([]apievents.AuditEvent, string, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.receivedRequests = append(t.receivedRequests, req)

	batch := t.eventBatches[t.index]
	t.index = (t.index + 1) % len(t.eventBatches)
	return batch.events, batch.nextKey, nil
}

func (t *testEventSourceSearch) GetEventExportChunks(ctx context.Context, req *auditlogpb.GetEventExportChunksRequest) stream.Stream[*auditlogpb.EventExportChunk] {
	// called to check if exporter is bulk or search, see auditLogExporter.isBulkExporter method.
	return stream.Fail[*auditlogpb.EventExportChunk](trace.NotImplemented("testEventSourceSearch does not implement GetEventExportChunks"))
}

func (t *testEventSourceSearch) Close() error {
	return nil
}

func (t *testEventSourceSearch) receivedSearchRequests() []events.SearchEventsRequest {
	t.mu.Lock()
	defer t.mu.Unlock()
	return slices.Clone(t.receivedRequests)
}

type testEvent struct {
	apievents.AuditEvent
	id      int
	message string
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
	return time.Now()
}
