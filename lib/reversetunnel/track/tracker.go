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

type Proxy struct {
	ID         string
	Group      string
	Generation string

	expiry       time.Time
	deleteSource string
}

type Lease struct {
	id int

	mu sync.Mutex
	t  *Tracker
	k  string
}

// New configures a new tracker instance.
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

func (t *Tracker) Acquire() <-chan *Lease {
	return t.leaseC
}

func (t *Tracker) run(ctx context.Context) {
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

func (t *Tracker) canSpawn() bool {
	t.mu.Lock()
	defer t.mu.Unlock()

	now := time.Now()
	for k, v := range t.tracked {
		if v.expiry.Before(now) {
			delete(t.tracked, k)
		}
	}

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

	desiredClaimed := 0
	for k := range t.claimed {
		if _, ok := desired[k]; ok {
			desiredClaimed++
		}
	}
	desiredUnclaimed := len(desired) - desiredClaimed

	println("tracked:", len(t.tracked), "desired:", len(desired), "claimed:", len(t.claimed), "desiredClaimed:", desiredClaimed)

	if t.connectionCount == 0 {
		return desiredUnclaimed > 0
	}

	return t.connectionCount > desiredClaimed
}

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
		p.deleteSource = ""
		t.tracked[p.ID] = p
	}

	if sourceID == "" {
		return
	}

	for k, v := range t.tracked {
		if v.expiry == expiry {
			continue
		}

		switch v.deleteSource {
		case "":
			println("tagging proxy for deletion", k, "source", sourceID)
			v.deleteSource = sourceID
			t.tracked[k] = v
		case sourceID:
		default:
			println("deleting proxy", k, "source", sourceID)
			delete(t.tracked, k)
		}
	}
}

func (t *Tracker) SetConnectionCount(connectionCount int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.notify()

	t.connectionCount = connectionCount
}

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

func (l *Lease) Release() {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.t == nil {
		return
	}

	l.t.release(l.k)
	l.t = nil
}

func (l *Lease) IsReleased() bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	return l.t == nil
}

func (t *Tracker) release(id string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.notify()

	if id == "" {
		t.inflight--
	} else {
		delete(t.claimed, id)
	}
}

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
