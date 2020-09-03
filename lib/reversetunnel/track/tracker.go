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

	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/workpool"
)

type Lease = workpool.Lease

// Key uniquely identifies a reversetunnel endpoint.
type Key struct {
	Cluster string
	Addr    utils.NetAddr
}

// Config configures basic Tracker parameters.
type Config struct {
	// ProxyExpiry is the duration an entry will be held sice the last
	// successful connection to, or message about, a given proxy.
	ProxyExpiry time.Duration
	// TickRate is the rate at which expired entries are cleared from
	// the cache of known proxies.
	TickRate time.Duration
}

// SetDefaults set default values for Config.
func (c *Config) SetDefaults() {
	if c.ProxyExpiry < 1 {
		c.ProxyExpiry = 3 * time.Minute
	}
	if c.TickRate < 1 {
		c.TickRate = 30 * time.Second
	}
}

// Tracker is a helper for tracking proxies located behind reverse tunnels
// and triggering agent spawning as needed.  Tracker wraps a workpool.Pool
// instance and manages a cache of proxies which *may* exist.  As proxies are
// discovered, or old proxies expire, the target counts are automatically updated
// for the associated key in the workpool.  Agents can attempt to "claim"
// exclusivity for a given proxy, ensuring that multiple agents are not run
// against the same proxy.
type Tracker struct {
	Config
	mu     sync.Mutex
	wp     *workpool.Pool
	sets   map[Key]*proxySet
	cancel context.CancelFunc
}

// New configures a new tracker instance.
func New(ctx context.Context, c Config) *Tracker {
	ctx, cancel := context.WithCancel(ctx)
	c.SetDefaults()
	t := &Tracker{
		Config: c,
		wp:     workpool.NewPool(ctx),
		sets:   make(map[Key]*proxySet),
		cancel: cancel,
	}
	go t.run(ctx)
	return t
}

func (t *Tracker) run(ctx context.Context) {
	ticker := time.NewTicker(t.TickRate)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			t.tick()
		case <-ctx.Done():
			return
		}
	}
}

// Acquire grants access to the Acquire channel of the
// embedded work group.
func (p *Tracker) Acquire() <-chan Lease {
	return p.wp.Acquire()
}

// TrackExpected starts/refreshes tracking for expected proxies.  Called by
// agents when gossip messages are received.
func (p *Tracker) TrackExpected(lease Lease, proxies ...string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	key := lease.Key().(Key)
	set, ok := p.sets[key]
	if !ok {
		return
	}
	t := time.Now()
	for _, name := range proxies {
		set.markSeen(t, name)
	}
	count := len(set.proxies)
	if count < 1 {
		count = 1
	}
	p.wp.Set(key, uint64(count))
}

// Start starts tracking for specified key.
func (p *Tracker) Start(key Key) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.getOrCreate(key)
}

// Stop stops tracking for specified key.
func (p *Tracker) Stop(key Key) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if _, ok := p.sets[key]; !ok {
		return
	}
	delete(p.sets, key)
	p.wp.Set(key, 0)
}

// StopAll permanently deactivates this tracker and cleans
// up all background goroutines.
func (p *Tracker) StopAll() {
	p.cancel()
}

func (p *Tracker) tick() {
	p.mu.Lock()
	defer p.mu.Unlock()
	cutoff := time.Now().Add(-1 * p.ProxyExpiry)
	for key, set := range p.sets {
		if set.expire(cutoff) > 0 {
			count := len(set.proxies)
			if count < 1 {
				count = 1
			}
			p.wp.Set(key, uint64(count))
		}
	}
}

func (p *Tracker) getOrCreate(key Key) *proxySet {
	if s, ok := p.sets[key]; ok {
		return s
	}
	set := newProxySet(key)
	p.sets[key] = set
	p.wp.Set(key, 1)
	return set
}

// WithProxy runs the supplied closure if and only if
// no other work is currently being done with the proxy
// identified by principals.
func (p *Tracker) WithProxy(work func(), lease Lease, principals ...string) (didWork bool) {
	key := lease.Key().(Key)
	if ok := p.claim(key, principals...); !ok {
		return false
	}
	defer p.unclaim(key, principals...)
	work()
	return true
}

func (p *Tracker) claim(key Key, principals ...string) (ok bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	set, ok := p.sets[key]
	if !ok {
		return false
	}
	return set.claim(principals...)
}

func (p *Tracker) unclaim(key Key, principals ...string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	set, ok := p.sets[key]
	if !ok {
		return
	}
	set.unclaim(principals...)
}

type entry struct {
	lastSeen time.Time
	claimed  bool
}

func newProxySet(key Key) *proxySet {
	return &proxySet{
		key:     key,
		proxies: make(map[string]entry),
	}
}

type proxySet struct {
	key     Key
	proxies map[string]entry
}

func (p *proxySet) claim(principals ...string) (ok bool) {
	proxy := p.resolveName(principals)
	e, ok := p.proxies[proxy]
	if !ok {
		p.proxies[proxy] = entry{
			claimed: true,
		}
		return true
	}
	if e.claimed {
		return false
	}
	e.claimed = true
	p.proxies[proxy] = e
	return true
}

func (p *proxySet) unclaim(principals ...string) {
	proxy := p.resolveName(principals)
	p.proxies[proxy] = entry{
		lastSeen: time.Now(),
	}
}

func (p *proxySet) markSeen(t time.Time, proxy string) {
	e, ok := p.proxies[proxy]
	if !ok {
		p.proxies[proxy] = entry{
			lastSeen: t,
		}
		return
	}
	if e.lastSeen.After(t) {
		return
	}
	e.lastSeen = t
	p.proxies[proxy] = e
}

func (p *proxySet) expire(cutoff time.Time) (removed int) {
	for name, entry := range p.proxies {
		if entry.claimed {
			continue
		}
		if entry.lastSeen.Before(cutoff) {
			delete(p.proxies, name)
			removed++
		}
	}
	return
}

// resolveName tries to extract the UUID of the proxy as that's the
// only unique identifier in the list of principals.
func (p *proxySet) resolveName(principals []string) string {
	// check if we're already using one of these principals.
	for _, name := range principals {
		if _, ok := p.proxies[name]; ok {
			return name
		}
	}
	// default to using the first principal
	name := principals[0]
	// if we have a `.<cluster-name>` suffix, remove it.
	if strings.HasSuffix(name, p.key.Cluster) {
		t := strings.TrimSuffix(name, p.key.Cluster)
		if strings.HasSuffix(t, ".") {
			name = strings.TrimSuffix(t, ".")
		}
	}
	return name
}
