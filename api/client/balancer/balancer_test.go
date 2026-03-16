// Copyright 2025 Gravitational, Inc
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

package balancer

import (
	"context"
	"log/slog"
	"net"
	"os"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/balancer"
	"google.golang.org/grpc/credentials/insecure"
	healthgrpc "google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/resolver"
	"google.golang.org/grpc/resolver/manual"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/wrapperspb"

	"github.com/gravitational/teleport/api/client/balancer/internal"
	grpcv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/grpcclientconfig/v1"
)

func init() {
	RegisterTeleportPickHealthyBalancer()
}

func TestPickFirstModeHealthAndReconnect(t *testing.T) {
	ctx := t.Context()
	server1 := newTestServer(t, grpcv1.Mode_MODE_PICK_FIRST, false)
	server2 := newTestServer(t, grpcv1.Mode_MODE_PICK_FIRST, false)
	resolver := newTestResolver(t)
	conn := newTestClient(t, resolver)

	resolver.UpdateAddresses(server1.Addr(), server2.Addr())
	conn.Connect()

	var prev, next *testServer
	require.Eventually(t, func() bool {
		name, err := pingServer(ctx, conn)
		if err != nil {
			return false
		}
		if name == server1.Name() {
			prev = server1
			next = server2
			return true
		}
		if name == server2.Name() {
			prev = server2
			next = server1
			return true
		}
		return false
	}, 5*time.Second, 50*time.Millisecond)

	// In pick_first mode the balancer should remain connected to the unhealthy
	// server.
	prev.SetUnhealthy()
	require.Never(t, func() bool {
		name, err := pingServer(ctx, conn)
		return err == nil && name == next.Name()
	}, 2*time.Second, 50*time.Millisecond)

	prev.Stop()
	require.Eventually(t, func() bool {
		name, err := pingServer(ctx, conn)
		return err == nil && name == next.Name()
	}, 5*time.Second, 50*time.Millisecond)
}

func TestReconnectModeHealthAndReconnect(t *testing.T) {
	ctx := t.Context()
	server1 := newTestServer(t, grpcv1.Mode_MODE_RECONNECT, true)
	server2 := newTestServer(t, grpcv1.Mode_MODE_RECONNECT, true)
	resolver := newTestResolver(t)
	conn := newTestClient(t, resolver)

	resolver.UpdateAddresses(server1.Addr(), server2.Addr())
	conn.Connect()

	var prev, next *testServer
	require.Eventually(t, func() bool {
		name, err := pingServer(ctx, conn)
		if err != nil {
			return false
		}
		if name == server1.Name() {
			prev = server1
			next = server2
			return true
		}
		if name == server2.Name() {
			prev = server2
			next = server1
			return true
		}
		return false
	}, 5*time.Second, 50*time.Millisecond)

	prev.SetUnhealthy()
	require.Eventually(t, func() bool {
		name, err := pingServer(ctx, conn)
		return err == nil && name == next.Name()
	}, 10*time.Second, 50*time.Millisecond)
	next.Stop()

	// Ensure rpcs can go through to an unhealthy server when it is the only
	// server available.
	require.Eventually(t, func() bool {
		name, err := pingServer(ctx, conn)
		return err == nil && name == prev.Name()
	}, 5*time.Second, 50*time.Millisecond)
}

func TestBalancerModeChange(t *testing.T) {
	ctx := t.Context()
	server1 := newTestServer(t, grpcv1.Mode_MODE_PICK_FIRST, false)
	server2 := newTestServer(t, grpcv1.Mode_MODE_RECONNECT, true)
	resolver := newTestResolver(t)
	resolver.UpdateAddresses(server1.Addr())

	balancerCh := make(chan *Balancer, 1)
	withBuildHook(t, func(b *Balancer) {
		balancerCh <- b
	})

	conn := newTestClient(t, resolver)
	name, err := pingServer(ctx, conn)
	require.NoError(t, err)
	require.Equal(t, server1.Name(), name)

	b := <-balancerCh
	getState := func() internal.State {
		var state internal.State
		wg := &sync.WaitGroup{}
		wg.Add(1)
		b.serializer.Put(func() {
			state = b.active.State()
			wg.Done()
		})
		wg.Wait()
		return state
	}

	require.Eventually(t, func() bool {
		state := getState()
		return state.Ready() && !state.Reconnect && !state.HealthChecking
	}, 5*time.Second, 10*time.Millisecond)

	server1.Stop()

	resolver.UpdateAddresses(server2.Addr())
	name, err = pingServer(ctx, conn)
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		state := getState()
		return state.Ready() && state.Reconnect && state.HealthChecking
	}, 5*time.Second, 10*time.Millisecond)
}

func TestBalancerClose(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		server := newTestServer(t, grpcv1.Mode_MODE_RECONNECT, false)
		resolver := newTestResolver(t)
		resolver.UpdateAddresses(server.Addr())
		balancerCh := make(chan *Balancer, 1)
		withBuildHook(t, func(b *Balancer) {
			balancerCh <- b
		})

		conn := newTestClient(t, resolver)
		_, err := pingServer(t.Context(), conn)
		require.NoError(t, err)

		b := <-balancerCh
		require.NoError(t, conn.Close())
		err = b.UpdateClientConnState(balancer.ClientConnState{})
		require.Error(t, err)
	})
}

// buildHookMu should only be used within [withBuildHook]
var buildHookMu sync.Mutex

// withBuildHook injects a hook into the balancer build process so the balancer
// can be retrieved. This is protected by a mutex to avoid concurrent tests from
// fighting with each other.
func withBuildHook(t *testing.T, innerHook func(*Balancer)) {
	once := &sync.Once{}
	buildHookMu.Lock()
	prev := testBuildHook
	testBuildHook = func(b *Balancer) {
		b.log = slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			Level:     slog.LevelDebug,
			AddSource: true,
		}))
		b.baseBackoff = time.Millisecond * 100
		b.maxBackoff = time.Millisecond * 100
		innerHook(b)

		testBuildHook = prev
		once.Do(func() {
			testBuildHook = prev
			buildHookMu.Unlock()
		})
	}
	t.Cleanup(func() {
		once.Do(func() {
			testBuildHook = prev
			buildHookMu.Unlock()
		})
	})
}

type configService struct {
	grpcv1.UnimplementedServiceConfigDiscoveryServiceServer
	mode           grpcv1.Mode
	healthChecking bool
}

func (s *configService) GetServiceConfig(context.Context, *grpcv1.GetServiceConfigRequest) (*grpcv1.GetServiceConfigResponse, error) {
	resp := &grpcv1.GetServiceConfigResponse{
		Config: &grpcv1.ServiceConfig{
			LoadBalancingConfig: []*grpcv1.LoadBalancerConfig{{
				Config: &grpcv1.LoadBalancerConfig_TeleportPickHealthy{
					TeleportPickHealthy: &grpcv1.TeleportPickHealthyConfig{
						Mode: s.mode,
					},
				},
			}},
		},
	}
	if s.healthChecking {
		empty := ""
		resp.Config.HealthCheckConfig = &grpcv1.HealthCheckConfig{ServiceName: &empty}
	}
	return resp, nil
}

type testResolver struct {
	*manual.Resolver
}

func newTestResolver(_ *testing.T) *testResolver {
	return &testResolver{Resolver: manual.NewBuilderWithScheme("teleport-test")}
}

func (r *testResolver) Target() string {
	return r.Scheme() + ":///teleport"
}

func (r *testResolver) UpdateAddresses(addrs ...string) {
	addresses := make([]resolver.Address, 0, len(addrs))
	for _, addr := range addrs {
		addresses = append(addresses, resolver.Address{Addr: addr})
	}
	r.UpdateState(resolver.State{Addresses: addresses})
}

type testServer struct {
	t        *testing.T
	listener net.Listener
	server   *grpc.Server
	config   *configService
	health   *healthgrpc.Server
	name     string
}

func newTestServer(t *testing.T, mode grpcv1.Mode, healthChecking bool) *testServer {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen: %v", err)
	}
	name := l.Addr().String()

	config := &configService{
		mode:           mode,
		healthChecking: healthChecking,
	}
	healthServer := healthgrpc.NewServer()
	healthServer.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)

	server := grpc.NewServer()
	grpcv1.RegisterServiceConfigDiscoveryServiceServer(server, config)
	healthpb.RegisterHealthServer(server, healthServer)
	registerTestPingServer(server, &testPingService{
		name: name,
	})

	go func() {
		_ = server.Serve(l)
	}()

	srv := &testServer{
		t:        t,
		listener: l,
		server:   server,
		config:   config,
		health:   healthServer,
		name:     name,
	}
	t.Cleanup(srv.Stop)
	return srv
}

func (s *testServer) Addr() string {
	return s.listener.Addr().String()
}

func (s *testServer) Name() string {
	return s.name
}

func (s *testServer) Stop() {
	if s.server != nil {
		s.server.Stop()
		s.server = nil
	}
	if s.listener != nil {
		_ = s.listener.Close()
		s.listener = nil
	}
}

func (s *testServer) SetHealthy() {
	s.health.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)
}

func (s *testServer) SetUnhealthy() {
	s.health.SetServingStatus("", healthpb.HealthCheckResponse_NOT_SERVING)
}

func newTestClient(t *testing.T, r *testResolver) *grpc.ClientConn {
	conn, err := grpc.NewClient(
		r.Target(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithResolvers(r),
		grpc.WithDefaultServiceConfig(`{"loadBalancingConfig":[{"teleport_pick_healthy":{}}],"healthCheckConfig":{"serviceName":""}}`),
	)
	require.NoError(t, err)
	t.Cleanup(func() { conn.Close() })
	return conn
}

func pingServer(ctx context.Context, conn *grpc.ClientConn) (string, error) {
	callCtx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
	defer cancel()
	resp := &wrapperspb.StringValue{}
	err := conn.Invoke(callCtx, testPingMethod, &emptypb.Empty{}, resp)
	if err != nil {
		return "", err
	}
	return resp.Value, nil
}

const testPingMethod = "/teleport.testing.balancer.TestService/Ping"

type testPingServiceServer interface {
	Ping(context.Context, *emptypb.Empty) (*wrapperspb.StringValue, error)
}

type testPingService struct {
	name string
}

func (s *testPingService) Ping(context.Context, *emptypb.Empty) (*wrapperspb.StringValue, error) {
	return wrapperspb.String(s.name), nil
}

func registerTestPingServer(server grpc.ServiceRegistrar, svc *testPingService) {
	server.RegisterService(&grpc.ServiceDesc{
		ServiceName: "teleport.testing.balancer.TestService",
		HandlerType: (*testPingServiceServer)(nil),
		Methods: []grpc.MethodDesc{{
			MethodName: "Ping",
			Handler: func(
				srv any,
				ctx context.Context,
				_ func(any) error,
				_ grpc.UnaryServerInterceptor,
			) (any, error) {
				req := &emptypb.Empty{}
				return srv.(*testPingService).Ping(ctx, req)
			},
		}},
	}, svc)
}
