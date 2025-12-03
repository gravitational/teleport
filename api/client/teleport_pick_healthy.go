/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package client

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"sync"
	"time"

	"google.golang.org/grpc/balancer"
	"google.golang.org/grpc/balancer/base"
	"google.golang.org/grpc/balancer/pickfirst/pickfirstleaf"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/resolver"
)

const (
	// Name is used by gRPC client service configuration to select this load balancer.
	Name = "teleport_pick_healthy"
)

func init() {
	balancer.Register(teleportPickHealthyBuilder{})
}

// teleportPickHealthyBuilder is used by gRPC clients to build a [teleportPickHealthyBalancer]
type teleportPickHealthyBuilder struct{}

// Build creates a [teleportPickHealthyBalancer] with an underlying pick_first_leaf balancer.
func (teleportPickHealthyBuilder) Build(cc balancer.ClientConn, opts balancer.BuildOptions) balancer.Balancer {
	log := slog.New(slog.NewJSONHandler(os.Stderr, nil))

	b := teleportPickHealthyBalancer{
		cc:   cc,
		opts: opts,
		log:  log,
	}

	b.current = newWrappedBalancer(&b)

	return &b
}

// Name returns the name of the load balancer that will be produced by this builder.
func (teleportPickHealthyBuilder) Name() string {
	return Name
}

// teleportPickHealthyBalancer balances client traffic using underlying pick_first_leaf balancers.
//
// teleportPickHealthyBalancer creates a pick_first_leaf balancer and enables health checking. When
// the balancer's state reports a transient failure (failing health check) then teleportPickHealthyBalancer will
// create a new pick_first_leaf balancer. Once the new pick_first_leaf balancer's [balancer.SubConn] becomes healthy
// then teleportPickHealthyBalancer will use the new balancer for new RPCs. It'll then close the previous pick_first_leaf
// balancer, which gracefully shuts down waiting for all current RPCs to complete before completely shutting down its
// subconnection.
//
// The pick_first_leaf load balancer iterates through resolved addresses/endpoints until it is able to successfully connect
// to one. The pick_first_leaf load balancer assumes that each address/endpoint ties to one server. This falls apart when
// the resolved address/endpoint is actually a load balancer. The pick_first_leaf will not try to establish a new connection
// to the same address/endpoint when health check fails because it assumes there is only 1 server for that address/endpoint and
// its unhealthy.
// This is where teleportPickHealthyBalancer comes into play. By creating a new pick_first_leaf load balancer, each resolved
// aaddress/endpoint is tried again. This provides the opportunity for the load balancer to forward the request to another server.
type teleportPickHealthyBalancer struct {
	// cc and opts are used for creating new pick_first_leaf load balancers
	// cc and opts are never written once configured in [teleportPickHealthyBuilder.Build]
	cc   balancer.ClientConn
	opts balancer.BuildOptions

	log *slog.Logger

	// mu lock should be held before accessing resolvedState or current/pending balancers
	mu sync.Mutex
	// resolvedState is used when creating new pick_first_leaf_balancers
	resolvedState resolver.State
	current       *wrappedBalancer
	pending       *wrappedBalancer
}

// Close closes the current and pending underlying pick_first_leaf balancers.
func (t *teleportPickHealthyBalancer) Close() {
	t.mu.Lock()

	current := t.current
	pending := t.pending

	t.current = nil
	t.pending = nil

	t.mu.Unlock()

	if current != nil {
		current.Close()
	}

	if pending != nil {
		pending.Close()
	}
}

// ExitIdle invokes ExitIdle on the most recently created load balancer.
func (t *teleportPickHealthyBalancer) ExitIdle() {
	bal := t.newestBalancer()

	if bal == nil {
		return
	}

	bal.ExitIdle()
}

// ResolverError invokes ResolverError for the newest load balancer.
//
// There is no point in invoking ResolverError for older load balancers
// since they've already completed resolving.
func (t *teleportPickHealthyBalancer) ResolverError(err error) {
	bal := t.newestBalancer()

	if bal == nil {
		t.cc.UpdateState(balancer.State{
			ConnectivityState: connectivity.TransientFailure,
			Picker:            base.NewErrPicker(err),
		})

		return
	}

	bal.ResolverError(err)
}

// UpdateClientConnState is responsible for enabling health checking for the underlying pick_first_leaf load balancers.
func (t *teleportPickHealthyBalancer) UpdateClientConnState(state balancer.ClientConnState) error {
	bal := t.newestBalancer()

	if bal == nil {
		return errors.New("balancer closed")
	}

	t.mu.Lock()
	t.resolvedState = state.ResolverState
	t.mu.Unlock()

	return bal.UpdateClientConnState(state)
}

// UpdateSubConnState forwards the state update to the corresponding balancer controlling the provided [balancer.SubConn].
func (t *teleportPickHealthyBalancer) UpdateSubConnState(sc balancer.SubConn, scs balancer.SubConnState) {
	t.mu.Lock()
	defer t.mu.Unlock()

	var bal *wrappedBalancer

	if t.current != nil && t.current.subConns[sc] {
		bal = t.current
	} else if t.pending != nil && t.pending.subConns[sc] {
		bal = t.pending
	}

	if bal == nil {
		return
	}

	t.log.InfoContext(context.Background(), "UpdateSubConnState invoked", slog.String("state", scs.ConnectivityState.String()))

	switch scs.ConnectivityState {
	case connectivity.Shutdown:
		delete(bal.subConns, sc)
	case connectivity.Ready:
		sc.RegisterHealthListener(func(state balancer.SubConnState) {
			t.healthListener(sc, state)
		})
	}

	// Do not invoke bal.UpdateSubConnState since the pick_first_leaf does not expect UpdateSubConnState to be invoked,
	// since it uses state listeners instead. Invoking will only emit error logs from the pick_first_leaf load balancer.
}

func (t *teleportPickHealthyBalancer) healthListener(sc balancer.SubConn, scs balancer.SubConnState) {
	t.mu.Lock()

	var bal *wrappedBalancer

	attrs := []any{
		slog.String("state", scs.ConnectivityState.String()),
	}

	if t.current != nil && t.current.subConns[sc] {
		bal = t.current
		attrs = append(attrs, slog.String("balancer", "current"))
	} else if t.pending != nil && t.pending.subConns[sc] {
		bal = t.pending
		attrs = append(attrs, slog.String("balancer", "pending"))
	} else {
		attrs = append(attrs, slog.String("balancer", "stale"))
	}

	t.log.InfoContext(context.Background(), "healthListener invoked", attrs...)

	if bal == nil {
		t.mu.Unlock()
		return
	}

	if bal == t.pending && scs.ConnectivityState == connectivity.TransientFailure {
		t.log.InfoContext(context.Background(), "Pending balancer is unhealthy, waiting before creating new balancer")

		t.mu.Unlock()

		// TODO(dustin.specker): use a backoff approach
		time.Sleep(1 * time.Second)

		t.mu.Lock()
	}

	switch bal {
	case t.current:
		switch scs.ConnectivityState {
		case connectivity.Ready:
			if t.pending != nil {
				t.log.InfoContext(context.Background(), "current balancer became healthy, closing pending balancer")

				t.pending.Close()

				t.pending = nil

				current := t.current
				resolvedState := t.resolvedState

				t.mu.Unlock()

				current.UpdateClientConnState(balancer.ClientConnState{
					ResolverState: resolvedState,
				})
			} else {
				t.mu.Unlock()
			}
		case connectivity.TransientFailure:
			t.log.InfoContext(context.Background(), "current balancer is unhealthy, creating new balancer")

			wb := newWrappedBalancer(t)

			t.pending = wb

			resolvedState := t.resolvedState

			t.mu.Unlock()

			// invoke UpdateClientConnState so that the new load balancer begins
			// creating a new subconnection
			wb.UpdateClientConnState(balancer.ClientConnState{
				ResolverState: resolvedState,
			})
		default:
			t.mu.Unlock()
		}
	case t.pending:
		switch scs.ConnectivityState {
		case connectivity.Ready:
			t.log.InfoContext(context.Background(), "pending balancer is ready, migrating to pending balancer")
			oldCurrent := t.current

			t.current = t.pending
			t.pending = nil

			t.mu.Unlock()

			// instruct the client connection to use this now ready subconnection
			t.cc.UpdateState(balancer.State{
				ConnectivityState: connectivity.Ready,
				Picker: &picker{
					sc: sc,
				},
			})

			oldCurrent.Close()
		case connectivity.TransientFailure:
			t.log.InfoContext(context.Background(), "pending balancer is unhealthy, creating a new one")

			t.pending.Close()

			wb := newWrappedBalancer(t)

			t.pending = wb

			resolvedState := t.resolvedState

			t.mu.Unlock()

			// invoke UpdateClientConnState so that the new load balancer begins
			// creating a new subconnection
			wb.UpdateClientConnState(balancer.ClientConnState{
				ResolverState: resolvedState,
			})
		default:
			t.mu.Unlock()
		}
	default:
		t.mu.Unlock()
	}
}

// newestBalancer returns the pending load balancer if existing, otherwise the current load balancer is returned.
func (t *teleportPickHealthyBalancer) newestBalancer() balancer.Balancer {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.pending != nil {
		return t.pending
	}

	return t.current
}

// wrappedBalancer is responsible for wrapping a [balancer.Balancer] and [balancer.SubConn]
type wrappedBalancer struct {
	balancer.ClientConn
	balancer.Balancer
	log *slog.Logger

	tlb *teleportPickHealthyBalancer

	subConns map[balancer.SubConn]bool
}

// newWrappedBalancer returns a wrapped pick_first_leaf balancer.
//
// It is up to the user to invoke UpdateClientConnState on the returned wrappedBalancer
// to cause the underlying pick_first_leaf balancer to create a new [balancer.SubConn].
func newWrappedBalancer(tphb *teleportPickHealthyBalancer) *wrappedBalancer {
	wb := wrappedBalancer{
		ClientConn: tphb.cc,
		log:        tphb.log,

		tlb:      tphb,
		subConns: make(map[balancer.SubConn]bool, 0),
	}

	pflb := balancer.Get(pickfirstleaf.Name).Build(&wb, tphb.opts)

	wb.Balancer = pflb

	return &wb
}

// Close will shutdown all registered subconnections.
//
// This is a graceful shutdown, so any active RPCs are waited on before shutting down the subconnection entirely.
func (t *wrappedBalancer) Close() {
	if t == nil {
		return
	}

	for sc := range t.subConns {
		sc.Shutdown()
	}
}

// NewSubConn registers created [balancer.SubConn] by the balancer, so that [teleportPickHealthyBalancer] can know which
// balancer created the subconnection for forwarding events to.
//
// This also registers a health listener so that [teleportPickHealthyBalancer] can know when the new subconnection has reached
// a ready state.
func (t *wrappedBalancer) NewSubConn(addrs []resolver.Address, opts balancer.NewSubConnOptions) (balancer.SubConn, error) {
	t.tlb.mu.Lock()

	if t != t.tlb.current && t != t.tlb.pending {
		t.tlb.mu.Unlock()
		return nil, errors.New("balancer that called NewSubConn is closed")
	}

	t.tlb.mu.Unlock()

	origListener := opts.StateListener

	var sc balancer.SubConn

	opts.StateListener = func(state balancer.SubConnState) {
		t.tlb.UpdateSubConnState(sc, state)

		if origListener != nil {
			origListener(state)
		}
	}

	//nolint:staticcheck // NewSubConn is the only way to currently create new sub connections.
	// The deprecation is noting that in the future providing multiples addresses will be deprecated.
	sc, err := t.tlb.cc.NewSubConn(addrs, opts)
	if err != nil {
		return nil, err
	}

	t.subConns[sc] = true

	return sc, nil
}

// ResolveNow only invokes ResolveNow if the balancer is the newest load balancer.
func (t *wrappedBalancer) ResolveNow(opts resolver.ResolveNowOptions) {
	if t != t.tlb.newestBalancer() {
		return
	}

	t.tlb.cc.ResolveNow(opts)
}

// RemoveSubConn shuts down the provided [balancer.SubConn]
func (t *wrappedBalancer) RemoveSubConn(sc balancer.SubConn) {
	sc.Shutdown()
}

// UpdateState handles creating new pick_first_leaf balancers in the case one becomes unhealthy.
func (t *wrappedBalancer) UpdateState(state balancer.State) {
	t.log.InfoContext(context.Background(), "UpdateState invoked", slog.String("state", state.ConnectivityState.String()))
	t.tlb.mu.Lock()

	// do not pass state changes for the pending load balancer because it's desired to only use
	// the new sub connections once the pending load balancer is ready and passing health checks
	if t != t.tlb.current {
		t.tlb.mu.Unlock()
		return
	}

	t.tlb.mu.Unlock()

	// always pass UpdateState to ClientConn if the current balancer experiences a state change
	t.tlb.cc.UpdateState(state)
}

// picker is used only to provide a [balancer.SubConn] for a client connection to use.
type picker struct {
	sc balancer.SubConn
}

// Pick returns the picker's [balancer.SubConn]
func (p *picker) Pick(balancer.PickInfo) (balancer.PickResult, error) {
	return balancer.PickResult{
		SubConn: p.sc,
	}, nil
}
