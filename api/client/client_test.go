/*
Copyright 2021 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package client

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/gravitational/trace"
	"github.com/gravitational/trace/trail"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
)

// mockServer mocks an Auth Server.
type mockServer struct {
	addr string
	grpc *grpc.Server
	*proto.UnimplementedAuthServiceServer
}

func newMockServer(addr string) *mockServer {
	m := &mockServer{
		addr:                           addr,
		grpc:                           grpc.NewServer(),
		UnimplementedAuthServiceServer: &proto.UnimplementedAuthServiceServer{},
	}
	proto.RegisterAuthServiceServer(m.grpc, m)
	return m
}

func (m *mockServer) Stop() {
	m.grpc.Stop()
}

func (m *mockServer) Addr() string {
	return m.addr
}

type ConfigOpt func(*Config)

func WithConfig(cfg Config) ConfigOpt {
	return func(config *Config) {
		*config = cfg
	}
}

func (m *mockServer) NewClient(ctx context.Context, opts ...ConfigOpt) (*Client, error) {
	cfg := Config{
		Addrs: []string{m.addr},
		Credentials: []Credentials{
			&mockInsecureTLSCredentials{}, // TODO(Joerger) replace insecure credentials
		},
		DialOpts: []grpc.DialOption{
			grpc.WithTransportCredentials(insecure.NewCredentials()), // TODO(Joerger) remove insecure dial option
		},
	}

	for _, opt := range opts {
		opt(&cfg)
	}

	return New(ctx, cfg)
}

// startMockServer starts a new mock server. Parallel tests cannot use the same addr.
func startMockServer(t *testing.T) *mockServer {
	l, err := net.Listen("tcp", "")
	require.NoError(t, err)
	return startMockServerWithListener(t, l)
}

// startMockServerWithListener starts a new mock server with the provided listener
func startMockServerWithListener(t *testing.T, l net.Listener) *mockServer {
	srv := newMockServer(l.Addr().String())

	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.grpc.Serve(l)
	}()

	t.Cleanup(func() {
		srv.grpc.Stop()
		require.NoError(t, <-errCh)
	})

	return srv
}

func (m *mockServer) Ping(ctx context.Context, req *proto.PingRequest) (*proto.PingResponse, error) {
	return &proto.PingResponse{}, nil
}

func (m *mockServer) ListResources(ctx context.Context, req *proto.ListResourcesRequest) (*proto.ListResourcesResponse, error) {
	resources, err := testResources(req.ResourceType, req.Namespace)
	if err != nil {
		return nil, trail.ToGRPC(err)
	}

	resp := &proto.ListResourcesResponse{
		Resources:  make([]*proto.PaginatedResource, 0),
		TotalCount: int32(len(resources)),
	}

	var (
		takeResources    = req.StartKey == ""
		lastResourceName string
	)
	for _, resource := range resources {
		if resource.GetName() == req.StartKey {
			takeResources = true
			continue
		}

		if !takeResources {
			continue
		}

		var protoResource *proto.PaginatedResource
		switch req.ResourceType {
		case types.KindDatabaseServer:
			database, ok := resource.(*types.DatabaseServerV3)
			if !ok {
				return nil, trace.Errorf("database server has invalid type %T", resource)
			}

			protoResource = &proto.PaginatedResource{Resource: &proto.PaginatedResource_DatabaseServer{DatabaseServer: database}}
		case types.KindAppServer:
			app, ok := resource.(*types.AppServerV3)
			if !ok {
				return nil, trace.Errorf("application server has invalid type %T", resource)
			}

			protoResource = &proto.PaginatedResource{Resource: &proto.PaginatedResource_AppServer{AppServer: app}}
		case types.KindNode:
			srv, ok := resource.(*types.ServerV2)
			if !ok {
				return nil, trace.Errorf("node has invalid type %T", resource)
			}

			protoResource = &proto.PaginatedResource{Resource: &proto.PaginatedResource_Node{Node: srv}}
		case types.KindKubeServer:
			srv, ok := resource.(*types.KubernetesServerV3)
			if !ok {
				return nil, trace.Errorf("kubernetes server has invalid type %T", resource)
			}

			protoResource = &proto.PaginatedResource{Resource: &proto.PaginatedResource_KubernetesServer{KubernetesServer: srv}}
		case types.KindWindowsDesktop:
			desktop, ok := resource.(*types.WindowsDesktopV3)
			if !ok {
				return nil, trace.Errorf("windows desktop has invalid type %T", resource)
			}

			protoResource = &proto.PaginatedResource{Resource: &proto.PaginatedResource_WindowsDesktop{WindowsDesktop: desktop}}
		}

		resp.Resources = append(resp.Resources, protoResource)
		lastResourceName = resource.GetName()
		if len(resp.Resources) == int(req.Limit) {
			break
		}
	}

	if len(resp.Resources) != len(resources) {
		resp.NextKey = lastResourceName
	}

	return resp, nil
}

func (m *mockServer) AddMFADeviceSync(ctx context.Context, req *proto.AddMFADeviceSyncRequest) (*proto.AddMFADeviceSyncResponse, error) {
	return nil, status.Error(codes.AlreadyExists, "Already Exists")
}

const fiveMBNode = "fiveMBNode"

func testResources(resourceType, namespace string) ([]types.ResourceWithLabels, error) {
	var err error
	size := 50
	// Artificially make each node ~ 100KB to force
	// ListResources to fail with chunks of >= 40.
	labelSize := 100000
	resources := make([]types.ResourceWithLabels, size)

	switch resourceType {
	case types.KindDatabaseServer:
		for i := 0; i < size; i++ {
			resources[i], err = types.NewDatabaseServerV3(types.Metadata{
				Name: fmt.Sprintf("db-%d", i),
				Labels: map[string]string{
					"label": string(make([]byte, labelSize)),
				},
			}, types.DatabaseServerSpecV3{
				Protocol: "",
				URI:      "localhost:5432",
				Hostname: "localhost",
				HostID:   fmt.Sprintf("host-%d", i),
			})

			if err != nil {
				return nil, trace.Wrap(err)
			}
		}
	case types.KindAppServer:
		for i := 0; i < size; i++ {
			app, err := types.NewAppV3(types.Metadata{
				Name: fmt.Sprintf("app-%d", i),
			}, types.AppSpecV3{
				URI: "localhost",
			})
			if err != nil {
				return nil, trace.Wrap(err)
			}

			resources[i], err = types.NewAppServerV3(types.Metadata{
				Name: fmt.Sprintf("app-%d", i),
				Labels: map[string]string{
					"label": string(make([]byte, labelSize)),
				},
			}, types.AppServerSpecV3{
				HostID: fmt.Sprintf("host-%d", i),
				App:    app,
			})

			if err != nil {
				return nil, trace.Wrap(err)
			}
		}
	case types.KindNode:
		for i := 0; i < size; i++ {
			nodeLabelSize := labelSize
			if namespace == fiveMBNode && i == 0 {
				// Artificially make a node ~ 5MB to force
				// ListNodes to fail regardless of chunk size.
				nodeLabelSize = 5000000
			}

			var err error
			resources[i], err = types.NewServerWithLabels(fmt.Sprintf("node-%d", i), types.KindNode, types.ServerSpecV2{},
				map[string]string{
					"label": string(make([]byte, nodeLabelSize)),
				},
			)
			if err != nil {
				return nil, trace.Wrap(err)
			}
		}
	case types.KindKubeServer:
		for i := 0; i < size; i++ {
			var err error
			name := fmt.Sprintf("kube-service-%d", i)
			kube, err := types.NewKubernetesClusterV3(types.Metadata{
				Name:   name,
				Labels: map[string]string{"name": name},
			},
				types.KubernetesClusterSpecV3{},
			)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			resources[i], err = types.NewKubernetesServerV3(
				types.Metadata{
					Name: name,
					Labels: map[string]string{
						"label": string(make([]byte, labelSize)),
					},
				},
				types.KubernetesServerSpecV3{
					HostID:  fmt.Sprintf("host-%d", i),
					Cluster: kube,
				},
			)
			if err != nil {
				return nil, trace.Wrap(err)
			}
		}
	case types.KindWindowsDesktop:
		for i := 0; i < size; i++ {
			var err error
			name := fmt.Sprintf("windows-desktop-%d", i)
			resources[i], err = types.NewWindowsDesktopV3(
				name,
				map[string]string{"label": string(make([]byte, labelSize))},
				types.WindowsDesktopSpecV3{
					Addr:   "_",
					HostID: "_",
				})
			if err != nil {
				return nil, trace.Wrap(err)
			}
		}

	default:
		return nil, trace.Errorf("unsupported resource type %s", resourceType)
	}

	return resources, nil
}

// mockInsecureCredentials mocks insecure Client credentials.
// it returns a nil tlsConfig which allows the client to run in insecure mode.
// TODO(Joerger) replace insecure credentials with proper TLS credentials.
type mockInsecureTLSCredentials struct{}

func (mc *mockInsecureTLSCredentials) Dialer(cfg Config) (ContextDialer, error) {
	return nil, trace.NotImplemented("no dialer")
}

func (mc *mockInsecureTLSCredentials) TLSConfig() (*tls.Config, error) {
	return nil, nil
}

func (mc *mockInsecureTLSCredentials) SSHClientConfig() (*ssh.ClientConfig, error) {
	return nil, trace.NotImplemented("no ssh config")
}

func TestNew(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	srv := startMockServer(t)

	tests := []struct {
		desc      string
		config    Config
		assertErr require.ErrorAssertionFunc
	}{{
		desc: "successfully dial tcp address.",
		config: Config{
			Addrs: []string{srv.Addr()},
			Credentials: []Credentials{
				&mockInsecureTLSCredentials{}, // TODO(Joerger) replace insecure credentials
			},
			DialOpts: []grpc.DialOption{
				grpc.WithTransportCredentials(insecure.NewCredentials()), // TODO(Joerger) remove insecure dial option
			},
		},
		assertErr: require.NoError,
	}, {
		desc: "synchronously dial addr/cred pairs and succeed with the 1 good pair.",
		config: Config{
			Addrs: []string{"bad addr", srv.Addr(), "bad addr"},
			Credentials: []Credentials{
				&tlsConfigCreds{nil},
				&mockInsecureTLSCredentials{}, // TODO(Joerger) replace insecure credentials
				&tlsConfigCreds{nil},
			},
			DialOpts: []grpc.DialOption{
				grpc.WithTransportCredentials(insecure.NewCredentials()), // TODO(Joerger) remove insecure dial option
			},
		},
		assertErr: require.NoError,
	}, {
		desc: "fail to dial with a bad address.",
		config: Config{
			DialTimeout: time.Second,
			Addrs:       []string{"bad addr"},
			Credentials: []Credentials{
				&mockInsecureTLSCredentials{}, // TODO(Joerger) replace insecure credentials
			},
			DialOpts: []grpc.DialOption{
				grpc.WithTransportCredentials(insecure.NewCredentials()), // TODO(Joerger) remove insecure dial option
			},
		},
		assertErr: func(t require.TestingT, err error, _ ...interface{}) {
			require.Error(t, err)
			require.Contains(t, err.Error(), "all connection methods failed")
		},
	}, {
		desc: "fail to dial with no address or dialer.",
		config: Config{
			DialTimeout: time.Second,
			Credentials: []Credentials{
				&mockInsecureTLSCredentials{}, // TODO(Joerger) replace insecure credentials
			},
			DialOpts: []grpc.DialOption{
				grpc.WithTransportCredentials(insecure.NewCredentials()), // TODO(Joerger) remove insecure dial option
			},
		},
		assertErr: func(t require.TestingT, err error, _ ...interface{}) {
			require.Error(t, err)
			require.Contains(t, err.Error(), "no connection methods found, try providing Dialer or Addrs in config")
		},
	}}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			clt, err := srv.NewClient(ctx, WithConfig(tt.config))
			tt.assertErr(t, err)

			if err == nil {
				t.Cleanup(func() { require.NoError(t, clt.Close()) })
				// requests to the server should succeed.
				_, err = clt.Ping(ctx)
				require.NoError(t, err)
			}
		})
	}
}

func TestNewDialBackground(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	// get listener but don't serve it yet.
	l, err := net.Listen("tcp", "")
	require.NoError(t, err)
	addr := l.Addr().String()

	// Create client before the server is listening.
	clt, err := New(ctx, Config{
		DialInBackground: true,
		Addrs:            []string{addr},
		Credentials: []Credentials{
			&mockInsecureTLSCredentials{}, // TODO(Joerger) replace insecure credentials
		},
		DialOpts: []grpc.DialOption{
			grpc.WithTransportCredentials(insecure.NewCredentials()), // TODO(Joerger) remove insecure dial option
		},
	})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, clt.Close()) })

	// requests to the server will result in a connection error.
	cancelCtx, cancel := context.WithTimeout(ctx, time.Second*3)
	defer cancel()
	_, err = clt.Ping(cancelCtx)
	require.Error(t, err)

	// Start the server and wait for the client connection to be ready.
	startMockServerWithListener(t, l)
	require.NoError(t, clt.waitForConnectionReady(ctx))

	// requests to the server should succeed.
	_, err = clt.Ping(ctx)
	require.NoError(t, err)
}

func TestWaitForConnectionReady(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	l, err := net.Listen("tcp", "")
	require.NoError(t, err)
	addr := l.Addr().String()

	// Create client before the server is listening.
	clt, err := New(ctx, Config{
		DialInBackground: true,
		Addrs:            []string{addr},
		Credentials: []Credentials{
			&mockInsecureTLSCredentials{}, // TODO(Joerger) replace insecure credentials
		},
		DialOpts: []grpc.DialOption{
			grpc.WithTransportCredentials(insecure.NewCredentials()), // TODO(Joerger) remove insecure dial option
		},
	})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, clt.Close()) })

	// WaitForConnectionReady should return false once the
	// context is canceled if the server isn't open to connections.
	cancelCtx, cancel := context.WithTimeout(ctx, time.Second*3)
	defer cancel()
	require.Error(t, clt.waitForConnectionReady(cancelCtx))

	// WaitForConnectionReady should return nil if the server is open to connections.
	startMockServerWithListener(t, l)
	require.NoError(t, clt.waitForConnectionReady(ctx))

	// WaitForConnectionReady should return an error if the grpc connection is closed.
	require.NoError(t, clt.Close())
	require.Error(t, clt.waitForConnectionReady(ctx))
}

func TestListResources(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	srv := startMockServer(t)

	testCases := map[string]struct {
		resourceType   string
		resourceStruct types.Resource
	}{
		"DatabaseServer": {
			resourceType:   types.KindDatabaseServer,
			resourceStruct: &types.DatabaseServerV3{},
		},
		"ApplicationServer": {
			resourceType:   types.KindAppServer,
			resourceStruct: &types.AppServerV3{},
		},
		"Node": {
			resourceType:   types.KindNode,
			resourceStruct: &types.ServerV2{},
		},
		"KubeServer": {
			resourceType:   types.KindKubeServer,
			resourceStruct: &types.KubernetesServerV3{},
		},
		"WindowsDesktop": {
			resourceType:   types.KindWindowsDesktop,
			resourceStruct: &types.WindowsDesktopV3{},
		},
	}

	// Create client
	clt, err := srv.NewClient(ctx)
	require.NoError(t, err)

	for name, test := range testCases {
		t.Run(name, func(t *testing.T) {
			resp, err := clt.ListResources(ctx, proto.ListResourcesRequest{
				Namespace:    defaults.Namespace,
				Limit:        10,
				ResourceType: test.resourceType,
			})
			require.NoError(t, err)
			require.NotEmpty(t, resp.NextKey)
			require.Len(t, resp.Resources, 10)
			require.IsType(t, test.resourceStruct, resp.Resources[0])

			// exceed the limit
			_, err = clt.ListResources(ctx, proto.ListResourcesRequest{
				Namespace:    defaults.Namespace,
				Limit:        50,
				ResourceType: test.resourceType,
			})
			require.Error(t, err)
			require.IsType(t, &trace.LimitExceededError{}, err.(*trace.TraceErr).OrigError())
		})
	}

	// Test a list with total count returned.
	resp, err := clt.ListResources(ctx, proto.ListResourcesRequest{
		ResourceType:   types.KindNode,
		Limit:          10,
		NeedTotalCount: true,
	})
	require.NoError(t, err)
	require.Equal(t, 50, resp.TotalCount)
}

func TestGetResources(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	srv := startMockServer(t)

	// Create client
	clt, err := srv.NewClient(ctx)
	require.NoError(t, err)

	testCases := map[string]struct {
		resourceType string
	}{
		"DatabaseServer": {
			resourceType: types.KindDatabaseServer,
		},
		"ApplicationServer": {
			resourceType: types.KindAppServer,
		},
		"Node": {
			resourceType: types.KindNode,
		},
		"KubeServer": {
			resourceType: types.KindKubeServer,
		},
		"WindowsDesktop": {
			resourceType: types.KindWindowsDesktop,
		},
	}

	for name, test := range testCases {
		t.Run(name, func(t *testing.T) {
			expectedResources, err := testResources(test.resourceType, defaults.Namespace)
			require.NoError(t, err)

			// Test listing everything at once errors with limit exceeded.
			_, err = clt.ListResources(ctx, proto.ListResourcesRequest{
				Namespace:    defaults.Namespace,
				Limit:        int32(len(expectedResources)),
				ResourceType: test.resourceType,
			})
			require.Error(t, err)
			require.IsType(t, &trace.LimitExceededError{}, err.(*trace.TraceErr).OrigError())

			// Test getting all resources by chunks to handle limit exceeded.
			resources, err := GetResourcesWithFilters(ctx, clt, proto.ListResourcesRequest{
				Namespace:    defaults.Namespace,
				ResourceType: test.resourceType,
			})
			require.NoError(t, err)
			require.Len(t, resources, len(expectedResources))
			require.Empty(t, cmp.Diff(expectedResources, resources))
		})
	}
}

type mockRoleServer struct {
	*mockServer
	roles map[string]*types.RoleV6
}

func newMockRoleServer() *mockRoleServer {
	m := &mockRoleServer{
		&mockServer{
			grpc:                           grpc.NewServer(),
			UnimplementedAuthServiceServer: &proto.UnimplementedAuthServiceServer{},
		},
		make(map[string]*types.RoleV6),
	}
	proto.RegisterAuthServiceServer(m.grpc, m)
	return m
}

func startMockRoleServer(t *testing.T) string {
	l, err := net.Listen("tcp", "")
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, l.Close()) })
	go newMockRoleServer().grpc.Serve(l)
	return l.Addr().String()
}

func (m *mockRoleServer) GetRole(ctx context.Context, req *proto.GetRoleRequest) (*types.RoleV6, error) {
	conn, ok := m.roles[req.Name]
	if !ok {
		return nil, trace.NotFound("not found")
	}
	return conn, nil
}

func (m *mockRoleServer) GetRoles(ctx context.Context, _ *emptypb.Empty) (*proto.GetRolesResponse, error) {
	var connectors []*types.RoleV6
	for _, conn := range m.roles {
		connectors = append(connectors, conn)
	}
	return &proto.GetRolesResponse{
		Roles: connectors,
	}, nil
}

func (m *mockRoleServer) UpsertRole(ctx context.Context, role *types.RoleV6) (*emptypb.Empty, error) {
	m.roles[role.Metadata.Name] = role
	return &emptypb.Empty{}, nil
}

func (m *mockRoleServer) GetCurrentUserRoles(_ *emptypb.Empty, stream proto.AuthService_GetCurrentUserRolesServer) error {
	for _, role := range m.roles {
		if err := stream.Send(role); err != nil {
			return trace.Wrap(err)
		}
	}

	return nil
}

// Test that client will perform properly with an old server
// DELETE IN 13.0.0
func TestSetRoleRequireSessionMFABackwardsCompatibility(t *testing.T) {
	ctx := context.Background()
	addr := startMockRoleServer(t)

	// Create client
	clt, err := New(ctx, Config{
		Addrs: []string{addr},
		Credentials: []Credentials{
			&mockInsecureTLSCredentials{}, // TODO(Joerger) replace insecure credentials
		},
		DialOpts: []grpc.DialOption{
			grpc.WithTransportCredentials(insecure.NewCredentials()), // TODO(Joerger) remove insecure dial option
		},
	})
	require.NoError(t, err)

	role := &types.RoleV6{
		Metadata: types.Metadata{
			Name: "one",
		},
	}

	t.Run("UpsertRole", func(t *testing.T) {
		// UpsertRole should set "RequireSessionMFA" on the provided role if "RequireMFAType" is set
		role.Spec.Options.RequireMFAType = types.RequireMFAType_SESSION
		role.Spec.Options.RequireSessionMFA = false
		err = clt.UpsertRole(ctx, role)
		require.NoError(t, err)
		require.True(t, role.GetOptions().RequireSessionMFA)
	})

	t.Run("GetRole", func(t *testing.T) {
		// GetRole should set "RequireMFAType" on the received role if empty
		role.Spec.Options.RequireMFAType = 0
		role.Spec.Options.RequireSessionMFA = true
		roleResp, err := clt.GetRole(ctx, role.GetName())
		require.NoError(t, err)
		require.Equal(t, types.RequireMFAType_SESSION, roleResp.GetOptions().RequireMFAType)
	})

	t.Run("GetRoles", func(t *testing.T) {
		// GetRoles should set "RequireMFAType" on the received roles if empty
		role.Spec.Options.RequireMFAType = 0
		role.Spec.Options.RequireSessionMFA = true
		rolesResp, err := clt.GetRoles(ctx)
		require.NoError(t, err)
		require.Len(t, rolesResp, 1)
		require.Equal(t, types.RequireMFAType_SESSION, rolesResp[0].GetOptions().RequireMFAType)
	})

	t.Run("GetCurrentUserRoles", func(t *testing.T) {
		// GetCurrentUserRoles should set "RequireMFAType" on the received roles if empty
		role.Spec.Options.RequireMFAType = 0
		role.Spec.Options.RequireSessionMFA = true
		rolesResp, err := clt.GetCurrentUserRoles(ctx)
		require.NoError(t, err)
		require.Len(t, rolesResp, 1)
		require.Equal(t, types.RequireMFAType_SESSION, rolesResp[0].GetOptions().RequireMFAType)
	})
}

type mockAuthPreferenceServer struct {
	*mockServer
	pref *types.AuthPreferenceV2
}

func newMockAuthPreferenceServer() *mockAuthPreferenceServer {
	m := &mockAuthPreferenceServer{
		mockServer: &mockServer{
			grpc:                           grpc.NewServer(),
			UnimplementedAuthServiceServer: &proto.UnimplementedAuthServiceServer{},
		},
	}
	proto.RegisterAuthServiceServer(m.grpc, m)
	return m
}

func startMockAuthPreferenceServer(t *testing.T) string {
	l, err := net.Listen("tcp", "")
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, l.Close()) })
	go newMockAuthPreferenceServer().grpc.Serve(l)
	return l.Addr().String()
}

func (m *mockAuthPreferenceServer) GetAuthPreference(ctx context.Context, _ *emptypb.Empty) (*types.AuthPreferenceV2, error) {
	if m.pref == nil {
		return nil, trace.NotFound("not found")
	}
	return m.pref, nil
}

func (m *mockAuthPreferenceServer) SetAuthPreference(ctx context.Context, pref *types.AuthPreferenceV2) (*emptypb.Empty, error) {
	m.pref = pref
	return &emptypb.Empty{}, nil
}

// Test that client will perform properly with an old server
// DELETE IN 13.0.0
func TestSetAuthPreferenceRequireSessionMFABackwardsCompatibility(t *testing.T) {
	ctx := context.Background()
	addr := startMockAuthPreferenceServer(t)

	// Create client
	clt, err := New(ctx, Config{
		Addrs: []string{addr},
		Credentials: []Credentials{
			&mockInsecureTLSCredentials{}, // TODO(Joerger) replace insecure credentials
		},
		DialOpts: []grpc.DialOption{
			grpc.WithTransportCredentials(insecure.NewCredentials()), // TODO(Joerger) remove insecure dial option
		},
	})
	require.NoError(t, err)

	pref := &types.AuthPreferenceV2{
		Metadata: types.Metadata{
			Name: "one",
		},
	}

	t.Run("SetAuthPreference", func(t *testing.T) {
		// SetAuthPreference should set "RequireSessionMFA" on the provided auth pref if "RequireMFAType" is set
		pref.Spec.RequireMFAType = types.RequireMFAType_SESSION
		pref.Spec.RequireSessionMFA = false
		err = clt.SetAuthPreference(ctx, pref)
		require.NoError(t, err)
		require.True(t, pref.Spec.RequireSessionMFA)
	})

	t.Run("GetAuthPreference", func(t *testing.T) {
		// GetAuthPreference should set "RequireMFAType" on the received auth pref if empty
		pref.Spec.RequireMFAType = 0
		pref.Spec.RequireSessionMFA = true
		prefResp, err := clt.GetAuthPreference(ctx)
		require.NoError(t, err)
		require.Equal(t, types.RequireMFAType_SESSION, prefResp.GetRequireMFAType())
	})
}
