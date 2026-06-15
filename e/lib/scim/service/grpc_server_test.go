package service_test

import (
	"context"
	"crypto"
	"net"
	"testing"
	"testing/synctest"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/test/bufconn"

	scimpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/scim/v1"
	"github.com/gravitational/teleport/api/types"
	grpcinterceptors "github.com/gravitational/teleport/api/utils/grpc/interceptors"
	"github.com/gravitational/teleport/e/lib/scim/service"
	"github.com/gravitational/teleport/e/lib/scim/service/common"
	"github.com/gravitational/teleport/e/lib/scim/service/provider"
	"github.com/gravitational/teleport/entitlements"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/authtest"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/modules/modulestest"
	"github.com/gravitational/teleport/lib/services/local"
)

const (
	pluginName  = "scim-plugin"
	pluginID    = "scim-plugin-id"
	tokenSecret = "scim-bearer-token"
	authHeader  = "Bearer " + tokenSecret
)

func TestRateLimiting(t *testing.T) {
	t.Parallel()

	t.Run("rate limit enforced", func(t *testing.T) {
		t.Parallel()
		s := newSuite(t, common.RateLimitConfig{
			Average: 2, Burst: 2, PeriodSeconds: 60,
		})
		s.registerPlugin(t)

		ctx := s.proxyCtx()

		_, err := s.ListSCIMResources(ctx, s.listReq())
		require.NoError(t, err)

		_, err = s.ListSCIMResources(ctx, s.listReq())
		require.NoError(t, err)

		_, err = s.ListSCIMResources(ctx, s.listReq())
		require.True(t, trace.IsLimitExceeded(err))
	})

	t.Run("concurrency limit enforced", func(t *testing.T) {
		t.Parallel()
		synctest.Test(t, func(t *testing.T) {
			s := newSuite(t, common.RateLimitConfig{
				Average: 500, Burst: 1000, PeriodSeconds: 60, MaxConcurrentOperations: 1,
			})
			s.registerPlugin(t)
			ctx := s.proxyCtx()

			unblock := make(chan struct{})
			s.Service.CreatePluginHandler = s.blockingCreateHandler(unblock)

			done := make(chan struct{})
			go func() {
				defer close(done)
				_, _ = s.CreateSCIMResource(ctx, s.createReq())
			}()
			synctest.Wait()

			_, err := s.CreateSCIMResource(ctx, s.createReq())
			require.True(t, trace.IsLimitExceeded(err))

			close(unblock)
			<-done
		})
	})

	t.Run("retry-after trailer set on rate limit exceeded", func(t *testing.T) {
		t.Parallel()
		s := newSuite(t, common.RateLimitConfig{
			Average: 1, Burst: 1, PeriodSeconds: 60,
		})
		s.registerPlugin(t)
		client := s.newGRPCClient(t)

		ctx := context.Background()

		_, err := client.ListSCIMResources(ctx, s.listReq())
		require.NoError(t, err)

		var trailer metadata.MD
		_, err = client.ListSCIMResources(ctx, s.listReq(), grpc.Trailer(&trailer))
		require.True(t, trace.IsLimitExceeded(err))
		require.NotEmpty(t, trailer.Get("retry-after"))
	})

	t.Run("plugin rate limit overrides server default", func(t *testing.T) {
		t.Parallel()
		s := newSuite(t, common.RateLimitConfig{
			Average: 500, Burst: 500, PeriodSeconds: 60,
		})

		s.registerPluginWithRateLimit(t, &types.PluginSCIMRateLimit{
			Average: 2, Burst: 2, PeriodSeconds: 60,
		})

		ctx := s.proxyCtx()

		_, err := s.ListSCIMResources(ctx, s.listReq())
		require.NoError(t, err)

		_, err = s.ListSCIMResources(ctx, s.listReq())
		require.NoError(t, err)

		_, err = s.ListSCIMResources(ctx, s.listReq())
		require.True(t, trace.IsLimitExceeded(err), "expected LimitExceeded from plugin rate limit, got: %v", err)
	})

	t.Run("retry-after trailer set on concurrency limit exceeded", func(t *testing.T) {
		t.Parallel()
		synctest.Test(t, func(t *testing.T) {
			s := newSuite(t, common.RateLimitConfig{
				Average: 500, Burst: 1000, PeriodSeconds: 60, MaxConcurrentOperations: 1,
			})
			s.registerPlugin(t)
			client := s.newGRPCClient(t)

			unblock := make(chan struct{})
			s.Service.CreatePluginHandler = s.blockingCreateHandler(unblock)

			ctx := t.Context()

			done := make(chan struct{})
			go func() {
				defer close(done)
				_, _ = client.CreateSCIMResource(ctx, s.createReq())
			}()
			synctest.Wait()

			var trailer metadata.MD
			_, err := client.CreateSCIMResource(ctx, s.createReq(), grpc.Trailer(&trailer))
			require.True(t, trace.IsLimitExceeded(err))
			require.NotEmpty(t, trailer.Get("retry-after"))

			close(unblock)
			<-done
		})
	})
}

type accessPoint struct {
	*auth.Server
	*local.PluginsService
	*local.PluginStaticCredentialsService
	*local.AccessListService
}

func (accessPoint) GetJWTSigner(context.Context, types.CertAuthority) (crypto.Signer, error) {
	panic("GetJWTSigner should not be called in these tests")
}

type backend struct {
	*local.AccessListService
	*local.AccessService
	*local.OktaService
}

type proxyAuthorizer struct{}

func (proxyAuthorizer) Authorize(ctx context.Context) (*authz.Context, error) {
	return authz.ContextForBuiltinRole(authz.BuiltinRole{
		Role:     types.RoleProxy,
		Username: string(types.RoleProxy),
	}, nil)
}

type suite struct {
	*service.Service
	*local.PluginsService
	*local.PluginStaticCredentialsService
	*local.AccessListService
	scimConfig common.Config
}

func newSuite(t *testing.T, rl common.RateLimitConfig) *suite {
	t.Helper()

	clock := clockwork.NewFakeClock()

	authServer, err := authtest.NewAuthServer(authtest.AuthServerConfig{
		Dir:   t.TempDir(),
		Clock: clock,
		Modules: &modulestest.Modules{
			TestBuildType: modules.BuildEnterprise,
			TestFeatures: modules.Features{
				Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
					entitlements.OktaSCIM: {Enabled: true},
				},
			},
		},
	})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, authServer.Close()) })

	pluginsSvc := local.NewPluginsService(authServer.Backend)

	credsSvc, err := local.NewPluginStaticCredentialsService(authServer.Backend)
	require.NoError(t, err)

	aclSvc, err := local.NewAccessListServiceV2(local.AccessListServiceConfig{
		Backend: authServer.Backend,
		Modules: modulestest.EnterpriseModules(),
	})
	require.NoError(t, err)

	oktaSvc, err := local.NewOktaService(authServer.Backend, clock)
	require.NoError(t, err)

	cfg := common.Config{
		Authorizer: proxyAuthorizer{},
		AccessPoint: &accessPoint{
			Server:                         authServer.AuthServer,
			PluginsService:                 pluginsSvc,
			PluginStaticCredentialsService: credsSvc,
			AccessListService:              aclSvc,
		},
		Backend: &backend{
			AccessListService: aclSvc,
			AccessService:     local.NewAccessService(authServer.Backend),
			OktaService:       oktaSvc,
		},
		Clock:       clock,
		ClusterName: authServer.ClusterName,
		Modules:     authServer.Modules,
		RateLimit:   rl,
	}

	svc, err := service.NewService(&cfg)
	require.NoError(t, err)

	svc.CreatePluginHandler = provider.CreatePluginHandler

	return &suite{
		Service:                        svc,
		PluginsService:                 pluginsSvc,
		PluginStaticCredentialsService: credsSvc,
		AccessListService:              aclSvc,
		scimConfig:                     cfg,
	}
}

func (s *suite) proxyCtx() context.Context {
	return authz.ContextWithUser(
		context.Background(),
		authz.BuiltinRole{
			Role:     types.RoleProxy,
			Username: string(types.RoleProxy),
		})
}

func (s *suite) registerPlugin(t *testing.T) {
	t.Helper()
	ctx := t.Context()

	tokenHash, err := bcrypt.GenerateFromPassword([]byte(tokenSecret), bcrypt.MinCost)
	require.NoError(t, err)

	require.NoError(t, s.PluginStaticCredentialsService.CreatePluginStaticCredentials(ctx, &types.PluginStaticCredentialsV1{
		ResourceHeader: types.ResourceHeader{
			Metadata: types.Metadata{
				Name:   pluginName + "-cred",
				Labels: map[string]string{"plugin": pluginID},
			},
		},
		Spec: &types.PluginStaticCredentialsSpecV1{
			Credentials: &types.PluginStaticCredentialsSpecV1_APIToken{
				APIToken: string(tokenHash),
			},
		},
	}))

	require.NoError(t, s.PluginsService.CreatePlugin(ctx, &types.PluginV1{
		Kind:     types.KindPlugin,
		Version:  types.V1,
		Metadata: types.Metadata{Name: pluginName},
		Spec: types.PluginSpecV1{
			Settings: &types.PluginSpecV1_Scim{Scim: &types.PluginSCIMSettings{
				SamlConnectorName: "test-connector",
			}},
		},
		Credentials: &types.PluginCredentialsV1{
			Credentials: &types.PluginCredentialsV1_StaticCredentialsRef{
				StaticCredentialsRef: &types.PluginStaticCredentialsRef{
					Labels: map[string]string{"plugin": pluginID},
				},
			},
		},
	}))
}

func (s *suite) registerPluginWithRateLimit(t *testing.T, rl *types.PluginSCIMRateLimit) {
	t.Helper()
	ctx := context.Background()

	tokenHash, err := bcrypt.GenerateFromPassword([]byte(tokenSecret), bcrypt.MinCost)
	require.NoError(t, err)

	require.NoError(t, s.PluginStaticCredentialsService.CreatePluginStaticCredentials(ctx, &types.PluginStaticCredentialsV1{
		ResourceHeader: types.ResourceHeader{
			Metadata: types.Metadata{
				Name:   pluginName + "-cred",
				Labels: map[string]string{"plugin": pluginID},
			},
		},
		Spec: &types.PluginStaticCredentialsSpecV1{
			Credentials: &types.PluginStaticCredentialsSpecV1_APIToken{
				APIToken: string(tokenHash),
			},
		},
	}))

	require.NoError(t, s.PluginsService.CreatePlugin(ctx, &types.PluginV1{
		Kind:     types.KindPlugin,
		Version:  types.V1,
		Metadata: types.Metadata{Name: pluginName},
		Spec: types.PluginSpecV1{
			Settings: &types.PluginSpecV1_Scim{Scim: &types.PluginSCIMSettings{
				SamlConnectorName: "test-connector",
				RateLimit:         rl,
			}},
		},
		Credentials: &types.PluginCredentialsV1{
			Credentials: &types.PluginCredentialsV1_StaticCredentialsRef{
				StaticCredentialsRef: &types.PluginStaticCredentialsRef{
					Labels: map[string]string{"plugin": pluginID},
				},
			},
		},
	}))
}

func (s *suite) listReq() *scimpb.ListSCIMResourcesRequest {
	return scimpb.ListSCIMResourcesRequest_builder{
		Target: scimpb.RequestTarget_builder{
			Authorization: authHeader,
			PluginId:      pluginName,
			ResourceType:  "Users",
		}.Build(),
		Page: scimpb.Page_builder{StartIndex: 1, Count: 10}.Build(),
	}.Build()
}

func (s *suite) createReq() *scimpb.CreateSCIMResourceRequest {
	return scimpb.CreateSCIMResourceRequest_builder{
		Target: scimpb.RequestTarget_builder{
			Authorization: authHeader,
			PluginId:      pluginName,
			ResourceType:  "Users",
		}.Build(),
		Resource: &scimpb.Resource{},
	}.Build()
}

// blockingCreateHandler returns a CreatePluginHandler factory whose Create
// blocks until unblock is closed.
func (s *suite) blockingCreateHandler(unblock <-chan struct{}) func(types.Plugin, common.Config, string) (common.ResourceHandler, error) {
	return func(_ types.Plugin, _ common.Config, _ string) (common.ResourceHandler, error) {
		return &blockingHandler{unblock: unblock}, nil
	}
}

// blockingHandler is a ResourceHandler that blocks Create until unblock is closed.
type blockingHandler struct {
	common.NotImplementedHandler
	unblock <-chan struct{}
}

func (b *blockingHandler) CreateResource(ctx context.Context, _ *scimpb.CreateSCIMResourceRequest) (*scimpb.Resource, error) {
	select {
	case <-b.unblock:
		return scimpb.Resource_builder{Id: "alice", Meta: scimpb.Meta_builder{Location: "/Users/alice"}.Build()}.Build(), nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (b *blockingHandler) ListResources(_ context.Context, _ *scimpb.ListSCIMResourcesRequest) (*scimpb.ResourceList, error) {
	return &scimpb.ResourceList{}, nil
}

func (s *suite) newGRPCClient(t *testing.T) scimpb.SCIMServiceClient {
	t.Helper()

	const bufSize = 1024 * 1024
	lis := bufconn.Listen(bufSize)

	grpcSrv := grpc.NewServer(
		grpc.ChainUnaryInterceptor(grpcinterceptors.GRPCServerUnaryErrorInterceptor),
	)
	scimpb.RegisterSCIMServiceServer(grpcSrv, s.Service)

	go func() { _ = grpcSrv.Serve(lis) }()
	t.Cleanup(func() {
		grpcSrv.Stop()
		lis.Close()
	})

	conn, err := grpc.NewClient(
		"passthrough:///bufnet",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return lis.DialContext(ctx)
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithChainUnaryInterceptor(grpcinterceptors.GRPCClientUnaryErrorInterceptor),
	)
	require.NoError(t, err)
	t.Cleanup(func() { conn.Close() })

	return scimpb.NewSCIMServiceClient(conn)
}
