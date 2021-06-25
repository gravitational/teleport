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
	"net"
	"strconv"
	"testing"
	"time"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
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

func (m *mockServer) Ping(ctx context.Context, req *proto.PingRequest) (*proto.PingResponse, error) {
	return &proto.PingResponse{}, nil
}

func (m *mockServer) ListNodes(ctx context.Context, req *proto.ListNodesRequest) (*proto.ListNodesResponse, error) {
	testNodes, err := testNodes()
	if err != nil {
		return nil, trail.ToGRPC(err)
	}

	// Implement simple pagination uses StartKey as an index of the test nodes.
	resp := &proto.ListNodesResponse{}
	var startKeyInt int
	if req.StartKey != "" {
		startKeyInt, err = strconv.Atoi(req.StartKey)
		if err != nil {
			return nil, trail.ToGRPC(err)
		}
	}

	// retrieve nodes starting at startKey until we reach the limit or run out of nodes.
	for i := startKeyInt; i < startKeyInt+int(req.Limit) && i < len(testNodes); i++ {
		node, ok := testNodes[i].(*types.ServerV2)
		if !ok {
			return nil, trail.ToGRPC(trace.Errorf("Unexpected type: %T", testNodes[i]))
		}
		resp.Servers = append(resp.Servers, node)
	}

	// Set NextKey to LastKey+1 if there are any pages remaining.
	if len(resp.Servers) == int(req.Limit) {
		resp.NextKey = strconv.Itoa(startKeyInt + int(req.Limit))
	}

	return resp, nil
}

func testNodes() ([]types.Server, error) {
	var err error
	nodes := make([]types.Server, 1000)
	for i := 0; i < 1000; i++ {
		nodes[i], err = types.NewServer("node"+strconv.Itoa(i), types.KindNode, types.ServerSpecV2{})
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}
	return nodes, nil
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

// startMockServer starts a new mock server. Parallel tests cannot use the same addr.
func startMockServer(t *testing.T, addr string) {
	l, err := net.Listen("tcp", addr)
	require.NoError(t, err)
	go newMockServer().grpc.Serve(l)
	t.Cleanup(func() { require.NoError(t, l.Close()) })
}

func TestNew(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	addr := "localhost:3025"
	startMockServer(t, addr)

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
				grpc.WithInsecure(), // TODO(Joerger) remove insecure dial option
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
				grpc.WithInsecure(), // TODO(Joerger) remove insecure dial option
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
				grpc.WithInsecure(), // TODO(Joerger) remove insecure dial option
			},
		},
		assertErr: func(t require.TestingT, err error, _ ...interface{}) {
			require.EqualError(t, err, "all auth methods failed\n\tcontext deadline exceeded")
		},
	}}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			t.Parallel()
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
	addr := "localhost:4025"

	// Create client before the server is listening.
	clt, err := New(ctx, Config{
		DialInBackground: true,
		Addrs:            []string{addr},
		Credentials: []Credentials{
			&mockInsecureTLSCredentials{}, // TODO(Joerger) replace insecure credentials
		},
		DialOpts: []grpc.DialOption{
			grpc.WithInsecure(), // TODO(Joerger) remove insecure dial option
		},
	})
	require.NoError(t, err)
	defer clt.Close()

	// requests to the server will result in a connection error.
	_, err = clt.Ping(ctx)
	require.Error(t, err)
	require.True(t, trace.IsConnectionProblem(err))

	// Start the server and wait for the client connection to be ready.
	startMockServer(t, addr)
	require.NoError(t, clt.waitForConnectionReady(ctx))

	// requests to the server should succeed.
	_, err = clt.Ping(ctx)
	require.NoError(t, err)
}

func TestWaitForConnectionReady(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	addr := "localhost:5025"

	// Create client before the server is listening.
	clt, err := New(ctx, Config{
		DialInBackground: true,
		Addrs:            []string{addr},
		Credentials: []Credentials{
			&mockInsecureTLSCredentials{}, // TODO(Joerger) replace insecure credentials
		},
		DialOpts: []grpc.DialOption{
			grpc.WithInsecure(), // TODO(Joerger) remove insecure dial option
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
	startMockServer(t, addr)
	require.NoError(t, clt.waitForConnectionReady(ctx))

	// WaitForConnectionReady should return an error if the grpc connection is closed.
	require.NoError(t, clt.GetConnection().Close())
	require.Error(t, clt.waitForConnectionReady(ctx))
}

func TestEndpoints(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	addr := "localhost:6025"
	startMockServer(t, addr)

	// Create client
	clt, err := New(ctx, Config{
		Addrs: []string{addr},
		Credentials: []Credentials{
			&mockInsecureTLSCredentials{}, // TODO(Joerger) replace insecure credentials
		},
		DialOpts: []grpc.DialOption{
			grpc.WithInsecure(), // TODO(Joerger) remove insecure dial option
		},
	})
	require.NoError(t, err)

	t.Run("GetNodes", func(t *testing.T) { testGetNodes(ctx, t, clt) })
	t.Run("ListNodes", func(t *testing.T) { testListNodes(ctx, t, clt) })
}

func testListNodes(ctx context.Context, t *testing.T, clt *Client) {
	// retrieve expected data set
	testNodes, err := testNodes()
	require.NoError(t, err)

	// GetNodes should retrieve nodes in pages
	resp, nextKey, err := clt.ListNodes(ctx, defaults.Namespace, 700, "")
	require.NoError(t, err)
	require.EqualValues(t, "700", nextKey)
	require.Empty(t, cmp.Diff(resp, testNodes[0:700],
		cmpopts.IgnoreFields(types.Metadata{}, "Labels"),
	))

	resp, nextKey, err = clt.ListNodes(ctx, defaults.Namespace, 301, nextKey)
	require.NoError(t, err)
	require.EqualValues(t, "", nextKey)
	require.Empty(t, cmp.Diff(resp, testNodes[700:1000],
		cmpopts.IgnoreFields(types.Metadata{}, "Labels"),
	))

	// ListNodes should fail on empty namespace
	_, _, err = clt.ListNodes(ctx, "", defaults.DefaultChunkSize, "")
	require.Error(t, err)
	require.IsType(t, trace.BadParameter(""), err)

	// ListNodes should default limit to 500
	resp, _, err = clt.ListNodes(ctx, defaults.Namespace, 0, "")
	require.NoError(t, err)
	require.True(t, len(resp) == defaults.DefaultChunkSize)
}

func testGetNodes(ctx context.Context, t *testing.T, clt *Client) {
	// retrieve expected data set
	testNodes, err := testNodes()
	require.NoError(t, err)

	// GetNodes should retrieve all nodes
	resp, err := clt.GetNodes(ctx, defaults.Namespace)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(testNodes, resp,
		cmpopts.IgnoreFields(types.Metadata{}, "Labels"),
	))

	// GetNodes should fail on empty namespace
	_, err = clt.GetNodes(ctx, "")
	require.Error(t, err)
	require.IsType(t, trace.BadParameter(""), err)
}
