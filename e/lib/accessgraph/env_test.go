package accessgraph

import (
	"context"
	"net"
	"testing"

	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/gravitational/teleport/api/constants"
	accessgraphsecretsv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/accessgraph/v1"
	"github.com/gravitational/teleport/api/types"
	accessgraphv1alpha "github.com/gravitational/teleport/gen/proto/go/accessgraph/v1alpha"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/backend/memory"
	dttestenv "github.com/gravitational/teleport/lib/devicetrust/testenv"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local"
	"github.com/gravitational/teleport/lib/services/readonly"
	"github.com/gravitational/teleport/lib/tlsca"
	usagereporter "github.com/gravitational/teleport/lib/usagereporter/teleport"
	"github.com/gravitational/teleport/lib/utils/log/logtest"
)

const clusterName = "test-cluster"

type env struct {
	service                   *Service
	secretsScannerClient      accessgraphsecretsv1pb.SecretsScannerServiceClient
	accessGraphClient         accessgraphv1alpha.AccessGraphServiceClient
	roleStorage               *local.AccessService
	userStorage               *local.IdentityService
	accessGraphSecretsStorage *local.AccessGraphSecretsService
	fakeAccessGraphServer     *serviceFake
	usageReporter             *fakeUsageReporter
}

type opts struct {
	authorizer           authz.Authorizer
	storedPrivateKeys    []*accessgraphsecretsv1pb.PrivateKey
	storedAuthorizedKeys []*accessgraphsecretsv1pb.AuthorizedKey
	device               *device
}

type device struct {
	device dttestenv.FakeDevice
	id     string
}

type option func(*opts)

func withAuthorizer(authorizer authz.Authorizer) option {
	return func(o *opts) {
		o.authorizer = authorizer
	}
}

func withPrivateKeys(privateKeys []*accessgraphsecretsv1pb.PrivateKey) option {
	return func(o *opts) {
		o.storedPrivateKeys = privateKeys
	}
}

func withAuthorizedKeys(authorizedKeys []*accessgraphsecretsv1pb.AuthorizedKey) option {
	return func(o *opts) {
		o.storedAuthorizedKeys = authorizedKeys
	}
}

func withDevice(deviceID string, dev dttestenv.FakeDevice) option {
	return func(o *opts) {
		o.device = &device{
			device: dev,
			id:     deviceID,
		}
	}
}

func setup(t *testing.T, ops ...option) env {
	t.Helper()

	o := opts{}
	for _, op := range ops {
		op(&o)
	}

	ctx := context.Background()

	clock := clockwork.NewFakeClock()
	backend, err := memory.New(memory.Config{Clock: clock})
	require.NoError(t, err)

	clusterConfigSvc, err := local.NewClusterConfigurationService(backend)
	require.NoError(t, err)

	trustSvc := local.NewCAService(backend)
	roleSvc := local.NewAccessService(backend)
	userSvc, err := local.NewIdentityService(backend)
	require.NoError(t, err)

	_, err = clusterConfigSvc.UpsertAuthPreference(ctx, types.DefaultAuthPreference())
	require.NoError(t, err)
	require.NoError(t, clusterConfigSvc.SetClusterAuditConfig(ctx, types.DefaultClusterAuditConfig()))
	_, err = clusterConfigSvc.UpsertClusterNetworkingConfig(ctx, types.DefaultClusterNetworkingConfig())
	require.NoError(t, err)
	_, err = clusterConfigSvc.UpsertSessionRecordingConfig(ctx, types.DefaultSessionRecordingConfig())
	require.NoError(t, err)

	accessPoint := &testClient{
		ClusterConfiguration: clusterConfigSvc,
		Trust:                trustSvc,
		RoleGetter:           roleSvc,
		UserGetter:           userSvc,
	}

	accessService := local.NewAccessService(backend)
	eventService := local.NewEventsService(backend)

	lockWatcher, err := services.NewLockWatcher(ctx, services.LockWatcherConfig{
		ResourceWatcherConfig: services.ResourceWatcherConfig{
			Client:    eventService,
			Component: "test",
		},
		LockGetter: accessService,
	})
	require.NoError(t, err)
	authorizer := o.authorizer
	if authorizer == nil {
		authorizer, err = authz.NewAuthorizer(authz.AuthorizerOpts{
			ClusterName: clusterName,
			AccessPoint: accessPoint,
			LockWatcher: lockWatcher,
		})
		require.NoError(t, err)
	}
	testFake := newServiceFake()

	svc, err := local.NewAccessGraphSecretsService(backend)
	require.NoError(t, err)

	for _, privateKey := range o.storedPrivateKeys {
		_, err := svc.UpsertPrivateKey(ctx, privateKey)
		require.NoError(t, err)
	}

	for _, authorizedKey := range o.storedAuthorizedKeys {
		_, err := svc.UpsertAuthorizedKey(ctx, authorizedKey)
		require.NoError(t, err)

	}
	var opts []dttestenv.Opt
	if o.device != nil {
		dev, pubKey, err := dttestenv.CreateEnrolledDevice(o.device.id, o.device.device)
		require.NoError(t, err)
		opts = append(opts, dttestenv.WithPreEnrolledDevice(dev, pubKey))
	}
	fakeSvc, err := dttestenv.New(opts...)
	require.NoError(t, err)

	t.Cleanup(func() {
		err := fakeSvc.Close()
		assert.NoError(t, err)
	})

	usageReporter := &fakeUsageReporter{}

	serviceNew, err := NewService(ServiceConfig{
		Authorizer:  authorizer,
		Logger:      logtest.NewLogger(),
		Client:      testFake,
		Storage:     svc,
		ClusterName: clusterName,
		AuthPreferenceGetter: func(ctx context.Context) (readonly.AuthPreference, error) {
			authPref := types.DefaultAuthPreference()
			authPref.SetDeviceTrust(&types.DeviceTrust{Mode: constants.DeviceTrustModeRequired})
			return authPref, nil
		},
		DeviceAssertionServer: fakeSvc.Service.CreateAssertCeremony,
		UsageReporter:         usageReporter,
	})
	require.NoError(t, err)

	grpcServer := grpc.NewServer()
	accessgraphv1alpha.RegisterAccessGraphServiceServer(grpcServer, serviceNew)
	accessgraphsecretsv1pb.RegisterSecretsScannerServiceServer(grpcServer, serviceNew)

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	go func() {
		err := grpcServer.Serve(lis)
		assert.NoError(t, err)
	}()
	t.Cleanup(func() {
		grpcServer.Stop()
		_ = lis.Close()
	})

	client, err := grpc.NewClient(
		lis.Addr().String(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)

	t.Cleanup(func() {
		err := client.Close()
		assert.NoError(t, err)
	})

	return env{
		service:                   serviceNew,
		accessGraphClient:         accessgraphv1alpha.NewAccessGraphServiceClient(client),
		secretsScannerClient:      accessgraphsecretsv1pb.NewSecretsScannerServiceClient(client),
		roleStorage:               roleSvc,
		userStorage:               userSvc,
		fakeAccessGraphServer:     testFake,
		accessGraphSecretsStorage: svc,
		usageReporter:             usageReporter,
	}
}

type testClient struct {
	services.ClusterConfiguration
	services.Trust
	services.RoleGetter
	services.UserGetter
}

func (e env) createUsersAndRoles(t *testing.T, ctx context.Context) {
	t.Helper()

	testRole, err := types.NewRole("test-role", types.RoleSpecV6{
		Allow: types.RoleConditions{
			Namespaces: []string{"*"},
			Rules: []types.Rule{
				{
					Resources: []string{types.KindAccessGraph},
					Verbs:     []string{types.VerbRead},
				},
			},
		},
	})

	require.NoError(t, err)

	_, err = e.roleStorage.CreateRole(ctx, testRole)
	require.NoError(t, err)

	testUser, err := types.NewUser("test-user")
	require.NoError(t, err)

	testUser.SetRoles([]string{})
	_, err = e.userStorage.CreateUser(ctx, testUser)
	require.NoError(t, err)

	testUserNoPerm, err := types.NewUser("test-user-no-perm")
	require.NoError(t, err)

	_, err = e.userStorage.CreateUser(ctx, testUserNoPerm)
	require.NoError(t, err)
}

func genUserContext(ctx context.Context, username string, groups []string, traits map[string][]string) context.Context {
	return authz.ContextWithUser(ctx, authz.LocalUser{
		Username: username,
		Identity: tlsca.Identity{
			Username: username,
			Groups:   groups,
			Traits:   traits,
		},
	})
}

type serviceFake struct {
	// Implements the AccessGraphServiceClient interface to avoid breaking changes
	// when TAG API is updated.
	accessgraphv1alpha.AccessGraphServiceClient
	calledFunctions map[string]int // map of function name to number of times called
}

func newServiceFake() *serviceFake {
	return &serviceFake{
		calledFunctions: make(map[string]int),
	}
}

func (s *serviceFake) Query(_ context.Context, _ *accessgraphv1alpha.QueryRequest, _ ...grpc.CallOption) (*accessgraphv1alpha.QueryResponse, error) {
	s.calledFunctions["Query"]++
	return &accessgraphv1alpha.QueryResponse{}, nil
}

func (s *serviceFake) GetFile(_ context.Context, _ *accessgraphv1alpha.GetFileRequest, _ ...grpc.CallOption) (*accessgraphv1alpha.GetFileResponse, error) {
	s.calledFunctions["GetFile"]++
	return &accessgraphv1alpha.GetFileResponse{}, nil
}

func (s *serviceFake) EventsStreamV2(_ context.Context, _ ...grpc.CallOption) (accessgraphv1alpha.AccessGraphService_EventsStreamV2Client, error) {
	s.calledFunctions["EventsStreamV2"]++
	return nil, nil
}

func (s *serviceFake) Register(ctx context.Context, in *accessgraphv1alpha.RegisterRequest, opts ...grpc.CallOption) (*accessgraphv1alpha.RegisterResponse, error) {
	s.calledFunctions["Register"]++
	return &accessgraphv1alpha.RegisterResponse{}, nil
}

func (s *serviceFake) ReplaceCAs(ctx context.Context, in *accessgraphv1alpha.ReplaceCAsRequest, opts ...grpc.CallOption) (*accessgraphv1alpha.ReplaceCAsResponse, error) {
	s.calledFunctions["ReplaceCAs"]++
	return &accessgraphv1alpha.ReplaceCAsResponse{}, nil
}

type fakeAuthorizer struct {
	identity *authz.Context
}

func (f fakeAuthorizer) Authorize(_ context.Context) (*authz.Context, error) {
	return f.identity, nil
}

type fakeUsageReporter struct {
	events []usagereporter.Anonymizable
}

func (f *fakeUsageReporter) AnonymizeAndSubmit(event ...usagereporter.Anonymizable) {
	f.events = append(f.events, event...)
}
