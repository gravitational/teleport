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
	"log/slog"
	"sync"

	"google.golang.org/grpc"
	"google.golang.org/grpc/balancer"
	"google.golang.org/grpc/connectivity"

	grpcv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/grpcclientconfig/v1"
	"github.com/gravitational/teleport/api/types/grpcclientconfig"
)

func noop() {}

// NewBalancerManager constructs a new [BalancerManager]. The callback is called
// when subconn initialization completes or health updates are received.
func NewBalancerManager(ctx context.Context, b balancer.Balancer, callback func(), log *slog.Logger) *BalancerManager {
	bm := &BalancerManager{
		ctx: ctx,
		b:   b,
		state: State{
			Conn:   connectivity.Connecting,
			Health: connectivity.Connecting,
		},
		initCancel: noop,
		callback:   callback,
		log:        log,
	}
	return bm
}

// BalancerManager manages the state of a [pickfirst.Balancer].
type BalancerManager struct {
	b   balancer.Balancer
	ctx context.Context

	mu         sync.Mutex
	state      State
	sc         balancer.SubConn
	initCancel func()
	closed     bool

	callback func()
	log      *slog.Logger
}

// GetBalancer returns the underlying [pickfirst.Balancer].
func (m *BalancerManager) GetBalancer() balancer.Balancer {
	return m.b
}

// Update handles state updates and optional SubConn selection from the picker.
func (m *BalancerManager) Update(sc balancer.SubConn, state balancer.State) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return
	}
	m.state.picker = state.Picker
	m.state.Conn = state.ConnectivityState

	if m.state.Conn == connectivity.Ready && sc != nil {
		m.sc = sc
		m.initLocked()
	} else {
		m.initCancel()
		m.initCancel = noop
		m.state.Health = connectivity.Connecting
		m.sc = nil
	}
}

// Close ensures all manages resources are cleaned up.
func (m *BalancerManager) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.b.Close()
	m.closed = true
	m.initCancel()
}

// State returns the [State] of the balancer.
func (m *BalancerManager) State() State {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.state
}

// initLocked starts subconn intialization in the background ensuring that
// previous initialization runs are cancelled. Initialization fetches the
// service config over the subconn, registers a health listener, and calls
// the [BalancerManager.callback].
func (m *BalancerManager) initLocked() {
	m.initCancel()
	ctx, cancel := context.WithCancel(m.ctx)
	var cancelled bool
	m.initCancel = func() {
		cancelled = true
		cancel()
	}
	sc := m.sc
	cc := m.sc.(grpc.ClientConnInterface)

	go func() {
		resp := &grpcv1.GetServiceConfigResponse{}
		configErr := cc.Invoke(ctx, grpcv1.ServiceConfigDiscoveryService_GetServiceConfig_FullMethodName, &grpcv1.GetServiceConfigRequest{}, resp)
		if configErr != nil {
			slog.DebugContext(ctx, "Failed to fetch service config", "error", configErr)
		}

		m.mu.Lock()
		defer m.mu.Unlock()
		if cancelled {
			return
		}
		if m.sc != sc {
			return
		}

		defer m.notify()
		// Skip health checking when the config fetch fails. Either the subconn
		// is closed or we're connected to an old server.
		if configErr != nil {
			m.state.Health = connectivity.Ready
			return
		}

		config := resp.GetConfig()
		tph := grpcclientconfig.TeleportPickHealthy(config)
		m.state.Reconnect = tph.GetMode() == grpcv1.Mode_MODE_RECONNECT

		m.state.HealthChecking = grpcclientconfig.HealthCheckingEnabled(config)
		if m.state.HealthChecking {
			m.state.Health = connectivity.Connecting
			sc.RegisterHealthListener(func(scs balancer.SubConnState) {
				m.healthListener(sc, scs)
			})
		} else {
			m.state.Health = connectivity.Ready
		}
	}()
}

func (m *BalancerManager) healthListener(sc balancer.SubConn, state balancer.SubConnState) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.sc != sc {
		return
	}
	m.state.Health = state.ConnectivityState
	m.notify()
}

func (m *BalancerManager) notify() {
	if m.callback == nil || m.closed {
		return
	}
	m.callback()
}
