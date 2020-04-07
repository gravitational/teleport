/*
Copyright 2020 Gravitational, Inc.

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

package workpool

import (
	"context"
	"sync"

	"go.uber.org/atomic"
)

// Pool manages a collection of work groups by key and is the primary means
// by which groups are managed.  Each work group has an adjustable target value
// which is the number of target leases which should be active for the given
// group.
type Pool struct {
	mu       sync.Mutex
	leaseIDs *atomic.Uint64
	groups   map[interface{}]*group
	grantC   chan Lease
	ctx      context.Context
	cancel   context.CancelFunc
}

func NewPool(ctx context.Context) *Pool {
	ctx, cancel := context.WithCancel(ctx)
	return &Pool{
		leaseIDs: atomic.NewUint64(0),
		groups:   make(map[interface{}]*group),
		grantC:   make(chan Lease),
		ctx:      ctx,
		cancel:   cancel,
	}
}

// Acquire is the channel which must be received on to acquire
// new leases.  Each lease acquired in this way *must* have its
// Release method called when the lease is no longer needed.
func (p *Pool) Acquire() <-chan Lease {
	return p.grantC
}

// Done signals pool closure.
func (p *Pool) Done() <-chan struct{} {
	return p.ctx.Done()
}

// Get gets the current counts for the specified key.
func (p *Pool) Get(key interface{}) Counts {
	p.mu.Lock()
	defer p.mu.Unlock()
	if g, ok := p.groups[key]; ok {
		return g.loadCounts()
	}
	return Counts{}
}

// Set sets the target for the specified key.
func (p *Pool) Set(key interface{}, target uint64) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if target < 1 {
		p.del(key)
		return
	}
	g, ok := p.groups[key]
	if !ok {
		p.start(key, target)
		return
	}
	g.setTarget(target)
}

// Start starts a new work group with the specified initial target.
// If Start returns false, the group already exists.
func (p *Pool) start(key interface{}, target uint64) {
	ctx, cancel := context.WithCancel(p.ctx)
	notifyC := make(chan struct{}, 1)
	g := &group{
		counts: Counts{
			Active: 0,
			Target: target,
		},
		leaseIDs: p.leaseIDs,
		key:      key,
		grantC:   p.grantC,
		notifyC:  notifyC,
		ctx:      ctx,
		cancel:   cancel,
	}
	p.groups[key] = g
	go g.run()
}

func (p *Pool) del(key interface{}) (ok bool) {
	group, ok := p.groups[key]
	if !ok {
		return false
	}
	group.cancel()
	delete(p.groups, key)
	return true
}

// Stop permanently halts all associated groups.
func (p *Pool) Stop() {
	p.cancel()
}

// Counts holds the target and active counts for a
// key/group.
type Counts struct {
	// Target is the number of active leases that we would
	// like to converge toward.
	Target uint64
	// Active is the current active lease count.
	Active uint64
}

// group is a work group for a particular key in the pool. It tracks the number of
// active and target leases and adds leases when active drops below target.
type group struct {
	cmu      sync.Mutex
	counts   Counts
	leaseIDs *atomic.Uint64
	key      interface{}
	grantC   chan Lease
	notifyC  chan struct{}
	ctx      context.Context
	cancel   context.CancelFunc
}

// notify ensures that group is in a notified state.
// if the group is already in a notified state, this
// method has no effect.  This function should be called
// any time state is changed to ensure that the group's
// goroutine unblocks & handles the updated state.
func (g *group) notify() {
	select {
	case g.notifyC <- struct{}{}:
	default:
	}
}

// loadCounts loads the current lease counts.
func (g *group) loadCounts() Counts {
	g.cmu.Lock()
	defer g.cmu.Unlock()
	return g.counts
}

// incrActive increases the active lease count by 1
// and ensures group is in the notified state.
func (g *group) incrActive() Counts {
	g.cmu.Lock()
	defer g.cmu.Unlock()
	g.counts.Active++
	g.notify()
	return g.counts
}

// decrActive decreases the active lease count by 1
// and ensures group is in the notified state.
func (g *group) decrActive() Counts {
	g.cmu.Lock()
	defer g.cmu.Unlock()
	g.counts.Active--
	g.notify()
	return g.counts
}

// setTarget sets the target lease count that the group should
// attempt to converge toward.
func (g *group) setTarget(target uint64) {
	g.cmu.Lock()
	defer g.cmu.Unlock()
	g.counts.Target = target
	g.notify()
}

func (g *group) run() {
	var counts Counts
	var nextLease Lease
	var grant chan Lease
	for {
		counts = g.loadCounts()
		if counts.Active < counts.Target {
			// we are in a "granting" state; ensure that the
			// grant channel is non-nil, and initialize `nextLease`
			// if it hasn't been already.
			grant = g.grantC
			if nextLease.id == 0 {
				nextLease = newLease(g)
			}
		} else {
			// we are not in a "granting" state, ensure that the
			// grant channel is nil (prevents sends).
			grant = nil
		}
		// if grant is nil, this select statement blocks until
		// notify() is called, or the context is canceled.
		select {
		case grant <- nextLease:
			g.incrActive()
			nextLease = Lease{}
		case <-g.notifyC:
		case <-g.ctx.Done():
			return
		}
	}
}

// Lease grants access to a resource or group.  When the lease is received,
// work can begin.  Leases are held by workers and must be released when
// the worker has finished its work.
type Lease struct {
	*group
	id      uint64
	relOnce *sync.Once
}

func newLease(group *group) Lease {
	return Lease{
		group:   group,
		id:      group.leaseIDs.Add(1),
		relOnce: new(sync.Once),
	}
}

// ID returns the unique ID of this lease.
func (l Lease) ID() uint64 {
	return l.id
}

// Key returns the key that this lease is associated with.
func (l Lease) Key() interface{} {
	return l.key
}

// IsZero checks if this is the zero value of Lease.
func (l Lease) IsZero() bool {
	return l == Lease{}
}

// Release relenquishes this lease.  Each lease is unique,
// so double-calling Release on the same Lease has no effect.
func (l Lease) Release() {
	if l.IsZero() {
		return
	}
	l.relOnce.Do(func() {
		l.decrActive()
	})
}
