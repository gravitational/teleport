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

	mu sync.Mutex

	// connectionCount is nonpositive for full connectivity (agent mesh) mode, a
	// positive number for the connection count of proxy peering mode.
	connectionCount int

	// cannotLease being true indicates that no leases should be granted, but
	// false does not necessarily mean that leases *can* be granted. this flag
	// should be reset whenever the tracker state is changed such that we
	// *might* be able to grant leases.
	cannotLease bool

	// lastLease is the ID of the last lease that was granted. It starts at 0,
	// so the first Lease will have ID 1.
	lastLease int

	// inflight counts the granted leases that haven't claimed
	inflight int

	// claimed contains the names of all the claimed proxies.
	claimed map[string]struct{}

	// tracked contains the tracked proxies as a map of name to Proxy struct.
	tracked map[string]Proxy
}

// Proxy holds the name and relevant metadata for a reverse tunnel server, as
// well as some internal bookkeeping data used by the Tracker.
type Proxy struct {
	Name       string
	Group      string
	Generation uint64

	expiry time.Time
}

// Lease represents an authorization to attempt to connect to a reverse tunnel
// server, and to attempt to exclusively claim a specific server. It should be
// explicitly released after use.
type Lease struct {
	// id is a counter used to distinguish leases in logs.
	id int

	mu sync.Mutex
	// tracker is nil if the Lease has been released.
	tracker *Tracker
	// claimName contains the name of the claimed server after a claim is
	// successful.
	claimName string
}

// New configures a new Tracker instance.
func New(cfg Config) (*Tracker, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	t := &Tracker{
		proxyExpiry:   cfg.ProxyExpiry,
		clusterSuffix: "." + cfg.ClusterName,
		claimed:       make(map[string]struct{}),
		tracked:       make(map[string]Proxy),
	}
	return t, nil
}

// TryAcquire attempts to acquire a [Lease] from the tracker; if we shouldn't
// attempt a new connection at the moment, it will return nil, otherwise it will
// return a new Lease in the unclaimed state that can attempt to claim
// exclusivity over a proxy and that must be released (with [Lease.Release()])
// at the end of its lifetime.
func (t *Tracker) TryAcquire() *Lease {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.expireProxiesLocked()

	if t.cannotLease {
		return nil
	}

	if !t.canLeaseLocked() {
		// until cannotLease is reset (because something in the state is
		// changed) we know that we can't grant a lease, so we don't need to
		// check again
		t.cannotLease = true
		return nil
	}

	t.lastLease++
	t.inflight++
	return &Lease{
		id:      t.lastLease,
		tracker: t,
	}
}

func (t *Tracker) expireProxiesLocked() {
	now := time.Now()
	for k, v := range t.tracked {
		if v.expiry.Before(now) {
			delete(t.tracked, k)
			t.cannotLease = false
		}
	}
}

// canLeaseLocked returns true if the current state of the Tracker is such that
// the client should attempt to spawn new connections.
func (t *Tracker) canLeaseLocked() bool {
	// degenerate condition: we've just started the tracker or we haven't
	// successfully connected to any server yet, or network conditions were such
	// that all tracked proxies have expired - spawn one connection, or we'll
	// get stuck like that forever
	if t.inflight == 0 && (len(t.claimed) == 0 || len(t.tracked) == 0) {
		return true
	}

	desiredGen := make(map[string]uint64, 8)
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

	desiredCount := len(desired)
	if t.connectionCount > 0 && desiredCount > t.connectionCount {
		desiredCount = t.connectionCount
	}

	return desiredCount > desiredClaimed+t.inflight
}

// TrackExpected starts/refreshes tracking for expected proxies.  Called by
// agents when gossip messages are received.
func (t *Tracker) TrackExpected(proxies ...Proxy) {
	if len(proxies) == 0 {
		return
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	expiry := time.Now().Add(t.proxyExpiry)
	t.cannotLease = false
	for _, p := range proxies {
		p.expiry = expiry
		t.tracked[p.Name] = p
	}
}

// SetConnectionCount updates the desired connection count as defined by the
// tunnel_strategy; 0 means full connectivity, i.e. "agent mesh" mode, a nonzero
// value (the connection_count of the tunnel_strategy) is proxy peering mode.
func (t *Tracker) SetConnectionCount(connectionCount int) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.cannotLease = false
	t.connectionCount = connectionCount
}

// Claim attempts to claim exclusive access to a reverse tunnel server
// identified by the principals. It will fail if the server is already claimed
// or if the Lease had already been released or has already claimed a different
// server.
func (l *Lease) Claim(principals ...string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.tracker == nil || l.claimName != "" {
		return false
	}

	l.claimName = l.tracker.claim(principals)
	return l.claimName != ""
}

// claim attempts to claim a server on behalf of an unclaimed, unreleased Lease.
// Returns the server name on success, an empty string on failure.
func (t *Tracker) claim(principals []string) string {
	t.mu.Lock()
	defer t.mu.Unlock()

	name := t.resolveNameLocked(principals)
	if name == "" {
		return ""
	}

	if _, ok := t.claimed[name]; ok {
		return ""
	}

	t.cannotLease = false
	t.claimed[name] = struct{}{}
	t.inflight--

	return name
}

// IsClaimed returns true if the reverse tunnel server identified by the
// principals has already been claimed at the time of the call. Keep in mind
// that a false return value does not imply that a subsequent call to Claim on a
// Lease with the same principals is guaranteed to succeed, as other goroutines
// might also be attempting to claim the same server.
func (t *Tracker) IsClaimed(principals ...string) bool {
	t.mu.Lock()
	defer t.mu.Unlock()

	name := t.resolveNameLocked(principals)
	if name == "" {
		return false
	}

	_, ok := t.claimed[name]
	return ok
}

// Release drops the claim on the server (if any) or the count of inflight
// connections in the tracker (if not). It's safe to call multiple times; calls
// other than the first are a no-op.
func (l *Lease) Release() {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.tracker == nil {
		return
	}

	l.tracker.release(l.claimName)
	l.tracker = nil
}

// IsReleased returns true if Release has been called. Used by tests.
func (l *Lease) IsReleased() bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	return l.tracker == nil
}

// release releases the claim on a server or reduces the inflight count, on
// behalf of a Lease.
func (t *Tracker) release(name string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.cannotLease = false
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
