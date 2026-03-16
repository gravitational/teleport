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

package internal

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/balancer"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/resolver"

	grpcv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/grpcclientconfig/v1"
	"github.com/gravitational/trace"
)

func TestBalancerManagerUpdates(t *testing.T) {
	t.Parallel()

	type event struct {
		kind      string
		connState connectivity.State
	}

	type subConnSpec struct {
		invokeErr      error
		mode           grpcv1.Mode
		healthChecking bool
	}

	type testCase struct {
		name             string
		initialReconnect bool
		subConn          subConnSpec
		events           []event
		wantReconnect    bool
		wantHealthCheck  bool
		wantConn         connectivity.State
		wantHealth       connectivity.State
		wantInvokes      int32
	}

	tests := []testCase{
		{
			name:    "balancer is ready and healthy",
			subConn: subConnSpec{mode: grpcv1.Mode_MODE_RECONNECT, healthChecking: true},
			events: []event{
				{kind: "update", connState: connectivity.Ready},
				{kind: "health", connState: connectivity.Ready},
			},
			wantReconnect:   true,
			wantHealthCheck: true,
			wantConn:        connectivity.Ready,
			wantHealth:      connectivity.Ready,
			wantInvokes:     1,
		},
		{
			name:             "balancer is ready and healthy with config failure",
			initialReconnect: true,
			subConn:          subConnSpec{invokeErr: errors.New("config fetch failed")},
			events: []event{
				{kind: "update", connState: connectivity.Ready},
			},
			wantReconnect:   true,
			wantHealthCheck: false,
			wantConn:        connectivity.Ready,
			wantHealth:      connectivity.Ready,
			wantInvokes:     1,
		},
		{
			name:    "balancer is ready and not healthy",
			subConn: subConnSpec{mode: grpcv1.Mode_MODE_RECONNECT, healthChecking: true},
			events: []event{
				{kind: "update", connState: connectivity.Ready},
				{kind: "health", connState: connectivity.TransientFailure},
			},
			wantReconnect:   true,
			wantHealthCheck: true,
			wantConn:        connectivity.Ready,
			wantHealth:      connectivity.TransientFailure,
			wantInvokes:     1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := t.Context()
			fb := &fakeBalancer{}
			callbacks := make(chan struct{}, 8)

			invokeCount := &atomic.Int32{}

			empty := ""
			sc := &fakeSubConn{
				invokeFn: func(ctx context.Context, _ string, _ any, reply any, _ ...grpc.CallOption) error {
					_ = ctx
					invokeCount.Add(1)
					if tt.subConn.invokeErr != nil {
						return tt.subConn.invokeErr
					}
					resp := reply.(*grpcv1.GetServiceConfigResponse)
					resp.Config = &grpcv1.ServiceConfig{
						LoadBalancingConfig: []*grpcv1.LoadBalancerConfig{{
							Config: &grpcv1.LoadBalancerConfig_TeleportPickHealthy{
								TeleportPickHealthy: &grpcv1.TeleportPickHealthyConfig{Mode: tt.subConn.mode},
							},
						}},
					}
					if tt.subConn.healthChecking {
						resp.Config.HealthCheckConfig = &grpcv1.HealthCheckConfig{ServiceName: &empty}
					}
					return nil
				},
			}

			bm := NewBalancerManager(ctx, fb, func() {
				callbacks <- struct{}{}
			}, nil)

			bm.state.Reconnect = tt.initialReconnect

			for _, ev := range tt.events {
				switch ev.kind {
				case "update":
					bm.Update(sc, balancer.State{ConnectivityState: ev.connState})
				case "health":
					sc.emitHealth(ev.connState)
				default:
					t.Fatalf("unknown event kind %q", ev.kind)
				}
				select {
				case <-callbacks:
				case <-time.After(5 * time.Second):
					t.Fatal("timeout waiting for balancer callback")
				}
			}

			st := bm.State()
			require.Equal(t, tt.wantReconnect, st.Reconnect)
			require.Equal(t, tt.wantHealthCheck, st.HealthChecking)
			require.Equal(t, tt.wantConn, st.Conn)
			require.Equal(t, tt.wantHealth, st.Health)
			require.Equal(t, tt.wantInvokes, invokeCount.Load())
			bm.Close()
			require.Equal(t, int32(1), fb.closeCount.Load())
		})
	}
}

type fakeBalancer struct {
	closeCount atomic.Int32
}

func (f *fakeBalancer) UpdateClientConnState(balancer.ClientConnState) error {
	return nil
}

func (f *fakeBalancer) ResolverError(error) {}

func (f *fakeBalancer) UpdateSubConnState(balancer.SubConn, balancer.SubConnState) {}

func (f *fakeBalancer) Close() {
	f.closeCount.Add(1)
}
func (f *fakeBalancer) ExitIdle() {}

type fakeSubConn struct {
	balancer.SubConn

	mu             sync.Mutex
	healthListener func(balancer.SubConnState)
	invokeFn       func(ctx context.Context, method string, args any, reply any, opts ...grpc.CallOption) error
}

func (f *fakeSubConn) Invoke(ctx context.Context, method string, args any, reply any, opts ...grpc.CallOption) error {
	if f.invokeFn == nil {
		return nil
	}
	return f.invokeFn(ctx, method, args, reply, opts...)
}

func (f *fakeSubConn) NewStream(context.Context, *grpc.StreamDesc, string, ...grpc.CallOption) (grpc.ClientStream, error) {
	return nil, trace.NotImplemented("new stream not implemented")
}

func (f *fakeSubConn) RegisterHealthListener(fn func(balancer.SubConnState)) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.healthListener = fn
}

func (f *fakeSubConn) emitHealth(state connectivity.State) {
	f.mu.Lock()
	fn := f.healthListener
	f.mu.Unlock()
	if fn != nil {
		fn(balancer.SubConnState{ConnectivityState: state})
	}
}

func (f *fakeSubConn) UpdateAddresses([]resolver.Address) {}
func (f *fakeSubConn) Connect()                           {}
func (f *fakeSubConn) GetOrBuildProducer(balancer.ProducerBuilder) (balancer.Producer, func()) {
	return nil, func() {}
}
func (f *fakeSubConn) Shutdown() {}
