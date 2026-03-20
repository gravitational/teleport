package accessgraph

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/proto"

	clusterconfigv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/clusterconfig/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/clusterconfig"
	apievents "github.com/gravitational/teleport/api/types/events"
	accessgraphv1alpha "github.com/gravitational/teleport/gen/proto/go/accessgraph/v1alpha"
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

	err = eventWatcher.Send(types.Event{Type: types.OpPut,
		Resource: &types.ServerV2{Metadata: types.Metadata{Name: "1"}},
	})
	require.NoError(t, err)

	err = eventWatcher.markReady()
	require.NoError(t, err)

	err = eventWatcher.Send(types.Event{Type: types.OpPut,
		Resource: &types.ServerV2{Metadata: types.Metadata{Name: "2"}},
	})
	require.NoError(t, err)

	require.Len(t, mock.events, 2)
	require.Equal(t, "1", unpackEvent(t, mock.events[0]).GetName())
	require.Equal(t, "2", unpackEvent(t, mock.events[1]).GetName())
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
		err := eventWatcher.Send(types.Event{Type: types.OpPut,
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
			err := eventWatcher.Send(types.Event{Type: types.OpPut,
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

func TestTeleportAccessGraphSync(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	svc := initService(t)

	accessGraphSettings, err := clusterconfig.NewAccessGraphSettings(&clusterconfigv1.AccessGraphSettingsSpec{
		SecretsScanConfig: clusterconfigv1.AccessGraphSecretsScanConfig_ACCESS_GRAPH_SECRETS_SCAN_CONFIG_DISABLED,
	})
	require.NoError(t, err)
	_, err = svc.authServer.CreateAccessGraphSettings(ctx, accessGraphSettings)
	require.NoError(t, err)

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
		assert.NoError(t, err)
	}()

	require.Eventually(t, func() bool {
		for _, msg := range svc.accessGraphService.getReceivedMessages() {
			if msg.GetSync() != nil {
				return true
			}
		}
		return false
	}, 10*time.Second, 100*time.Millisecond, "expected to receive sync message before timeout")
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

	// mu protects receivedMessages
	mu               sync.Mutex
	receivedMessages []*accessgraphv1alpha.EventsStreamV2Request
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

func (a *accessGraphService) EventsStreamV2(stream accessgraphv1alpha.AccessGraphService_EventsStreamV2Server) error {
	const supportedResourcesKey = "supported-kinds"
	if err := stream.SendHeader(metadata.MD{
		supportedResourcesKey: []string{
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
		},
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

	_, err = svc.authServer.Identity.CreateUser(ctx, user)
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
