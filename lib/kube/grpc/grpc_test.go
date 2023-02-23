// Copyright 2023 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package kubev1

import (
	"context"
	"crypto/tls"
	"net"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/protobuf/testing/protocmp"
	"k8s.io/client-go/rest"

	"github.com/gravitational/teleport"
	proto "github.com/gravitational/teleport/api/gen/proto/go/teleport/kube/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth"
	kubeproxy "github.com/gravitational/teleport/lib/kube/proxy"
	testingkubemock "github.com/gravitational/teleport/lib/kube/proxy/testing/kube_server"
	"github.com/gravitational/teleport/lib/limiter"
	"github.com/gravitational/teleport/lib/utils"
)

func TestListKubernetesResources(t *testing.T) {
	var (
		usernameWithFullAccess = "full_user"
		usernameNoAccess       = "limited_user"
		kubeCluster            = "test_cluster"
		kubeUsers              = []string{"kube_user"}
		kubeGroups             = []string{"kube_user"}
	)
	// kubeMock is a Kubernetes API mock for the session tests.
	// Once a new session is created, this mock will write to
	// stdout and stdin (if available) the pod name, followed
	// by copying the contents of stdin into both streams.
	kubeMock, err := testingkubemock.NewKubeAPIMock()
	require.NoError(t, err)
	t.Cleanup(func() { kubeMock.Close() })

	// creates a Kubernetes service with a configured cluster pointing to mock api server
	testCtx := kubeproxy.SetupTestContext(
		context.Background(),
		t,
		kubeproxy.TestConfig{
			Clusters: []kubeproxy.KubeClusterConfig{{Name: kubeCluster, APIEndpoint: kubeMock.URL}},
		},
	)
	// close tests
	t.Cleanup(func() { require.NoError(t, testCtx.Close()) })

	grpcServerListener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	initGRPCServer(t, testCtx, grpcServerListener)

	userWithFullAccess, fullAccessRole := testCtx.CreateUserAndRole(
		testCtx.Context,
		t,
		usernameWithFullAccess,
		kubeproxy.RoleSpec{
			Name:       usernameWithFullAccess,
			KubeUsers:  kubeUsers,
			KubeGroups: kubeGroups,
		},
	)

	userNoAccess, _ := testCtx.CreateUserAndRole(
		testCtx.Context,
		t,
		usernameNoAccess,
		kubeproxy.RoleSpec{
			Name:       usernameNoAccess,
			KubeUsers:  kubeUsers,
			KubeGroups: kubeGroups,
			SetupRoleFunc: func(role types.Role) {
				// override the role to deny access to all kube resources.
				role.SetKubernetesLabels(types.Allow, nil)
				// set the role to allow searching as fullAccessRole.
				role.SetSearchAsRoles(types.Allow, []string{fullAccessRole.GetName()})
			},
		},
	)
	type args struct {
		user           types.User
		searchAsRoles  bool
		namespace      string
		searchKeywords []string
		sortBy         *types.SortBy
		startKey       string
	}
	tests := []struct {
		name      string
		args      args
		assertErr require.ErrorAssertionFunc
		want      *proto.ListKubernetesResourcesResponse
	}{
		{
			name: "user with full access and listing all namespaces",
			args: args{
				user:          userWithFullAccess,
				searchAsRoles: false,
			},
			want: &proto.ListKubernetesResourcesResponse{
				Resources: []*types.KubernetesResourceV1{
					{
						Kind:    "pod",
						Version: "v1",
						Metadata: types.Metadata{
							Name:      "nginx-1",
							Namespace: "default",
						},
						Spec: types.KubernetesResourceSpecV1{
							Namespace: "default",
						},
					},
					{
						Kind:    "pod",
						Version: "v1",
						Metadata: types.Metadata{
							Name:      "nginx-2",
							Namespace: "default",
						},
						Spec: types.KubernetesResourceSpecV1{
							Namespace: "default",
						},
					},
					{
						Kind:    "pod",
						Version: "v1",
						Metadata: types.Metadata{
							Name:      "test",
							Namespace: "default",
						},
						Spec: types.KubernetesResourceSpecV1{
							Namespace: "default",
						},
					},
					{
						Kind:    "pod",
						Version: "v1",
						Metadata: types.Metadata{
							Name:      "nginx-1",
							Namespace: "default",
						},
						Spec: types.KubernetesResourceSpecV1{
							Namespace: "dev",
						},
					},
					{
						Kind:    "pod",
						Version: "v1",
						Metadata: types.Metadata{
							Name:      "nginx-2",
							Namespace: "default",
						},
						Spec: types.KubernetesResourceSpecV1{
							Namespace: "dev",
						},
					},
				},
			},
			assertErr: require.NoError,
		},
		{
			name: "user with full access listing dev namespace",
			args: args{
				user:          userWithFullAccess,
				searchAsRoles: false,
				namespace:     "dev",
			},
			want: &proto.ListKubernetesResourcesResponse{
				Resources: []*types.KubernetesResourceV1{
					{
						Kind: "pod",
						Metadata: types.Metadata{
							Name: "nginx-1",
						},
						Spec: types.KubernetesResourceSpecV1{
							Namespace: "dev",
						},
					},
					{
						Kind: "pod",
						Metadata: types.Metadata{
							Name: "nginx-2",
						},
						Spec: types.KubernetesResourceSpecV1{
							Namespace: "dev",
						},
					},
				},
			},
			assertErr: require.NoError,
		},
		{
			name: "user with no access listing dev namespace",
			args: args{
				user:          userNoAccess,
				searchAsRoles: false,
				namespace:     "dev",
			},
			assertErr: require.Error,
		},
		{
			name: "user with no access listing dev namespace using search as roles",
			args: args{
				user:          userNoAccess,
				searchAsRoles: true,
				namespace:     "dev",
			},
			want: &proto.ListKubernetesResourcesResponse{
				Resources: []*types.KubernetesResourceV1{
					{
						Kind: "pod",
						Metadata: types.Metadata{
							Name: "nginx-1",
						},
						Spec: types.KubernetesResourceSpecV1{
							Namespace: "dev",
						},
					},
					{
						Kind: "pod",
						Metadata: types.Metadata{
							Name: "nginx-2",
						},
						Spec: types.KubernetesResourceSpecV1{
							Namespace: "dev",
						},
					},
				},
			},
			assertErr: require.NoError,
		},
		{
			name: "user with no access listing dev namespace using search as roles and filtering",
			args: args{
				user:           userNoAccess,
				searchAsRoles:  true,
				namespace:      "dev",
				searchKeywords: []string{"nginx-1"},
			},
			want: &proto.ListKubernetesResourcesResponse{
				Resources: []*types.KubernetesResourceV1{
					{
						Kind: "pod",
						Metadata: types.Metadata{
							Name: "nginx-1",
						},
						Spec: types.KubernetesResourceSpecV1{
							Namespace: "dev",
						},
					},
				},
			},
			assertErr: require.NoError,
		},
		{
			name: "user with no access listing dev namespace using search as roles and sort",
			args: args{
				user:          userNoAccess,
				searchAsRoles: true,
				namespace:     "dev",
				sortBy: &types.SortBy{
					Field:  "name",
					IsDesc: true,
				},
			},
			want: &proto.ListKubernetesResourcesResponse{
				TotalCount: 2,
				Resources: []*types.KubernetesResourceV1{
					{
						Kind: "pod",
						Metadata: types.Metadata{
							Name: "nginx-2",
						},
						Spec: types.KubernetesResourceSpecV1{
							Namespace: "dev",
						},
					},
					{
						Kind: "pod",
						Metadata: types.Metadata{
							Name: "nginx-1",
						},
						Spec: types.KubernetesResourceSpecV1{
							Namespace: "dev",
						},
					},
				},
			},
			assertErr: require.NoError,
		},
		{
			name: "user with no access listing dev namespace using search as roles and sort with start key",
			args: args{
				user:          userNoAccess,
				searchAsRoles: true,
				namespace:     "dev",
				sortBy: &types.SortBy{
					Field:  "name",
					IsDesc: true,
				},
				startKey: "nginx-1",
			},
			want: &proto.ListKubernetesResourcesResponse{
				TotalCount: 2,
				Resources: []*types.KubernetesResourceV1{
					{
						Kind: "pod",
						Metadata: types.Metadata{
							Name: "nginx-1",
						},
						Spec: types.KubernetesResourceSpecV1{
							Namespace: "dev",
						},
					},
				},
			},
			assertErr: require.NoError,
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, restCfg := testCtx.GenTestKubeClientTLSCert(t, tt.args.user.GetName(), "")

			tlsConfig, err := rest.TLSConfigFor(restCfg)
			require.NoError(t, err)
			kubeClient := newGrpcClient(testCtx.Context, t, grpcServerListener.Addr().String(), tlsConfig)

			rsp, err := kubeClient.ListKubernetesResources(
				context.Background(),
				&proto.ListKubernetesResourcesRequest{
					ResourceType:        types.KindKubePod,
					Limit:               100,
					KubernetesCluster:   kubeCluster,
					TeleportCluster:     testCtx.ClusterName,
					KubernetesNamespace: tt.args.namespace,
					UseSearchAsRoles:    tt.args.searchAsRoles,
					SearchKeywords:      tt.args.searchKeywords,
					SortBy:              tt.args.sortBy,
					StartKey:            tt.args.startKey,
				},
			)
			tt.assertErr(t, err)
			if tt.want != nil {
				for _, want := range tt.want.Resources {
					// fill in defaults
					err := want.CheckAndSetDefaults()
					require.NoError(t, err)
				}
			}
			require.Empty(t, cmp.Diff(rsp, tt.want, protocmp.Transform()))
		})
	}
}

// initGRPCServer creates a grpc server serving on the provided listener.
func initGRPCServer(t *testing.T, testCtx *kubeproxy.TestContext, listener net.Listener) {
	clusterName := testCtx.ClusterName
	serverIdentity, err := auth.NewServerIdentity(testCtx.AuthServer, testCtx.HostID, types.RoleProxy)
	require.NoError(t, err)
	tlsConfig, err := serverIdentity.TLSConfig(nil)
	require.NoError(t, err)
	limiter, err := limiter.NewLimiter(limiter.Config{MaxConnections: 100})
	require.NoError(t, err)
	// authMiddleware authenticates request assuming TLS client authentication
	// adds authentication information to the context
	// and passes it to the API server
	authMiddleware := &auth.Middleware{
		AccessPoint:   testCtx.AuthClient,
		Limiter:       limiter,
		AcceptedUsage: []string{teleport.UsageKubeOnly},
	}

	grpcServer := grpc.NewServer(
		grpc.ChainUnaryInterceptor(
			utils.GRPCServerUnaryErrorInterceptor,
			otelgrpc.UnaryServerInterceptor(),
			authMiddleware.UnaryInterceptor(),
		),
		grpc.ChainStreamInterceptor(
			utils.GRPCServerStreamErrorInterceptor,
			otelgrpc.StreamServerInterceptor(),
			authMiddleware.StreamInterceptor(),
		),
		grpc.Creds(credentials.NewTLS(
			copyAndConfigureTLS(tlsConfig, logrus.New(), testCtx.AuthClient, clusterName),
		)),
	)
	t.Cleanup(grpcServer.GracefulStop)
	// Auth client, lock watcher and authorizer for Kube proxy.
	proxyAuthClient, err := testCtx.TLSServer.NewClient(auth.TestBuiltin(types.RoleProxy))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, proxyAuthClient.Close()) })

	server, err := New(
		Config{
			ClusterName:   testCtx.ClusterName,
			Signer:        proxyAuthClient,
			AccessPoint:   proxyAuthClient,
			Emitter:       testCtx.Emitter,
			KubeProxyAddr: testCtx.KubeServiceAddress(),
			Authz:         testCtx.Authz,
		},
	)
	require.NoError(t, err)

	proto.RegisterKubeServiceServer(grpcServer, server)
	errC := make(chan error, 1)
	t.Cleanup(func() {
		grpcServer.GracefulStop()
		require.NoError(t, <-errC)
	})
	go func() {
		err := grpcServer.Serve(listener)
		errC <- trace.Wrap(err)
	}()
}

// copyAndConfigureTLS can be used to copy and modify an existing *tls.Config
// for Teleport application proxy servers.
func copyAndConfigureTLS(config *tls.Config, log logrus.FieldLogger, accessPoint auth.AccessCache, clusterName string) *tls.Config {
	tlsConfig := config.Clone()

	// Require clients to present a certificate
	tlsConfig.ClientAuth = tls.RequireAndVerifyClientCert

	// Configure function that will be used to fetch the CA that signed the
	// client's certificate to verify the chain presented. If the client does not
	// pass in the cluster name, this functions pulls back all CA to try and
	// match the certificate presented against any CA.
	tlsConfig.GetConfigForClient = auth.WithClusterCAs(tlsConfig.Clone(), accessPoint, clusterName, log)

	return tlsConfig
}

func newGrpcClient(ctx context.Context, t *testing.T, addr string, tlsConfig *tls.Config) proto.KubeServiceClient {
	conn, err := grpc.DialContext(
		ctx,
		addr,
		grpc.WithBlock(),
		grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)),
	)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, conn.Close()) })
	return proto.NewKubeServiceClient(conn)
}
