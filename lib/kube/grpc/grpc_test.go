/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package kubev1

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"log/slog"
	"net"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/protobuf/testing/protocmp"
	"k8s.io/client-go/rest"

	"github.com/gravitational/teleport"
	proto "github.com/gravitational/teleport/api/gen/proto/go/teleport/kube/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/authclient"
	testingkubemock "github.com/gravitational/teleport/lib/kube/proxy/testing/kube_server"
	"github.com/gravitational/teleport/lib/limiter"
	"github.com/gravitational/teleport/lib/modules"
)

func TestListKubernetesResources(t *testing.T) {
	modules.SetInsecureTestMode(true)
	var (
		usernameWithFullAccess                = "full_user"
		usernameNoAccess                      = "limited_user"
		usernameWithEnforceKubePodOrNamespace = "request_kind_enforce_pod_user"
		usernameWithEnforceKubeSecret         = "request_kind_enforce_secret_user"
		kubeCluster                           = "test_cluster"
		kubeUsers                             = []string{"kube_user"}
		kubeGroups                            = []string{"kube_user"}
	)
	// kubeMock is a Kubernetes API mock for the session tests.
	// Once a new session is created, this mock will write to
	// stdout and stdin (if available) the pod name, followed
	// by copying the contents of stdin into both streams.
	kubeMock, err := testingkubemock.NewKubeAPIMock()
	require.NoError(t, err)
	t.Cleanup(func() { kubeMock.Close() })

	// creates a Kubernetes service with a configured cluster pointing to mock api server
	testCtx := SetupTestContext(
		context.Background(),
		t,
		TestConfig{
			Clusters: []KubeClusterConfig{{Name: kubeCluster, APIEndpoint: kubeMock.URL}},
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
		RoleSpec{
			Name:       usernameWithFullAccess,
			KubeUsers:  kubeUsers,
			KubeGroups: kubeGroups,
			SetupRoleFunc: func(r types.Role) {
				// override the role to allow access to all kube resources.
				r.SetKubeResources(
					types.Allow,
					[]types.KubernetesResource{
						{
							Kind:      types.Wildcard,
							Name:      types.Wildcard,
							Namespace: types.Wildcard,
							Verbs:     []string{types.Wildcard},
							APIGroup:  types.Wildcard,
						},
					},
				)
			},
		},
	)

	userWithEnforceKubePodOrNamespace, _ := testCtx.CreateUserAndRole(
		testCtx.Context,
		t,
		usernameWithEnforceKubePodOrNamespace,
		RoleSpec{
			Name:       usernameWithEnforceKubePodOrNamespace,
			KubeUsers:  kubeUsers,
			KubeGroups: kubeGroups,
			SetupRoleFunc: func(role types.Role) {
				// override the role to deny access to all kube resources.
				role.SetKubernetesLabels(types.Allow, nil)
				// set the role to allow searching as fullAccessRole.
				role.SetSearchAsRoles(types.Allow, []string{fullAccessRole.GetName()})
				// restrict querying to pods only
				role.SetRequestKubernetesResources(types.Allow, []types.RequestKubernetesResource{{Kind: "namespace"}, {Kind: "pod"}})
			},
		},
	)

	userWithEnforceKubeSecret, _ := testCtx.CreateUserAndRole(
		testCtx.Context,
		t,
		usernameWithEnforceKubeSecret,
		RoleSpec{
			Name:       usernameWithEnforceKubeSecret,
			KubeUsers:  kubeUsers,
			KubeGroups: kubeGroups,
			SetupRoleFunc: func(role types.Role) {
				// override the role to deny access to all kube resources.
				role.SetKubernetesLabels(types.Allow, nil)
				// set the role to allow searching as fullAccessRole.
				role.SetSearchAsRoles(types.Allow, []string{fullAccessRole.GetName()})
				// restrict querying to secrets only
				role.SetRequestKubernetesResources(types.Allow, []types.RequestKubernetesResource{{Kind: "secret"}})

			},
		},
	)

	userNoAccess, _ := testCtx.CreateUserAndRole(
		testCtx.Context,
		t,
		usernameNoAccess,
		RoleSpec{
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
		resourceKind   string
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
				resourceKind:  types.KindKubePod,
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
				resourceKind:  types.KindKubePod,
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
				resourceKind:  types.KindKubePod,
			},
			assertErr: require.Error,
		},
		{
			name: "user with no access listing dev namespace using search as roles",
			args: args{
				user:          userNoAccess,
				searchAsRoles: true,
				namespace:     "dev",
				resourceKind:  types.KindKubePod,
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
				resourceKind:   types.KindKubePod,
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
				resourceKind: types.KindKubePod,
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
				startKey:     "nginx-1",
				resourceKind: types.KindKubePod,
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
		{
			name: "user with no access, deny listing dev pod, with role that enforces secret",
			args: args{
				user:          userWithEnforceKubeSecret,
				searchAsRoles: true,
				namespace:     "dev",
				resourceKind:  types.KindKubePod,
			},
			assertErr: require.Error,
		},
		{
			name: "user with no access, allow listing dev secret, with role that enforces secret",
			args: args{
				user:          userWithEnforceKubeSecret,
				searchAsRoles: true,
				namespace:     "dev",
				resourceKind:  types.KindKubeSecret,
			},
			want: &proto.ListKubernetesResourcesResponse{
				Resources: []*types.KubernetesResourceV1{
					{
						Kind:    types.KindKubeSecret,
						Version: "v1",
						Metadata: types.Metadata{
							Name: "secret-1",
						},
						Spec: types.KubernetesResourceSpecV1{
							Namespace: "dev",
						},
					},
					{
						Kind:    types.KindKubeSecret,
						Version: "v1",
						Metadata: types.Metadata{
							Name: "secret-2",
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
			name: "user with no access, allow listing dev pod, with role that enforces namespace or pods",
			args: args{
				user:          userWithEnforceKubePodOrNamespace,
				searchAsRoles: true,
				namespace:     "dev",
				resourceKind:  types.KindKubePod,
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
			name: "user with full access and listing secrets in all namespaces",
			args: args{
				user:          userWithFullAccess,
				searchAsRoles: false,
				resourceKind:  types.KindKubeSecret,
			},
			want: &proto.ListKubernetesResourcesResponse{
				Resources: []*types.KubernetesResourceV1{
					{
						Kind:    types.KindKubeSecret,
						Version: "v1",
						Metadata: types.Metadata{
							Name:      "secret-1",
							Namespace: "default",
						},
						Spec: types.KubernetesResourceSpecV1{
							Namespace: "default",
						},
					},
					{
						Kind:    types.KindKubeSecret,
						Version: "v1",
						Metadata: types.Metadata{
							Name:      "secret-2",
							Namespace: "default",
						},
						Spec: types.KubernetesResourceSpecV1{
							Namespace: "default",
						},
					},
					{
						Kind:    types.KindKubeSecret,
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
						Kind:    types.KindKubeSecret,
						Version: "v1",
						Metadata: types.Metadata{
							Name:      "secret-1",
							Namespace: "default",
						},
						Spec: types.KubernetesResourceSpecV1{
							Namespace: "dev",
						},
					},
					{
						Kind:    types.KindKubeSecret,
						Version: "v1",
						Metadata: types.Metadata{
							Name:      "secret-2",
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
			name: "user with full access and listing cluster roles",
			args: args{
				user:          userWithFullAccess,
				searchAsRoles: false,
				resourceKind:  types.KindKubeClusterRole,
			},
			want: &proto.ListKubernetesResourcesResponse{
				Resources: []*types.KubernetesResourceV1{
					{
						Kind:    types.KindKubeClusterRole,
						Version: "v1",
						Metadata: types.Metadata{
							Name:      "cr-nginx-1",
							Namespace: "default",
						},
						Spec: types.KubernetesResourceSpecV1{},
					},
					{
						Kind:    types.KindKubeClusterRole,
						Version: "v1",
						Metadata: types.Metadata{
							Name:      "cr-nginx-2",
							Namespace: "default",
						},
						Spec: types.KubernetesResourceSpecV1{},
					},
					{
						Kind:    types.KindKubeClusterRole,
						Version: "v1",
						Metadata: types.Metadata{
							Name:      "cr-test",
							Namespace: "default",
						},
						Spec: types.KubernetesResourceSpecV1{},
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
					ResourceType:        tt.args.resourceKind,
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
func initGRPCServer(t *testing.T, testCtx *TestContext, listener net.Listener) {
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
		ClusterName:   clusterName,
		Limiter:       limiter,
		AcceptedUsage: []string{teleport.UsageKubeOnly},
	}

	tlsConf := copyAndConfigureTLS(tlsConfig, testCtx.AuthClient, clusterName)
	creds, err := auth.NewTransportCredentials(auth.TransportCredentialsConfig{
		TransportCredentials: credentials.NewTLS(tlsConf),
		UserGetter:           authMiddleware,
	})
	require.NoError(t, err)

	grpcServer := grpc.NewServer(
		grpc.ChainUnaryInterceptor(authMiddleware.UnaryInterceptors()...),
		grpc.ChainStreamInterceptor(authMiddleware.StreamInterceptors()...),
		grpc.Creds(creds),
	)
	t.Cleanup(grpcServer.GracefulStop)
	// Auth client, lock watcher and authorizer for Kube proxy.
	proxyAuthClient, err := testCtx.TLSServer.NewClient(auth.TestBuiltin(types.RoleProxy))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, proxyAuthClient.Close()) })

	proxyTLSConfig, err := serverIdentity.TLSConfig(nil)
	require.NoError(t, err)
	require.Len(t, proxyTLSConfig.Certificates, 1)
	require.NotNil(t, proxyTLSConfig.RootCAs)

	server, err := New(
		Config{
			ClusterName: testCtx.ClusterName,
			GetConnTLSCertificate: func() (*tls.Certificate, error) {
				return &proxyTLSConfig.Certificates[0], nil
			},
			GetConnTLSRoots: func() (*x509.CertPool, error) {
				return proxyTLSConfig.RootCAs, nil
			},
			AccessPoint:   proxyAuthClient,
			Emitter:       testCtx.Emitter,
			KubeProxyAddr: testCtx.KubeProxyAddress(),
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
func copyAndConfigureTLS(config *tls.Config, accessPoint authclient.AccessCache, clusterName string) *tls.Config {
	tlsConfig := config.Clone()

	// Require clients to present a certificate
	tlsConfig.ClientAuth = tls.RequireAndVerifyClientCert

	// Configure function that will be used to fetch the CA that signed the
	// client's certificate to verify the chain presented. If the client does not
	// pass in the cluster name, this functions pulls back all CA to try and
	// match the certificate presented against any CA.
	tlsConfig.GetConfigForClient = authclient.WithClusterCAs(tlsConfig.Clone(), accessPoint, clusterName, slog.Default())

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
