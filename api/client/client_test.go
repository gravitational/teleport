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
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/gravitational/trace"
	"github.com/gravitational/trace/trail"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
	"google.golang.org/grpc"
)

// mockServer mocks an Auth Server.
type mockServer struct {
	grpc *grpc.Server
	*proto.UnimplementedAuthServiceServer
}

func newMockServer() *mockServer {
	m := &mockServer{
		grpc.NewServer(),
		&proto.UnimplementedAuthServiceServer{},
	}
	proto.RegisterAuthServiceServer(m.grpc, m)
	return m
}

// startMockServer starts a new mock server. Parallel tests cannot use the same addr.
func startMockServer(t *testing.T) string {
	l, err := net.Listen("tcp", "")
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, l.Close()) })
	go newMockServer().grpc.Serve(l)
	return l.Addr().String()
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
		Resources: make([]*proto.PaginatedResource, 0),
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
	addr := startMockServer(t)

	tests := []struct {
		desc      string
		config    Config
		assertErr require.ErrorAssertionFunc
	}{{
		desc: "successfully dial tcp address.",
		config: Config{
			Addrs: []string{addr},
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
			Addrs: []string{"bad addr", addr, "bad addr"},
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
			require.True(t, strings.Contains(err.Error(), "all connection methods failed"))
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
			require.True(t, strings.Contains(err.Error(), "no connection methods found, try providing Dialer or Addrs in config"))
		},
	}}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			clt, err := New(ctx, tt.config)
			tt.assertErr(t, err)

			if err == nil {
				defer clt.Close()
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
	t.Cleanup(func() { require.NoError(t, l.Close()) })
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
	defer clt.Close()

	// requests to the server will result in a connection error.
	cancelCtx, cancel := context.WithTimeout(ctx, time.Second*3)
	defer cancel()
	_, err = clt.Ping(cancelCtx)
	require.Error(t, err)

	// Start the server and wait for the client connection to be ready.
	go newMockServer().grpc.Serve(l)
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
	t.Cleanup(func() { require.NoError(t, l.Close()) })
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
	defer clt.Close()

	// WaitForConnectionReady should return false once the
	// context is canceled if the server isn't open to connections.
	cancelCtx, cancel := context.WithTimeout(ctx, time.Second*3)
	defer cancel()
	require.Error(t, clt.waitForConnectionReady(cancelCtx))

	// WaitForConnectionReady should return nil if the server is open to connections.
	go newMockServer().grpc.Serve(l)
	require.NoError(t, clt.waitForConnectionReady(ctx))

	// WaitForConnectionReady should return an error if the grpc connection is closed.
	require.NoError(t, clt.GetConnection().Close())
	require.Error(t, clt.waitForConnectionReady(ctx))
}

func TestLimitExceeded(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	addr := startMockServer(t)

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

	// ListNodes should return a limit exceeded error when exceeding gRPC message size limit.
	_, _, err = clt.ListNodes(ctx, proto.ListNodesRequest{
		Namespace: defaults.Namespace,
		Limit:     50,
	})
	require.IsType(t, &trace.LimitExceededError{}, err.(*trace.TraceErr).OrigError())

	// GetNodes should retrieve all nodes and transparently handle limit exceeded errors.
	expectedResources, err := testResources(types.KindNode, defaults.Namespace)
	require.NoError(t, err)

	expectedNodes := make([]types.Server, len(expectedResources))
	for i, expectedResource := range expectedResources {
		var ok bool
		expectedNodes[i], ok = expectedResource.(*types.ServerV2)
		require.True(t, ok)
	}

	resp, err := clt.GetNodes(ctx, defaults.Namespace)
	require.NoError(t, err)
	require.EqualValues(t, expectedNodes, resp)

	// GetNodes should fail with a limit exceeded error if a
	// single node is too big to send over gRPC (over 4MB).
	_, err = clt.GetNodes(ctx, fiveMBNode)
	require.IsType(t, &trace.LimitExceededError{}, err.(*trace.TraceErr).OrigError())
}

func TestListResources(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	addr := startMockServer(t)

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
	}

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

	for name, test := range testCases {
		t.Run(name, func(t *testing.T) {
			resources, nextKey, err := clt.ListResources(ctx, proto.ListResourcesRequest{
				Namespace:    defaults.Namespace,
				Limit:        10,
				ResourceType: test.resourceType,
			})
			require.NoError(t, err)
			require.NotEmpty(t, nextKey)
			require.Len(t, resources, 10)
			require.IsType(t, test.resourceStruct, resources[0])

			// exceed the limit
			_, _, err = clt.ListResources(ctx, proto.ListResourcesRequest{
				Namespace:    defaults.Namespace,
				Limit:        50,
				ResourceType: test.resourceType,
			})
			require.Error(t, err)
			require.IsType(t, &trace.LimitExceededError{}, err.(*trace.TraceErr).OrigError())
		})
	}
}

func TestGetResources(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	addr := startMockServer(t)

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
	}

	for name, test := range testCases {
		t.Run(name, func(t *testing.T) {
			expectedResources, err := testResources(test.resourceType, defaults.Namespace)
			require.NoError(t, err)

			// listing everything at once breaks the ListResource.
			_, _, err = clt.ListResources(ctx, proto.ListResourcesRequest{
				Namespace:    defaults.Namespace,
				Limit:        int32(len(expectedResources)),
				ResourceType: test.resourceType,
			})
			require.Error(t, err)
			require.IsType(t, &trace.LimitExceededError{}, err.(*trace.TraceErr).OrigError())

			resources, err := clt.GetResources(ctx, defaults.Namespace, test.resourceType)
			require.NoError(t, err)
			require.Len(t, resources, len(expectedResources))
			require.Empty(t, cmp.Diff(expectedResources, resources))
		})
	}
}
