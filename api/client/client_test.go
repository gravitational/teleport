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
	"strconv"
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
	addr string
	grpc *grpc.Server
	*proto.UnimplementedAuthServiceServer
}

func newMockServer(addr string) *mockServer {
	m := &mockServer{
		addr,
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
	go newMockServer(l.Addr().String()).grpc.Serve(l)
	return l.Addr().String()
}

func (m *mockServer) NewClient(ctx context.Context) (*Client, error) {
	cfg := Config{
		Addrs: []string{m.addr},
		Credentials: []Credentials{
			&mockInsecureTLSCredentials{},
		},
		DialOpts: []grpc.DialOption{
			grpc.WithTransportCredentials(insecure.NewCredentials()),
		},
	}

	return New(ctx, cfg)
}

func (m *mockServer) Ping(ctx context.Context, req *proto.PingRequest) (*proto.PingResponse, error) {
	return &proto.PingResponse{}, nil
}

// Implement ListNodes handling of limit exceeded errors.
func (m *mockServer) ListNodes(ctx context.Context, req *proto.ListNodesRequest) (*proto.ListNodesResponse, error) {
	nodes, err := testNodes(req.Namespace)
	if err != nil {
		return nil, trail.ToGRPC(err)
	}

	// Implement simple pagination using StartKey as an index of nodes.
	resp := &proto.ListNodesResponse{}
	var startKeyInt int
	if req.StartKey != "" {
		startKeyInt, err = strconv.Atoi(req.StartKey)
		if err != nil {
			return nil, trail.ToGRPC(err)
		}
	}

	// retrieve nodes starting at startKey until we reach the limit or run out of nodes.
	for i := startKeyInt; i < startKeyInt+int(req.Limit) && i < len(nodes); i++ {
		node, ok := nodes[i].(*types.ServerV2)
		if !ok {
			return nil, trail.ToGRPC(trace.Errorf("Unexpected type: %T", nodes[i]))
		}
		resp.Servers = append(resp.Servers, node)
	}

	// Set NextKey to LastKey+1 if there are any pages remaining.
	if len(resp.Servers) == int(req.Limit) {
		resp.NextKey = fmt.Sprint(startKeyInt + int(req.Limit))
	}

	return resp, nil
}

func (m *mockServer) ListResources(ctx context.Context, req *proto.ListResourcesRequest) (*proto.ListResourcesResponse, error) {
	resources, err := testResources(req.ResourceType)
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

func testNodes(namespace string) ([]types.Server, error) {
	switch namespace {
	case fiveMBNode:
		node, err := types.NewServerWithLabels(
			"fiveMBNode",
			types.KindNode,
			types.ServerSpecV2{},
			map[string]string{
				// Artificially make a node ~ 5MB to force
				// ListNodes to fail regardless of chunk size.
				"label": string(make([]byte, 5000000)),
			},
		)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return []types.Server{node}, nil
	default:
		var err error
		nodes := make([]types.Server, 50)
		for i := 0; i < 50; i++ {
			if nodes[i], err = types.NewServerWithLabels(
				fmt.Sprintf("node%v", i),
				types.KindNode,
				types.ServerSpecV2{},
				map[string]string{
					// Artificially make each node ~ 100KB to force
					// ListNodes to fail with chunks of >= 40.
					"label": string(make([]byte, 100000)),
				},
			); err != nil {
				return nil, trace.Wrap(err)
			}
		}
		return nodes, nil
	}
}

func testResources(resourceType string) ([]types.Resource, error) {
	var err error
	size := 50
	resources := make([]types.Resource, size)

	switch resourceType {
	case types.KindDatabaseServer:
		for i := 0; i < size; i++ {
			resources[i], err = types.NewDatabaseServerV3(types.Metadata{
				Name: fmt.Sprintf("db-%d", i),
				Labels: map[string]string{
					// Artificially make each node ~ 100KB to force
					// ListResources to fail with chunks of >= 40.
					"label": string(make([]byte, 100000)),
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
					// Artificially make each node ~ 100KB to force
					// ListResources to fail with chunks of >= 40.
					"label": string(make([]byte, 100000)),
				},
			}, types.AppServerSpecV3{
				HostID: fmt.Sprintf("host-%d", i),
				App:    app,
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
	go newMockServer(l.Addr().String()).grpc.Serve(l)
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
	go newMockServer(l.Addr().String()).grpc.Serve(l)
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
	expectedNodes, err := testNodes(defaults.Namespace)
	require.NoError(t, err)

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
	}

	for name, test := range testCases {
		t.Run(name, func(t *testing.T) {
			expectedResources, err := testResources(test.resourceType)
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

type mockAccessRequestServer struct {
	*mockServer
}

func (g *mockAccessRequestServer) GetAccessRequests(ctx context.Context, f *types.AccessRequestFilter) (*proto.AccessRequests, error) {
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
	ctx := context.Background()
	l, err := net.Listen("tcp", "")
	require.NoError(t, err)

	m := &mockAccessRequestServer{
		&mockServer{
			addr:                           l.Addr().String(),
			grpc:                           grpc.NewServer(),
			UnimplementedAuthServiceServer: &proto.UnimplementedAuthServiceServer{},
		},
	}
	proto.RegisterAuthServiceServer(m.grpc, m)
	t.Cleanup(m.grpc.Stop)

	remoteErr := make(chan error)
	go func() {
		remoteErr <- m.grpc.Serve(l)
	}()

	clt, err := m.NewClient(ctx)
	require.NoError(t, err)

	items, err := clt.GetAccessRequests(ctx, types.AccessRequestFilter{})
	require.NoError(t, err)
	require.Len(t, items, 1)
	m.grpc.Stop()
	require.NoError(t, <-remoteErr)
}
