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
	"flag"
	"fmt"
	"net"
	"os"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/gravitational/trace"
	"github.com/gravitational/trace/trail"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
)

func TestMain(m *testing.M) {
	flag.Parse()
	if testing.Verbose() {
		logrus.SetLevel(logrus.DebugLevel)
	}
	os.Exit(m.Run())
}

type pingService struct {
	*proto.UnimplementedAuthServiceServer
}

func (s *pingService) Ping(ctx context.Context, req *proto.PingRequest) (*proto.PingResponse, error) {
	return &proto.PingResponse{}, nil
}

func TestNew(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	srv := startMockServer(t, &pingService{})

	tests := []struct {
		desc         string
		modifyConfig func(*Config)
		assertErr    require.ErrorAssertionFunc
	}{{
		desc:         "successfully dial tcp address.",
		modifyConfig: func(c *Config) { /* noop */ },
		assertErr:    require.NoError,
	}, {
		desc: "synchronously dial addr/cred pairs and succeed with the 1 good pair.",
		modifyConfig: func(c *Config) {
			c.Addrs = append(c.Addrs, "bad addr", "bad addr")
			c.Credentials = append([]Credentials{&tlsConfigCreds{nil}, &tlsConfigCreds{nil}}, c.Credentials...)
		},
		assertErr: require.NoError,
	}, {
		desc: "fail to dial with a bad address.",
		modifyConfig: func(c *Config) {
			c.Addrs = []string{"bad addr"}
		},
		assertErr: func(t require.TestingT, err error, _ ...interface{}) {
			require.Error(t, err)
			require.ErrorContains(t, err, "all connection methods failed")
		},
	}, {
		desc: "fail to dial with no address or dialer.",
		modifyConfig: func(c *Config) {
			c.Addrs = nil
		},
		assertErr: func(t require.TestingT, err error, _ ...interface{}) {
			require.Error(t, err)
			require.ErrorContains(t, err, "no connection methods found, try providing Dialer or Addrs in config")
		},
	}}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			cfg := srv.clientCfg()
			tt.modifyConfig(&cfg)

			clt, err := New(ctx, cfg)
			tt.assertErr(t, err)
			if err != nil {
				return
			}

			// Requests to the server should succeed.
			_, err = clt.Ping(ctx)
			assert.NoError(t, err, "Ping failed")
			assert.NoError(t, clt.Close(), "Close failed")
		})
	}
}

func TestNewDialBackground(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	// Create a server but don't serve it yet.
	l, err := net.Listen("tcp", "localhost:")
	require.NoError(t, err)
	addr := l.Addr().String()
	srv := newMockServer(t, addr, &pingService{})

	// Create client before the server is listening.
	cfg := srv.clientCfg()
	cfg.DialInBackground = true
	clt, err := New(ctx, cfg)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, clt.Close()) })

	// requests to the server will result in a connection error.
	cancelCtx, cancel := context.WithTimeout(ctx, time.Second*3)
	defer cancel()
	_, err = clt.Ping(cancelCtx)
	require.Error(t, err)

	// Server the listener and wait for the client connection to be ready.
	srv.serve(t, l)
	require.NoError(t, clt.waitForConnectionReady(ctx))

	// requests to the server should succeed.
	_, err = clt.Ping(ctx)
	require.NoError(t, err)
}

func TestWaitForConnectionReady(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	// Create a server but don't serve it yet.
	l, err := net.Listen("tcp", "localhost:")
	require.NoError(t, err)
	addr := l.Addr().String()
	srv := newMockServer(t, addr, &proto.UnimplementedAuthServiceServer{})

	// Create client before the server is listening.
	cfg := srv.clientCfg()
	cfg.DialInBackground = true
	clt, err := New(ctx, cfg)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, clt.Close()) })

	// WaitForConnectionReady should return an error once the
	// context is canceled if the server isn't open to connections.
	cancelCtx, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
	defer cancel()
	require.Error(t, clt.waitForConnectionReady(cancelCtx))

	// WaitForConnectionReady should return nil if the server is open to connections.
	srv.serve(t, l)
	require.NoError(t, clt.waitForConnectionReady(ctx))

	// WaitForConnectionReady should return an error if the grpc connection is closed.
	require.NoError(t, clt.Close())
	require.Error(t, clt.waitForConnectionReady(ctx))
}

type listResourcesService struct {
	*proto.UnimplementedAuthServiceServer
}

func (s *listResourcesService) ListResources(ctx context.Context, req *proto.ListResourcesRequest) (*proto.ListResourcesResponse, error) {
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
		case types.KindKubeService:
			srv, ok := resource.(*types.ServerV2)
			if !ok {
				return nil, trace.Errorf("kubernetes service has invalid type %T", resource)
			}

			protoResource = &proto.PaginatedResource{Resource: &proto.PaginatedResource_KubeService{KubeService: srv}}
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
			resources[i], err = types.NewKubernetesServerV3(types.Metadata{
				Name:   name,
				Labels: map[string]string{"name": name},
			},
				types.KubernetesServerSpecV3{
					Hostname: "test",
					Cluster: &types.KubernetesClusterV3{
						Metadata: types.Metadata{
							Name:   name,
							Labels: map[string]string{"name": name},
						},
					},
				},
			)
			if err != nil {
				return nil, trace.Wrap(err)
			}
		}
	case types.KindKubeService:
		for i := 0; i < size; i++ {
			var err error
			name := fmt.Sprintf("kube-service-%d", i)
			resources[i], err = types.NewServerWithLabels(name, types.KindKubeService, types.ServerSpecV2{
				KubernetesClusters: []*types.KubernetesCluster{
					{Name: name, StaticLabels: map[string]string{"name": name}},
				},
			}, map[string]string{
				"label": string(make([]byte, labelSize)),
			})

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

func TestListResources(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	srv := startMockServer(t, &listResourcesService{})

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
		"KubeService": {
			resourceType:   types.KindKubeService,
			resourceStruct: &types.ServerV2{},
		},
		"WindowsDesktop": {
			resourceType:   types.KindWindowsDesktop,
			resourceStruct: &types.WindowsDesktopV3{},
		},
	}

	// Create client
	clt, err := New(ctx, srv.clientCfg())
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
	srv := startMockServer(t, &listResourcesService{})

	// Create client
	clt, err := New(ctx, srv.clientCfg())
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
		"KubeService": {
			resourceType: types.KindKubeService,
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

type accessRequestService struct {
	*proto.UnimplementedAuthServiceServer
}

func (s *accessRequestService) GetAccessRequests(ctx context.Context, f *types.AccessRequestFilter) (*proto.AccessRequests, error) {
	req, err := types.NewAccessRequest("foo", "bob", "admin")
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &proto.AccessRequests{
		AccessRequests: []*types.AccessRequestV3{req.(*types.AccessRequestV3)},
	}, nil
}

// TestAccessRequestDowngrade tests that the client will downgrade to the non stream API for fetching access requests
// if the stream API is not available.
func TestAccessRequestDowngrade(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	server := startMockServer(t, &accessRequestService{})

	clt, err := New(ctx, server.clientCfg())
	require.NoError(t, err)

	items, err := clt.GetAccessRequests(ctx, types.AccessRequestFilter{})
	require.NoError(t, err)
	require.Len(t, items, 1)
}

type roleService struct {
	*proto.UnimplementedAuthServiceServer
	roles map[string]*types.RoleV6
}

func (s *roleService) GetRole(ctx context.Context, req *proto.GetRoleRequest) (*types.RoleV6, error) {
	role, ok := s.roles[req.Name]
	if !ok {
		return nil, trace.NotFound("not found")
	}
	return role, nil
}

func (s *roleService) GetRoles(ctx context.Context, _ *emptypb.Empty) (*proto.GetRolesResponse, error) {
	var roles []*types.RoleV6
	for _, role := range s.roles {
		roles = append(roles, role)
	}
	return &proto.GetRolesResponse{
		Roles: roles,
	}, nil
}

func (s *roleService) UpsertRole(ctx context.Context, role *types.RoleV6) (*emptypb.Empty, error) {
	s.roles[role.Metadata.Name] = role
	return &emptypb.Empty{}, nil
}

func (s *roleService) GetCurrentUserRoles(_ *emptypb.Empty, stream proto.AuthService_GetCurrentUserRolesServer) error {
	for _, role := range s.roles {
		if err := stream.Send(role); err != nil {
			return trace.Wrap(err)
		}
	}

	return nil
}

// Test that client will perform properly with an old server
// DELETE IN 13.0.0
func TestSetRoleRequireSessionMFABackwardsCompatibility(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	server := startMockServer(t, &roleService{
		roles: make(map[string]*types.RoleV6),
	})

	clt, err := New(ctx, server.clientCfg())
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

type authPreferenceService struct {
	*proto.UnimplementedAuthServiceServer
	pref *types.AuthPreferenceV2
}

func (s *authPreferenceService) GetAuthPreference(ctx context.Context, _ *emptypb.Empty) (*types.AuthPreferenceV2, error) {
	if s.pref == nil {
		return nil, trace.NotFound("not found")
	}
	return s.pref, nil
}

func (s *authPreferenceService) SetAuthPreference(ctx context.Context, pref *types.AuthPreferenceV2) (*emptypb.Empty, error) {
	s.pref = pref
	return &emptypb.Empty{}, nil
}

// Test that client will perform properly with an old server
// DELETE IN 13.0.0
func TestSetAuthPreferenceRequireSessionMFABackwardsCompatibility(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	server := startMockServer(t, &authPreferenceService{})

	clt, err := New(ctx, server.clientCfg())
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
