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

package track

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/gravitational/trace"
)

const (
	// DefaultProxyExpiry is the default amount of time a tracker will attempt
	// to successfully connect to a proxy before giving up
	DefaultProxyExpiry = 3 * time.Minute
)

// Config configures basic Tracker parameters.
type Config struct {
	// ProxyExpiry is the duration an entry will be held since the last
	// successful connection to, or message about, a given proxy.
	ProxyExpiry time.Duration
	// ClusterName is the name of the tracked cluster.
	ClusterName string
}

// CheckAndSetDefaults set default values for Config.
func (c *Config) CheckAndSetDefaults() error {
	if c.ProxyExpiry < 1 {
		c.ProxyExpiry = DefaultProxyExpiry
	}
	if c.ClusterName == "" {
		return trace.BadParameter("missing ClusterName in track.Config")
	}
	return nil
}

// Tracker represents the view that a reverse tunnel client (i.e. an agentpool)
// has over the reverse tunnel servers (i.e. proxies) that it's connected to and
// that it knows about. Based on that information, the Tracker is in charge of
// deciding if new connection attempts should be made, by giving out [Lease]s.
type Tracker struct {
	proxyExpiry   time.Duration
	clusterSuffix string

	leaseC chan *Lease

	mu   sync.Mutex
	cond chan struct{}

	connectionCount int

	inflight int
	claimed  map[string]struct{}
	tracked  map[string]Proxy
}

// Proxy holds the name and relevant metadata for a reverse tunnel server, as
// well as some internal bookkeeping data used by the Tracker.
type Proxy struct {
	Name       string
	Group      string
	Generation string

	expiry        time.Time
	deleteAttempt string
}

// Lease represents an authorization to attempt to connect to a reverse tunnel
// server, and to attempt to exclusively claim a specific server. It should be
// explicitly released after use.
type Lease struct {
	id int

	mu sync.Mutex
	t  *Tracker
	k  string
}

// New configures a new Tracker instance. All background goroutines stop when
// the context is closed.
func New(ctx context.Context, cfg Config) (*Tracker, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	t := &Tracker{
		proxyExpiry:   cfg.ProxyExpiry,
		clusterSuffix: "." + cfg.ClusterName,
		leaseC:        make(chan *Lease),
		cond:          make(chan struct{}, 1),
		claimed:       make(map[string]struct{}),
		tracked:       make(map[string]Proxy),
	}
	go t.run(ctx)
	return t, nil
}

// Acquire returns the channel from which the Tracker will give out Leases.
func (t *Tracker) Acquire() <-chan *Lease {
	return t.leaseC
}

func (t *Tracker) run(ctx context.Context) {
	// in lieu of figuring out the closest proxy expiration time every time we
	// enter the select, we just pick a reasonably frequent interval - there's
	// no real drawback to attempting new connections for a few more seconds
	// than necessary
	expiryTicker := time.NewTicker(t.proxyExpiry / 4)
	defer expiryTicker.Stop()

	newLeaseID := 1
	newLease := &Lease{
		id: newLeaseID,
		t:  t,
	}

	for {
		leaseC := t.leaseC
		if !t.canSpawn() {
			leaseC = nil
		}
		select {
		case leaseC <- newLease:
			newLeaseID++
			newLease = &Lease{
				id: newLeaseID,
				t:  t,
			}

			t.mu.Lock()
			t.inflight++
			t.mu.Unlock()

		case <-ctx.Done():
			return

		case <-t.cond:
		case <-expiryTicker.C:
		}
	}
}

// canSpawn returns true if the current state of the Tracker is such that the
// client should attempt to spawn new connections.
func (t *Tracker) canSpawn() bool {
	t.mu.Lock()
	defer t.mu.Unlock()

	now := time.Now()
	for k, v := range t.tracked {
		if v.expiry.Before(now) {
			delete(t.tracked, k)
		}
	}

	// degenerate condition: we've just started the tracker or we haven't
	// successfully connected to any server yet, or network conditions were such
	// that all tracked proxies have expired - spawn one connection, or we'll
	// get stuck like that forever
	if t.inflight == 0 && (len(t.claimed) == 0 || len(t.tracked) == 0) {
		return true
	}

	desiredGen := make(map[string]string, 8)
	for _, v := range t.tracked {
		if v.Generation > desiredGen[v.Group] {
			desiredGen[v.Group] = v.Generation
		}
	}

	desired := make(map[string]struct{}, len(t.tracked))
	for k, v := range t.tracked {
		if desiredGen[v.Group] == v.Generation {
			desired[k] = struct{}{}
		}
	}

	// do the set intersection like this because claimed is generally going to
	// be smaller than desired/tracked
	desiredClaimed := 0
	for k := range t.claimed {
		if _, ok := desired[k]; ok {
			desiredClaimed++
		}
	}

	if t.connectionCount == 0 {
		return len(desired) > desiredClaimed
	}

	return t.connectionCount > desiredClaimed
}

// notify signals the run loop that conditions have changed and that the tracker
// should check again if it should be giving out a lease or not.
func (t *Tracker) notify() {
	select {
	case t.cond <- struct{}{}:
	default:
	}
}

// TrackExpected starts/refreshes tracking for expected proxies.  Called by
// agents when gossip messages are received.
func (t *Tracker) TrackExpected(sourceID string, proxies ...Proxy) {
	if len(proxies) == 0 {
		return
	}

	t.mu.Lock()
	defer t.mu.Unlock()
	t.notify()

	expiry := time.Now().Add(t.proxyExpiry)
	for _, p := range proxies {
		p.expiry = expiry
		p.deleteAttempt = ""
		t.tracked[p.Name] = p
	}

	if sourceID == "" {
		return
	}

	for k, v := range t.tracked {
		if v.expiry == expiry {
			// we have just added/updated this (or some other gossip message did
			// the same in the exact same nanosecond, which isn't really
			// possible), so we should not attempt to delete this proxy
			continue
		}

		switch v.deleteAttempt {
		case "":
			v.deleteAttempt = sourceID
			t.tracked[k] = v
		case sourceID:
			// do nothing, we want a second opinion before deleting the proxy
		default:
			// we are the second opinion
			delete(t.tracked, k)
		}
	}
}

// SetConnectionCount updates the desired connection count as defined by the
// tunnel_strategy; 0 means full connectivity, i.e. "agent mesh" mode, a nonzero
// value (the connection_count of the tunnel_strategy) is proxy peering mode.
func (t *Tracker) SetConnectionCount(connectionCount int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.notify()

	t.connectionCount = connectionCount
}

// Claim attempts to claim exclusive access to a reverse tunnel server
// identified by the principals. It will fail if the server is already claimed
// or if the Lease had already been released or has already claimed a different
// server.
func (l *Lease) Claim(principals ...string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.t == nil || l.k != "" {
		return false
	}

	k := l.t.claim(principals)
	if k == "" {
		return false
	}

	l.k = k
	return true
}

// claim attempts to claim a server on behalf of an unclaimed, unreleased Lease.
// Returns the server name on success, an empty string on failure.
func (t *Tracker) claim(principals []string) string {
	t.mu.Lock()
	defer t.mu.Unlock()

	k := t.resolveNameLocked(principals)
	if k == "" {
		return ""
	}

	if _, ok := t.claimed[k]; ok {
		return ""
	}

	t.notify()
	t.claimed[k] = struct{}{}
	t.inflight--

	return k
}

// IsClaimed returns true if the reverse tunnel server identified by the
// principals has already been claimed at the time of the call. Keep in mind
// that a false return value does not imply that a subsequent call to Claim on a
// Lease with the same principals is guaranteed to succeed, as other goroutines
// might also be attempting to claim the same server.
func (t *Tracker) IsClaimed(principals ...string) bool {
	t.mu.Lock()
	defer t.mu.Unlock()

	k := t.resolveNameLocked(principals)
	if k == "" {
		return false
	}

	_, ok := t.claimed[k]
	return ok
}

// Release drops the claim on the server (if any) or the count of inflight
// connections in the tracker (if not). It's safe to call multiple times; calls
// other than the first are a no-op.
func (l *Lease) Release() {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.t == nil {
		return
	}

	l.t.release(l.k)
	l.t = nil
}

// IsReleased returns true if Release has been called. Used by tests.
func (l *Lease) IsReleased() bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	return l.t == nil
}

// release releases the claim on a server or reduces the inflight count, on
// behalf of a Lease.
func (t *Tracker) release(name string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.notify()

	if name == "" {
		t.inflight--
	} else {
		delete(t.claimed, name)
	}
}

// ID returns a numerical ID associated with the Lease, for debugging purposes;
// IDs are consecutively assigned starting from 1 for each given Tracker.
func (l *Lease) ID() int {
	return l.id
}

// resolveName tries to extract the UUID of the proxy as that's the
// only unique identifier in the list of principals.
func (t *Tracker) resolveNameLocked(principals []string) string {
	if len(principals) == 0 {
		return ""
	}

	// check if we're already using one of these principals.
	for _, name := range principals {
		if _, ok := t.tracked[name]; ok {
			return name
		}
		if _, ok := t.claimed[name]; ok {
			return name
		}
	}

	// default to using the first principal without the `.<cluster-name>` suffix, if any
	name, _ := strings.CutSuffix(principals[0], t.clusterSuffix)
	return name
}
