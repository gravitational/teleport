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
	"testing"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/trace/trail"

	"github.com/gravitational/trace"
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
func startMockServer(t *testing.T, addr string) {
	l, err := net.Listen("tcp", addr)
	require.NoError(t, err)
	go newMockServer().grpc.Serve(l)
	t.Cleanup(func() { require.NoError(t, l.Close()) })
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

const fiveMBNode = "fiveMBNode"

func testNodes(namespace string) ([]types.Server, error) {
	switch namespace {
	case fiveMBNode:
		node := &types.ServerV2{
			Version: types.V2,
			Kind:    types.KindNode,
			Metadata: types.Metadata{
				Name:      "node1",
				Namespace: defaults.Namespace,
				Labels: map[string]string{
					// Artificially make a node ~ 5MB to force
					// ListNodes to fail regardless of chunk size.
					"label": string(make([]byte, 5000000)),
				},
			},
		}
		return []types.Server{node}, nil
	default:
		nodes := make([]types.Server, 50)
		for i := 0; i < 50; i++ {
			nodes[i] = &types.ServerV2{
				Version: types.V2,
				Kind:    types.KindNode,
				Metadata: types.Metadata{
					Name:      "node1",
					Namespace: defaults.Namespace,
					Labels: map[string]string{
						// Artificially make each node ~ 100KB to force
						// ListNodes to fail with chunks of >= 40.
						"label": string(make([]byte, 100000)),
					},
				},
			}
		}
		return nodes, nil
	}
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

func TestLimitExceeded(t *testing.T) {
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
