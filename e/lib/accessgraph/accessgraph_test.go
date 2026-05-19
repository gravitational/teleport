package accessgraph

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"slices"
	"strconv"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/google/uuid"
	"github.com/jonboulle/clockwork"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/proto"

	authpb "github.com/gravitational/teleport/api/client/proto"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	accessgraphsecretsv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/accessgraph/v1"
	clusterconfigv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/clusterconfig/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accessgraph"
	"github.com/gravitational/teleport/api/types/clusterconfig"
	apievents "github.com/gravitational/teleport/api/types/events"
	accessgraphv1alpha "github.com/gravitational/teleport/gen/proto/go/accessgraph/v1alpha"
	prehogv1a "github.com/gravitational/teleport/gen/proto/go/prehog/v1alpha"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/authtest"
	authority "github.com/gravitational/teleport/lib/auth/testauthority"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/fixtures"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/modules/modulestest"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local"
	usagereporter "github.com/gravitational/teleport/lib/usagereporter/teleport"
	"github.com/gravitational/teleport/lib/utils/clocki"
)

type mockTagEventWatcher struct {
	events    []*accessgraphv1alpha.EventsStreamV2Request
	eventsMtx sync.Mutex
}

func (m *mockTagEventWatcher) Send(event *accessgraphv1alpha.EventsStreamV2Request) error {
	m.eventsMtx.Lock()
	defer m.eventsMtx.Unlock()

	m.events = append(m.events, event)
	return nil
}

type mockAccessRequestLister struct {
	requests []*types.AccessRequestV3
	calls    []authpb.ListAccessRequestsRequest
}

func (m *mockAccessRequestLister) ListAccessRequests(_ context.Context, req *authpb.ListAccessRequestsRequest) (*authpb.ListAccessRequestsResponse, error) {
	m.calls = append(m.calls, *req)

	start := 0
	if req.StartKey != "" {
		var err error
		start, err = strconv.Atoi(req.StartKey)
		if err != nil {
			return nil, err
		}
	}

	limit := int(req.Limit)
	if limit <= 0 {
		limit = len(m.requests)
	}
	end := min(start+limit, len(m.requests))

	nextKey := ""
	if end < len(m.requests) {
		nextKey = strconv.Itoa(end)
	}

	return &authpb.ListAccessRequestsResponse{
		AccessRequests: m.requests[start:end],
		NextKey:        nextKey,
	}, nil
}

type mockRoleLister struct {
	roles    []*types.RoleV6
	requests []authpb.ListRolesRequest
}

func (m *mockRoleLister) ListRoles(_ context.Context, req *authpb.ListRolesRequest) (*authpb.ListRolesResponse, error) {
	m.requests = append(m.requests, *req)

	start := 0
	if req.StartKey != "" {
		var err error
		start, err = strconv.Atoi(req.StartKey)
		if err != nil {
			return nil, err
		}
	}

	limit := int(req.Limit)
	if limit <= 0 {
		limit = len(m.roles)
	}
	end := min(start+limit, len(m.roles))

	nextKey := ""
	if end < len(m.roles) {
		nextKey = strconv.Itoa(end)
	}

	return &authpb.ListRolesResponse{
		Roles:   m.roles[start:end],
		NextKey: nextKey,
	}, nil
}

func unpackEvent(t *testing.T, event *accessgraphv1alpha.EventsStreamV2Request) *types.ServerV2 {
	t.Helper()

	require.NotNil(t, event)

	operation, ok := event.Operation.(*accessgraphv1alpha.EventsStreamV2Request_Upsert)
	require.True(t, ok)

	// assert that there is only one resource
	resources := operation.Upsert.Resources
	require.Len(t, resources, 1)

	// assert that the resource is a Server
	server, ok := resources[0].Resource.(*accessgraphv1alpha.ResourceEntry_Server)
	require.True(t, ok)

	return server.Server
}

func Test_tagEventWatcher_Send(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	mock := &mockTagEventWatcher{}
	eventWatcher := newTagEventWatcher(ctx, mock)

	// Init should be ignored
	err := eventWatcher.Send(types.Event{Type: types.OpInit})
	require.NoError(t, err)

	err = eventWatcher.Send(types.Event{
		Type:     types.OpPut,
		Resource: &types.ServerV2{Metadata: types.Metadata{Name: "1"}},
	})
	require.NoError(t, err)

	err = eventWatcher.markReady()
	require.NoError(t, err)

	err = eventWatcher.Send(types.Event{
		Type:     types.OpPut,
		Resource: &types.ServerV2{Metadata: types.Metadata{Name: "2"}},
	})
	require.NoError(t, err)

	require.Len(t, mock.events, 2)
	require.Equal(t, "1", unpackEvent(t, mock.events[0]).GetName())
	require.Equal(t, "2", unpackEvent(t, mock.events[1]).GetName())
}

func TestSendAccessRequestsPaginatedUpserts(t *testing.T) {
	t.Parallel()

	requestCount := apidefaults.DefaultChunkSize*2 + 3
	requests := make([]*types.AccessRequestV3, 0, requestCount)
	for i := range requestCount {
		req, err := types.NewAccessRequest(strconv.Itoa(i), "user", "role")
		require.NoError(t, err)
		requests = append(requests, req.(*types.AccessRequestV3))
	}

	stream := &mockTagEventWatcher{}
	lister := &mockAccessRequestLister{requests: requests}
	err := sendAccessRequests(context.Background(), lister, stream)
	require.NoError(t, err)

	require.Len(t, lister.calls, 3)
	require.Equal(t, int32(apidefaults.DefaultChunkSize), lister.calls[0].Limit)
	require.Empty(t, lister.calls[0].StartKey)
	require.Equal(t, strconv.Itoa(apidefaults.DefaultChunkSize), lister.calls[1].StartKey)
	require.Equal(t, strconv.Itoa(apidefaults.DefaultChunkSize*2), lister.calls[2].StartKey)

	require.Len(t, stream.events, requestCount)
	var gotNames []string
	for _, event := range stream.events {
		upsert := event.GetUpsert()
		require.NotNil(t, upsert)
		require.Len(t, upsert.Resources, 1)
		accessRequest := upsert.Resources[0].GetAccessRequest()
		require.NotNil(t, accessRequest)
		gotNames = append(gotNames, accessRequest.GetName())
	}
	require.Len(t, gotNames, requestCount)
	for i, name := range gotNames {
		require.Equal(t, strconv.Itoa(i), name)
	}
}

func TestSendRolesPaginatedUpserts(t *testing.T) {
	t.Parallel()

	roleCount := apidefaults.DefaultChunkSize + 2
	roles := make([]*types.RoleV6, 0, roleCount)
	for i := range roleCount {
		role, err := types.NewRole(strconv.Itoa(i), types.RoleSpecV6{})
		require.NoError(t, err)
		roles = append(roles, role.(*types.RoleV6))
	}

	stream := &mockTagEventWatcher{}
	lister := &mockRoleLister{roles: roles}
	err := sendRoles(context.Background(), lister, stream)
	require.NoError(t, err)

	require.Len(t, lister.requests, 2)
	require.Equal(t, int32(apidefaults.DefaultChunkSize), lister.requests[0].Limit)
	require.Empty(t, lister.requests[0].StartKey)
	require.Equal(t, strconv.Itoa(apidefaults.DefaultChunkSize), lister.requests[1].StartKey)

	require.Len(t, stream.events, roleCount)
	var gotNames []string
	for _, event := range stream.events {
		upsert := event.GetUpsert()
		require.NotNil(t, upsert)
		require.Len(t, upsert.Resources, 1)
		role := upsert.Resources[0].GetRole()
		require.NotNil(t, role)
		gotNames = append(gotNames, role.GetName())
	}
	require.Len(t, gotNames, roleCount)
	for i, name := range gotNames {
		require.Equal(t, strconv.Itoa(i), name)
	}
}

func Test_tagEventWatcher_Send_Concurrent(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	mock := &mockTagEventWatcher{}
	eventWatcher := newTagEventWatcher(ctx, mock)

	// Init should be ignored
	err := eventWatcher.Send(types.Event{Type: types.OpInit})
	require.NoError(t, err)

	for i := range 100 {
		err := eventWatcher.Send(types.Event{
			Type:     types.OpPut,
			Resource: &types.ServerV2{Metadata: types.Metadata{Name: strconv.Itoa(i)}},
		})
		assert.NoError(t, err)
	}

	// Watcher is not ready yet, so no events should be sent
	require.Empty(t, mock.events)

	wg := sync.WaitGroup{}
	// Send a bunch of events concurrently
	wg.Go(func() {
		for i := 100; i < 200; i++ {
			err := eventWatcher.Send(types.Event{
				Type:     types.OpPut,
				Resource: &types.ServerV2{Metadata: types.Metadata{Name: strconv.Itoa(i)}},
			})
			assert.NoError(t, err)
		}
	})

	// Mark ready. This should flush the cache and send all events
	err = eventWatcher.markReady()
	require.NoError(t, err)

	// wait for all events to be sent
	wg.Wait()

	require.Len(t, mock.events, 200)

	// All events should be in order
	for i := range 200 {
		require.Equal(t, strconv.Itoa(i), unpackEvent(t, mock.events[i]).GetName())
	}
}

func TestConvertEvent(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		inputEvent *accessgraphv1alpha.AuditEvent
		validate   func(t *testing.T, outputEvent apievents.AuditEvent)
	}{
		{
			name: "nil event",
			inputEvent: &accessgraphv1alpha.AuditEvent{
				Event: nil,
			},
			validate: func(t *testing.T, outputEvent apievents.AuditEvent) {
				require.Nil(t, outputEvent)
			},
		},
		{
			name: "AccessPathChanged event",
			inputEvent: &accessgraphv1alpha.AuditEvent{
				Event: &accessgraphv1alpha.AuditEvent_AccessPathChanged{
					AccessPathChanged: &accessgraphv1alpha.AccessPathChanged{
						ChangeId:               "sample-change-id",
						AffectedResourceName:   "sample-resource-name",
						AffectedResourceSource: "sample-resource-source",
					},
				},
			},
			validate: func(t *testing.T, outputEvent apievents.AuditEvent) {
				require.NotNil(t, outputEvent)
				require.Equal(t, events.AccessGraphAccessPathChangedEvent, outputEvent.GetType())
				require.Equal(t, events.AccessGraphAccessPathChangedCode, outputEvent.GetCode())

				accessPathEvent, ok := outputEvent.(*apievents.AccessPathChanged)
				require.True(t, ok)
				require.Equal(t, "sample-change-id", accessPathEvent.ChangeID)
				require.Equal(t, "sample-resource-name", accessPathEvent.AffectedResourceName)
				require.Equal(t, "sample-resource-source", accessPathEvent.AffectedResourceSource)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			auditEvent := convertEvent(tt.inputEvent)
			tt.validate(t, auditEvent)
		})
	}
}

type fakeUsageEventSender struct {
	usageCalls int
	lastUsage  usagereporter.Anonymizable
}

func (f *fakeUsageEventSender) EmitAuditEvent(ctx context.Context, e apievents.AuditEvent) error {
	return nil
}

func (f *fakeUsageEventSender) AnonymizeAndSubmit(event ...usagereporter.Anonymizable) {
	f.usageCalls++
	if len(event) > 0 {
		f.lastUsage = event[0]
	}
}

func TestProcessTAGMessageUsageEvents(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	log := slog.Default()

	tests := []struct {
		name      string
		msg       *accessgraphv1alpha.EventsStreamV2Response
		wantType  any
		wantCalls int
	}{
		{
			name: "graph size",
			msg: &accessgraphv1alpha.EventsStreamV2Response{
				Action: &accessgraphv1alpha.EventsStreamV2Response_UsageEvent{
					UsageEvent: &accessgraphv1alpha.UsageEvent{
						Event: &accessgraphv1alpha.UsageEvent_GraphSize{
							GraphSize: &prehogv1a.IdentitySecurityGraphSizeEvent{Provider: "teleport"},
						},
					},
				},
			},
			wantType:  (*usagereporter.IdentitySecurityGraphSizeEvent)(nil),
			wantCalls: 1,
		},
		{
			name: "audit logs ingested",
			msg: &accessgraphv1alpha.EventsStreamV2Response{
				Action: &accessgraphv1alpha.EventsStreamV2Response_UsageEvent{
					UsageEvent: &accessgraphv1alpha.UsageEvent{
						Event: &accessgraphv1alpha.UsageEvent_AuditLogsIngested{
							AuditLogsIngested: &prehogv1a.IdentitySecurityAuditLogsIngestedEvent{Provider: "teleport"},
						},
					},
				},
			},
			wantType:  (*usagereporter.IdentitySecurityAuditLogsIngestedEvent)(nil),
			wantCalls: 1,
		},
		{
			name: "nil graph size",
			msg: &accessgraphv1alpha.EventsStreamV2Response{
				Action: &accessgraphv1alpha.EventsStreamV2Response_UsageEvent{
					UsageEvent: &accessgraphv1alpha.UsageEvent{
						Event: &accessgraphv1alpha.UsageEvent_GraphSize{
							GraphSize: nil,
						},
					},
				},
			},
			wantCalls: 0,
		},
		{
			name: "nil audit logs ingested",
			msg: &accessgraphv1alpha.EventsStreamV2Response{
				Action: &accessgraphv1alpha.EventsStreamV2Response_UsageEvent{
					UsageEvent: &accessgraphv1alpha.UsageEvent{
						Event: &accessgraphv1alpha.UsageEvent_AuditLogsIngested{
							AuditLogsIngested: nil,
						},
					},
				},
			},
			wantCalls: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sender := &fakeUsageEventSender{}
			processTAGMessage(ctx, tt.msg, sender, log)

			require.Equal(t, tt.wantCalls, sender.usageCalls)
			if tt.wantType != nil {
				require.IsType(t, tt.wantType, sender.lastUsage)
			}
		})
	}
}

func TestTeleportAccessGraphSync(t *testing.T) {
	setAccessGraphConnected(accessGraphMetricStreamEvent, false)
	setAccessGraphConnected(accessGraphMetricStreamAuditLog, false)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	svc := initService(t)

	accessGraphSettings, err := clusterconfig.NewAccessGraphSettings(&clusterconfigv1.AccessGraphSettingsSpec{
		SecretsScanConfig: clusterconfigv1.AccessGraphSecretsScanConfig_ACCESS_GRAPH_SECRETS_SCAN_CONFIG_DISABLED,
	})
	require.NoError(t, err)
	_, err = svc.authServer.CreateAccessGraphSettings(ctx, accessGraphSettings)
	require.NoError(t, err)

	errC := make(chan error, 1)
	go func() {
		errC <- initializeAndWatchAccessGraph(
			ctx,
			slog.Default(),
			ServiceClientConfig{
				Addr:     svc.accessGraphListener.Addr().String(),
				Insecure: true,
			},
			func() (*tls.Certificate, error) {
				return &fixtures.LocalhostTLSCertificate, nil
			},
			svc.authServer,
			svc.bk,
		)
	}()

	require.Eventually(t, func() bool {
		for _, msg := range svc.accessGraphService.getReceivedMessages() {
			if msg.GetSync() != nil {
				return true
			}
		}
		return false
	}, 10*time.Second, 100*time.Millisecond, "expected to receive sync message before timeout")

	require.Eventually(t, func() bool {
		actions := svc.accessGraphService.getSupportedActions()
		return slices.Contains(actions, supportedActionUsageEvent)
	}, 10*time.Second, 100*time.Millisecond, "expected supported-actions metadata to be sent")

	require.Eventually(t, func() bool {
		return testutil.ToFloat64(accessGraphConnected.WithLabelValues(accessGraphMetricStreamEvent)) == 1
	}, 10*time.Second, 100*time.Millisecond, "expected access graph connected metric to be set")

	cancel()
	require.Eventually(t, func() bool {
		return testutil.ToFloat64(accessGraphConnected.WithLabelValues(accessGraphMetricStreamEvent)) == 0
	}, 10*time.Second, 100*time.Millisecond, "expected access graph connected metric to be reset")

	select {
	case err := <-errC:
		if err != nil && !errors.Is(err, context.Canceled) {
			require.NoError(t, err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("timed out waiting for access graph sync to stop")
	}
}

func newAccessGraphFakeService(t *testing.T, lis net.Listener) *accessGraphService {
	localTLSConfig, err := fixtures.LocalTLSConfig()
	require.NoError(t, err)
	tlsConfig := localTLSConfig.TLS.Clone()
	tlsConfig.InsecureSkipVerify = true
	tlsConfig.ClientAuth = tls.RequestClientCert
	tlsConfig.RootCAs = nil
	s := grpc.NewServer(
		grpc.Creds(
			credentials.NewTLS(tlsConfig),
		),
	)
	t.Cleanup(s.GracefulStop)

	accessService := &accessGraphService{}
	accessgraphv1alpha.RegisterAccessGraphServiceServer(s, accessService)

	healthService := health.NewServer()
	// empty service name is used to represent the health of the whole access graph instance
	healthService.SetServingStatus("" /* service */, healthpb.HealthCheckResponse_SERVING)
	healthpb.RegisterHealthServer(s, healthService)

	go s.Serve(lis)

	return accessService
}

type accessGraphService struct {
	accessgraphv1alpha.UnimplementedAccessGraphServiceServer
	healthpb.UnimplementedHealthServer

	// mu protects receivedMessages and supportedActions
	mu               sync.Mutex
	receivedMessages []*accessgraphv1alpha.EventsStreamV2Request
	supportedActions []string
	// kinds overrides the supported kinds sent in the stream header.
	// If nil, defaults to the standard set of cache-watcher kinds.
	kinds []string
}

func (a *accessGraphService) getReceivedMessages() []*accessgraphv1alpha.EventsStreamV2Request {
	a.mu.Lock()
	defer a.mu.Unlock()
	messages := make([]*accessgraphv1alpha.EventsStreamV2Request, len(a.receivedMessages))
	for i, msg := range a.receivedMessages {
		messages[i] = proto.Clone(msg).(*accessgraphv1alpha.EventsStreamV2Request)
	}
	return messages
}

func (a *accessGraphService) getSupportedActions() []string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return slices.Clone(a.supportedActions)
}

func (a *accessGraphService) EventsStreamV2(stream accessgraphv1alpha.AccessGraphService_EventsStreamV2Server) error {
	md, _ := metadata.FromIncomingContext(stream.Context())
	a.mu.Lock()
	a.supportedActions = md.Get(supportedActionsKey)
	a.mu.Unlock()

	kinds := a.kinds
	if len(kinds) == 0 {
		kinds = []string{
			types.KindUser,
			types.KindRole,
			types.KindNode,
			types.KindKubeServer,
			types.KindAppServer,
			types.KindWindowsDesktop,
			types.KindDatabaseServer,
			types.KindDatabaseObject,
			types.KindAccessRequest,
			"non_supported_kind", /* this kind is not supported  but is here to ensure that the cache runs with allow partials */
		}
	}
	if err := stream.SendHeader(metadata.MD{
		supportedKindsKey: kinds,
	}); err != nil {
		return fmt.Errorf("send header: %w", err)
	}

	for {
		recv, err := stream.Recv()
		if err != nil {
			return fmt.Errorf("receive: %w", err)
		}
		a.mu.Lock()
		a.receivedMessages = append(a.receivedMessages, recv)
		a.mu.Unlock()
	}
}

type testServiceComponents struct {
	clock               clocki.FakeClock
	authServer          *auth.Server
	bk                  backend.Backend
	accessGraphListener net.Listener
	accessGraphService  *accessGraphService
}

func initService(t *testing.T) testServiceComponents {
	t.Helper()

	clock := clockwork.NewFakeClock()
	backend, err := memory.New(memory.Config{
		Clock: clock,
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		err := backend.Close()
		require.NoError(t, err)
	})

	clusterName, err := services.NewClusterNameWithRandomID(types.ClusterNameSpecV2{
		ClusterName: "localhost",
	})
	require.NoError(t, err)
	clusterConfigService, err := local.NewClusterConfigurationService(backend)
	require.NoError(t, err)

	keygen, err := authority.NewKeygen(modules.BuildEnterprise, clock.Now)
	require.NoError(t, err)

	authConfig := &auth.InitConfig{
		ClusterName:            clusterName,
		Backend:                backend,
		ClusterConfiguration:   clusterConfigService,
		VersionStorage:         authtest.NewFakeTeleportVersion(),
		Authority:              keygen,
		SkipPeriodicOperations: true,
		Clock:                  clock,
		HostUUID:               uuid.NewString(),
		Modules:                modulestest.EnterpriseModules(),
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

	accessGraphListener, err := net.Listen("tcp", "localhost:0")
	require.NoError(t, err)

	t.Cleanup(func() {
		accessGraphListener.Close()
	})

	// Create a fake access graph service
	accessGraphService := newAccessGraphFakeService(t, accessGraphListener)

	return testServiceComponents{
		clock:               clock,
		authServer:          authServer,
		bk:                  backend,
		accessGraphService:  accessGraphService,
		accessGraphListener: accessGraphListener,
	}
}

// TestUserSecretsCleanup tests that user secrets are cleaned up from the types.User object
// before sending it to the access graph service.
func TestUserSecretsCleanup(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	svc := initService(t)
	accessGraphSettings, err := clusterconfig.NewAccessGraphSettings(&clusterconfigv1.AccessGraphSettingsSpec{
		SecretsScanConfig: clusterconfigv1.AccessGraphSecretsScanConfig_ACCESS_GRAPH_SECRETS_SCAN_CONFIG_ENABLED,
	})
	require.NoError(t, err)
	_, err = svc.authServer.CreateAccessGraphSettings(ctx, accessGraphSettings)
	require.NoError(t, err)

	user, err := types.NewUser("user1")
	require.NoError(t, err)
	hash, err := bcrypt.GenerateFromPassword([]byte("password"), bcrypt.MinCost)
	require.NoError(t, err)
	user.SetLocalAuth(&types.LocalAuthSecrets{
		PasswordHash: hash,
	})

	_, err = svc.authServer.Services.CreateUser(ctx, user)
	require.NoError(t, err)
	require.Eventually(t, func() bool {
		_, err := svc.authServer.GetUser(ctx, "user1", false)
		return err == nil
	}, 10*time.Second, 100*time.Millisecond, "expected to receive user before timeout")

	go func() {
		err := initializeAndWatchAccessGraph(
			ctx,
			slog.Default(),
			ServiceClientConfig{
				Addr:     svc.accessGraphListener.Addr().String(),
				Insecure: true,
			},
			func() (*tls.Certificate, error) {
				return &fixtures.LocalhostTLSCertificate, nil
			},
			svc.authServer,
			svc.bk,
		)
		if errors.Is(err, context.Canceled) {
			return
		}
		assert.NoError(t, err)
	}()

	require.Eventually(t, func() bool {
		usersFound := false
		noSecret := false
		hasSync := false
		for _, msg := range svc.accessGraphService.getReceivedMessages() {
			if msg.GetSync() != nil {
				hasSync = true
				continue
			}
			for _, msg := range msg.GetUpsert().Resources {
				if msg.GetUser() != nil {
					usersFound = true
					if msg.GetUser().Spec.LocalAuth == nil {
						noSecret = true
					} else {
						assert.Fail(t, "user secret was not cleaned up", "user: %v", msg.GetUser())
					}
				}
			}
		}
		return usersFound && noSecret && hasSync
	}, 10*time.Second, 100*time.Millisecond, "expected to receive non-secret user before timeout")
}

// TestProcessEventStream_WatcherCreation exercises the full watcher-initialization
// path inside processEventStream using a real auth server and an in-process gRPC
// server backed by bufconn.
//
// It verifies that both the cache watcher (KindRole) and the services watcher
// (KindAccessGraphSecretAuthorizedKey) are properly created and forward events.
// The services-watcher assertion is the regression check for the := vs = bug:
// with the old code servicesWatcher was left as noOpWatcher (nil Events channel)
// so all authorized-key events were silently dropped.
func TestProcessEventStream_WatcherCreation(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())

		as, err := authtest.NewAuthServer(authtest.AuthServerConfig{
			Dir:      t.TempDir(),
			AuditLog: events.NewDiscardAuditLog(),
			Modules:  modulestest.EnterpriseModules(),
		})
		require.NoError(t, err)

		// Wire an AccessGraphSecretsService so that authorized-key writes emit
		// backend events and sendAuthorizedKeys has a working lister.
		secretsSvc, err := local.NewAccessGraphSecretsService(as.Backend)
		require.NoError(t, err)
		as.AuthServer.SetAccessGraphSecretService(secretsSvc)

		// Build the in-process gRPC server via bufconn.
		// Advertise KindRole (→ cache watcher) and KindAccessGraphSecretAuthorizedKey (→ services watcher).
		lis := bufconn.Listen(1 << 20)
		grpcSrv := grpc.NewServer()
		tagSvc := &accessGraphService{
			kinds: []string{
				types.KindRole,
				types.KindAccessGraphSecretAuthorizedKey,
			},
		}
		accessgraphv1alpha.RegisterAccessGraphServiceServer(grpcSrv, tagSvc)
		go grpcSrv.Serve(lis)

		conn, err := grpc.NewClient(
			"passthrough:///bufconn",
			grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
				return lis.DialContext(ctx)
			}),
			grpc.WithTransportCredentials(insecure.NewCredentials()),
		)
		require.NoError(t, err)

		stream, err := accessgraphv1alpha.NewAccessGraphServiceClient(conn).EventsStreamV2(ctx)
		require.NoError(t, err)

		errc := make(chan error, 1)
		go func() {
			errc <- processEventStream(ctx, slog.Default(), stream, as.AuthServer)
		}()

		// processEventStream creates both watchers, lists all initial resources
		// (empty for a fresh backend), sends the sync message, and calls markReady().
		// forwardEventsWatch is now blocked on channel select.
		synctest.Wait()

		syncFound := false
		for _, msg := range tagSvc.getReceivedMessages() {
			if msg.GetSync() != nil {
				syncFound = true
				break
			}
		}
		require.True(t, syncFound, "sync not received after initialization")

		// Create a role; the cache watcher should forward the upsert event.
		role, err := types.NewRole("test-role", types.RoleSpecV6{})
		require.NoError(t, err)
		_, err = as.AuthServer.Services.UpsertRole(ctx, role)
		require.NoError(t, err)
		synctest.Wait()

		roleFound := false
		for _, msg := range tagSvc.getReceivedMessages() {
			for _, r := range msg.GetUpsert().GetResources() {
				if r.GetRole() != nil && r.GetRole().GetName() == "test-role" {
					roleFound = true
				}
			}
		}
		require.True(t, roleFound, "role upsert not received via cache watcher")

		// Create an authorized key; the services watcher should forward the upsert event.
		// Regression: the := bug left servicesWatcher as noOpWatcher (nil Events channel),
		// causing all authorized-key events to be silently dropped.
		authKey, err := accessgraph.NewAuthorizedKey(&accessgraphsecretsv1pb.AuthorizedKeySpec{
			HostId:         "host1",
			HostUser:       "user1",
			KeyFingerprint: "AAAAB3NzaC1yc2EAAAADAQABAAABAQC",
			KeyType:        "ssh-rsa",
		})
		require.NoError(t, err)
		_, err = secretsSvc.UpsertAuthorizedKey(ctx, authKey)
		require.NoError(t, err)
		synctest.Wait()

		authKeyFound := false
		for _, msg := range tagSvc.getReceivedMessages() {
			for _, r := range msg.GetUpsert().GetResources() {
				if r.GetAuthorizedKey() != nil && r.GetAuthorizedKey().GetSpec().GetHostId() == "host1" {
					authKeyFound = true
				}
			}
		}
		require.True(t, authKeyFound, "authorized key not received via services watcher")

		// Terminate: cancel the stream context, close the auth server (terminates
		// all watcher goroutines in the bubble), stop the gRPC server, close the
		// client connection, then drain.
		cancel()
		as.Close()
		grpcSrv.Stop()
		conn.Close()
		lis.Close()
		synctest.Wait()
		require.NoError(t, <-errc)
	})
}
