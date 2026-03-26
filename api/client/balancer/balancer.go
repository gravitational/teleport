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
	"math/rand"
	"time"

	"github.com/gravitational/trace"
	"google.golang.org/grpc/balancer"
	"google.golang.org/grpc/balancer/pickfirst"
	"google.golang.org/grpc/connectivity"

	"github.com/gravitational/teleport/api/client/balancer/internal"
)

func RegisterTeleportPickHealthyBalancer() {
	balancer.Register(balancerBuilder{})
}

const (
	// Name is the name used to register the customer [Balancer].
	Name = "teleport_pick_healthy"

	// reconnect deafults.
	reconnectBaseBackoff = 1 * time.Second
	reconnectMaxBackoff  = 16 * time.Second
)

var (
	testBuildHook func(*Balancer)
)

// balancerBuilder implements [balancer.Builder] for building [Balancer].
type balancerBuilder struct{}

func (balancerBuilder) Name() string {
	return Name
}

func (balancerBuilder) Build(cc balancer.ClientConn, opts balancer.BuildOptions) balancer.Balancer {
	ctx, cancel := context.WithCancel(context.Background())
	b := &Balancer{
		cc:          cc,
		opts:        opts,
		log:         slog.Default(),
		ctx:         ctx,
		cancel:      cancel,
		serializer:  internal.NewCallbackSerializer(),
		backoff:     reconnectBaseBackoff,
		baseBackoff: reconnectBaseBackoff,
		maxBackoff:  reconnectMaxBackoff,
	}
	b.active = b.newManager()
	if testBuildHook != nil {
		testBuildHook(b)
	}
	return b
}

// Balancer is a customer [balancer.Balancer] implementation registered under
// [Name] = "teleport_pick_healthy". By default this balancer is designed to
// behave nearly identical to the default grpc "pick_first" balancer.
//
// The difference is it makes a RPC call when a new [balancer.SubConn] becomes
// ready to discover configuration from the connected server. This allows server
// configuration to change the [Balancer] behavior.
//
// When the server sets "reconnect" mode the balancer will begin using the grpc
// health check API to monitor a servers health. When a server becomes unhealthy
// the [Balancer] will begin attempts to connect to another server.
type Balancer struct {
	cc     balancer.ClientConn
	opts   balancer.BuildOptions
	log    *slog.Logger
	ctx    context.Context
	cancel func()

	// serializer ensures synchronization and ordering of balancer operations.
	// This was prefered over a mutex to avoid potential issues with deadlocks
	// and out-of-order events. The main culprit of this is the [balancer.UpdateClientConnState]
	// which can trigger events which call back into this the [Balancer].
	serializer *internal.CallbackSerializer

	// All the fields below must be access within the serializer.
	active       *internal.BalancerManager
	backup       *internal.BalancerManager
	state        balancer.ClientConnState
	lastCCUpdate balancer.State

	// reconnect is the timer returned from the [time.AfterFunc] used to fire
	// off a reconnect. A non nil value here indicates a reconnect is in flight.
	reconnect *time.Timer
	// backoff is the amount of time to wait before a reconnect runs. This uses
	// an exponential backoff with 50% jitter and resets when a balancer becomes
	// ready.
	backoff     time.Duration
	baseBackoff time.Duration
	maxBackoff  time.Duration
}

// UpdateClientConnState receives [balancer.ClientConnState] updates from a
// [grpc.Resolver].
func (b *Balancer) UpdateClientConnState(state balancer.ClientConnState) error {
	errc := make(chan error)
	ok := b.serializer.Put(func() {
		state.BalancerConfig = nil
		state.ResolverState.ServiceConfig = nil
		b.state = state
		errc <- b.active.GetBalancer().UpdateClientConnState(b.state)
		close(errc)
		if b.backup != nil {
			b.backup.GetBalancer().UpdateClientConnState(b.state)
		}
	})
	if !ok {
		close(errc)
		return trace.Errorf("failed to update state: balancer is closed")
	}
	return <-errc
}

// UpdateSubConnState is deprecated but still required to implement the
// [balancer.Balancer] interface.
func (b *Balancer) UpdateSubConnState(subConn balancer.SubConn, state balancer.SubConnState) {
	b.log.Warn("UpdateSubConnState called unexpectedly", "subconn", subConn, "state", state)
}

// ResolverError receives errors from a [grpc.Resolver].
func (b *Balancer) ResolverError(err error) {
	b.serializer.Put(func() {
		b.active.GetBalancer().ResolverError(err)
		if b.backup != nil {
			b.backup.GetBalancer().ResolverError(err)
		}
	})
}

// ExitIdle calls ExitIdle on all underlying balancers.
func (b *Balancer) ExitIdle() {
	b.serializer.Put(func() {
		b.active.GetBalancer().ExitIdle()
		if b.backup != nil {
			b.backup.GetBalancer().ExitIdle()
		}
	})
}

// Close closes the [Balancer] instance.
func (b *Balancer) Close() {
	b.serializer.CloseFn(func() {
		b.resetReconnectSerial()
		b.active.Close()
		if b.backup != nil {
			b.backup.Close()
		}
		b.active = nil
		b.backup = nil
	})
}

func (b *Balancer) newManager() *internal.BalancerManager {
	var bm *internal.BalancerManager
	wrapped := &clientConnWrapper{
		ClientConn: b.cc,
		notify: func(sc balancer.SubConn, state balancer.State) {
			ok := b.serializer.Put(func() {
				bm.Update(sc, state)
				b.updateHandlerSerial(bm)
			})
			if !ok {
				b.log.Warn("received subconn event after close")
			}

		},
	}
	innerBalancer := balancer.Get(pickfirst.Name).Build(wrapped, b.opts)
	bm = internal.NewBalancerManager(b.ctx, innerBalancer, func() {
		ok := b.serializer.Put(func() { b.updateHandlerSerial(bm) })
		if !ok {
			b.log.Warn("received balancer event after close")
		}
	}, b.log)
	return bm
}

// updateHandlerSerial manages all state updates. This must be called from within
// the serializer.
func (b *Balancer) updateHandlerSerial(bm *internal.BalancerManager) {
	state := bm.State()
	b.log.Debug("receive state update for balancer",
		"balancer", b.balancer(bm),
		"state", state.Conn.String(),
		"health", state.Health.String(),
		"reconnect", state.Reconnect,
		"health_checking", state.HealthChecking,
	)

	// Ignore the state update if it is not from one of the tracked balancers.
	// It should already be closed.
	if bm != b.active && bm != b.backup {
		return
	}

	b.serializer.Put(func() {
		ccUpdate := b.active.State().BalancerState()
		if ccUpdate == b.lastCCUpdate {
			return
		}
		b.cc.UpdateState(ccUpdate)
		b.lastCCUpdate = ccUpdate
	})

	active := b.active.State()
	if active.Ready() || !active.Reconnect {
		b.resetReconnectSerial()
		if b.backup != nil {
			bm := b.backup
			state := bm.State()
			b.backup = nil
			b.log.Debug("closing previous backup balancer",
				"balancer", b.balancer(bm),
				"state", state.Conn.String(),
				"health", state.Health.String(),
				"reconnect", state.Reconnect,
				"health_checking", state.HealthChecking,
			)
			bm.Close()
		}
		return
	}

	if active.Connecting() {
		return
	}
	if b.backup == nil {
		b.scheduleReconnectSerial()
		return
	}

	backup := b.backup.State()
	if backup.Ready() {
		bm := b.active
		b.active = b.backup
		b.backup = nil
		b.log.Debug("closing previous active balancer",
			"balancer", b.balancer(bm),
			"state", active.Conn.String(),
			"health", active.Health.String(),
			"reconnect", state.Reconnect,
			"health_checking", state.HealthChecking,
		)
		bm.Close()
		return
	}

	if backup.Connecting() {
		return
	}

	if backup.Unhealthy() {
		b.backoff = b.backoff * 2
		if b.backoff > b.maxBackoff {
			b.backoff = b.maxBackoff
		}
		b.scheduleReconnectSerial()
		return
	}
}

func (b *Balancer) resetReconnectSerial() {
	if b.reconnect == nil {
		return
	}
	b.reconnect.Stop()
	b.reconnect = nil
	b.backoff = b.baseBackoff
}

// scheduleReconnectSerial kicks off a reconnect at some point in the future.
// This must be called within the serializer.
func (b *Balancer) scheduleReconnectSerial() {
	if b.reconnect != nil {
		return
	}

	delay := b.backoff + time.Duration(float32(b.backoff/2)*rand.Float32())
	var reconnect *time.Timer
	reconnect = time.AfterFunc(delay, func() {
		b.serializer.Put(func() {
			// Check against the local reconnect to ensure this instances is
			// expected to run. This protects against the condition where the
			// reconnect was stopped but the timer already put this function
			// in the serializer queue.
			if b.reconnect != reconnect {
				return
			}
			b.reconnectSerial()
			b.reconnect = nil
		})
	})
	b.reconnect = reconnect
}

// reconnectSerial creates a new failover balancer. This must be called within
// the serializer.
func (b *Balancer) reconnectSerial() {
	if b.active.State().Ready() {
		return
	}
	if b.backup != nil {
		state := b.backup.State()
		if !state.Unhealthy() {
			return
		}
		bm := b.backup
		b.backup = nil
		b.log.Debug("closing previous backup balancer",
			"balancer", b.balancer(bm),
			"state", state.Conn.String(),
			"health", state.Health.String(),
			"reconnect", state.Reconnect,
			"health_checking", state.HealthChecking,
		)
		bm.Close()
	}
	b.backup = b.newManager()
	// Shuffle addresses on reconnect to avoid chances of reconnecting to the
	// same unhealthy server everytime. This is only an issue when there is no
	// shuffling at the resolver, the address routes to a single server, a
	// connection is able to be established to the server, and the server is
	// unhealthy.
	rand.Shuffle(len(b.state.ResolverState.Endpoints), func(i, j int) {
		b.state.ResolverState.Endpoints[i], b.state.ResolverState.Endpoints[j] =
			b.state.ResolverState.Endpoints[j], b.state.ResolverState.Endpoints[i]
	})
	rand.Shuffle(len(b.state.ResolverState.Addresses), func(i, j int) {
		b.state.ResolverState.Addresses[i], b.state.ResolverState.Addresses[j] =
			b.state.ResolverState.Addresses[j], b.state.ResolverState.Addresses[i]
	})
	b.backup.GetBalancer().UpdateClientConnState(b.state)
}

func (b *Balancer) balancer(bm *internal.BalancerManager) string {
	if b.active == bm {
		return "active"
	}
	if b.backup == bm {
		return "backup"
	}
	return "untracked"
}

// clientConnWrapper intercepts [balancer.ClientConn.UpdateState] ands sends
// them to the notify function.
type clientConnWrapper struct {
	balancer.ClientConn
	notify func(balancer.SubConn, balancer.State)
}

// UpdateState intercepts [balancer.ClientConn.UpdateState] calls.
func (w *clientConnWrapper) UpdateState(state balancer.State) {
	var sc balancer.SubConn
	if state.ConnectivityState == connectivity.Ready && state.Picker != nil {
		result, err := state.Picker.Pick(balancer.PickInfo{})
		if err == nil {
			sc = result.SubConn
		}
	}
	w.notify(sc, state)
}
