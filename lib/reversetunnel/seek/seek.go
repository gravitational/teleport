/*
Copyright 2019 Gravitational, Inc.

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

package seek

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
)

var connectedGauge = prometheus.NewGauge(
	prometheus.GaugeOpts{
		Name: "reversetunnel_connected_proxies",
		Help: "Number of known proxies being sought.",
	},
)

func init() {
	prometheus.MustRegister(connectedGauge)
}

// Key uniquely identifies a seek group
type Key struct {
	Cluster string
	Type    string
	Addr    utils.NetAddr
}

// Config describes the various parameters related to a seek operation
type Config struct {
	// TickRate defines the maximum amount of time between expiry & seek checks.
	// Shorter tick rates reduce discovery delay.  Longer tick rates reduce resource
	// consumption (default: ~4s).
	TickRate time.Duration
	// EntryExpiry defines how long a seeker entry should be held without successfully
	// establishing a healthy connection.  This value should be reasonably long
	// (default: 3m).
	EntryExpiry time.Duration
	// BackoffInterval defines the basline backoff amount observed by seekers.  This value
	// should be reasonably short (default: 256ms)
	BackoffInterval time.Duration
	// BackoffThreshold defines the minimum amount of time that a connection is expected to last
	// if the conencted peer is generally healthy.  Connections which fail before BackoffThreshold
	// cause the seekstate to enter backoff (default: 30s)
	BackoffThreshold time.Duration
}

func (s *Config) Check() error {
	if s.TickRate < time.Millisecond {
		return trace.BadParameter("sub-millisecond tick-rate is not allowed")
	}
	if s.EntryExpiry <= 2*s.TickRate {
		return trace.BadParameter("entry-expiry must be greater than 2x tick-rate")
	}
	if s.EntryExpiry <= s.BackoffInterval {
		return trace.BadParameter("entry-expiry must be greater than backoff-interval")
	}
	if s.EntryExpiry <= s.BackoffThreshold {
		return trace.BadParameter("entry-expiry must be greater than backoff-threshold")
	}
	return nil
}

const (
	defaultTickRate         = time.Millisecond * 4096
	defaultEntryExpriy      = time.Minute * 3
	defaultBackoffInterval  = time.Millisecond * 256
	defaultBackoffThreshold = time.Second * 30
)

func (s *Config) CheckAndSetDefaults() error {
	const granularity = time.Millisecond
	if s.TickRate < granularity {
		s.TickRate = defaultTickRate
	}
	if s.EntryExpiry < granularity {
		s.EntryExpiry = defaultEntryExpriy
	}
	if s.BackoffInterval < granularity {
		s.BackoffInterval = defaultBackoffInterval
	}
	if s.BackoffThreshold < granularity {
		s.BackoffThreshold = defaultBackoffThreshold
	}
	return s.Check()
}

// Pool manages a collection of group-level seek operations.
type Pool struct {
	m      sync.Mutex
	conf   Config
	groups map[Key]GroupHandle
	seekC  chan Key
	ctx    context.Context
}

// NewPool configures a seek pool.
func NewPool(ctx context.Context, conf Config) (*Pool, error) {
	if err := conf.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	return &Pool{
		conf:   conf,
		groups: make(map[Key]GroupHandle),
		seekC:  make(chan Key, 128),
		ctx:    ctx,
	}, nil
}

// Group gets a handle to the seek manager for the specified
// group.  If none exists, one will be started.
func (p *Pool) Group(key Key) GroupHandle {
	p.m.Lock()
	defer p.m.Unlock()
	if group, ok := p.groups[key]; ok {
		return group
	}
	group := newGroupHandle(p.ctx, p.conf, p.seekC, key)
	p.groups[key] = group
	return group
}

// Seek channel yields stream of keys indicating which groups
// are seeking.
func (p *Pool) Seek() <-chan Key {
	return p.seekC
}

// Stop stops one or more group-level seek operations
func (p *Pool) Stop(group Key, groups ...Key) {
	p.m.Lock()
	defer p.m.Unlock()
	p.stopGroupHandle(group)
	for _, g := range groups {
		p.stopGroupHandle(g)
	}
}

// Shutdown stops all seek operations
func (p *Pool) Shutdown() {
	p.m.Lock()
	defer p.m.Unlock()
	for g, _ := range p.groups {
		p.stopGroupHandle(g)
	}
}

func (p *Pool) stopGroupHandle(key Key) {
	group, ok := p.groups[key]
	if !ok {
		return
	}
	group.cancel()
	delete(p.groups, key)
}

// GroupHandle is a handle to an ongoing seek process.  Each seek process
// manages a group of related proxies.  This handle allows agents to
// claim exclusive "locks" for individual proxies and to broadcast
// gossip to the process.
type GroupHandle struct {
	inner  *proxyGroup
	cancel context.CancelFunc
	proxyC chan<- string
	seekC  <-chan Key
	statC  <-chan Status
}

func newGroupHandle(ctx context.Context, conf Config, seekC chan Key, id Key) GroupHandle {
	ctx, cancel := context.WithCancel(ctx)
	proxyC := make(chan string, 128)
	statC := make(chan Status, 1)
	seekers := &proxyGroup{
		id:     id,
		conf:   conf,
		states: make(map[string]seeker),
		proxyC: proxyC,
		seekC:  seekC,
		statC:  statC,
	}
	handle := GroupHandle{
		inner:  seekers,
		cancel: cancel,
		proxyC: proxyC,
		seekC:  seekC,
		statC:  statC,
	}
	go seekers.run(ctx)
	return handle
}

// WithProxy is used to wrap the connection-handling logic of an agent,
// ensuring that it is run if and only if no other agent is already
// handling this proxy.
func (s *GroupHandle) WithProxy(do func(), principals ...string) (did bool) {
	if !s.inner.TryAcquireProxy(principals...) {
		return false
	}
	defer s.inner.ReleaseProxy(principals...)
	connectedGauge.Inc()
	defer connectedGauge.Dec()
	do()
	return true
}

// Status channel is regularly updated with the most recent status
// value.  Consuming status values is optional.
func (s *GroupHandle) Status() <-chan Status {
	return s.statC
}

// Gossip channel must be informed whenever a proxy's identity
// becomes known via gossip messages.
func (s *GroupHandle) Gossip() chan<- string {
	return s.proxyC
}

// proxyGroup manages all proxy seekers for a group.
type proxyGroup struct {
	sync.Mutex
	id       Key
	conf     Config
	states   map[string]seeker
	proxyC   <-chan string
	seekC    chan<- Key
	statC    chan Status
	lastStat *Status
}

// run is the "main loop" for the seek process.
func (p *proxyGroup) run(ctx context.Context) {
	const logInterval int = 8
	ticker := time.NewTicker(p.conf.TickRate)
	defer ticker.Stop()
	// supply initial status & seek notification.
	p.notifyStatus(p.Tick())
	p.notifyShouldSeek()
	var ticks int
	for {
		select {
		case <-ticker.C:
			stat := p.Tick()
			p.notifyStatus(stat)
			if stat.ShouldSeek() {
				p.notifyShouldSeek()
			}
			ticks++
			if ticks%logInterval == 0 {
				log.Debugf("SeekStates(states=%+v,id=%s)", p.GetStates(), p.id)
			}
		case proxy := <-p.proxyC:
			proxies := []string{proxy}
			// Collect any additional proxy messages
			// without blocking.
		Collect:
			for {
				select {
				case pr := <-p.proxyC:
					proxies = append(proxies, pr)
				default:
					break Collect
				}
			}
			count := p.RefreshProxy(proxies...)
			if count > 0 {
				p.notifyShouldSeek()
			}
		case <-ctx.Done():
			return
		}
	}
}

func (p *proxyGroup) Tick() Status {
	p.Lock()
	defer p.Unlock()
	now := time.Now()
	return p.tick(now)
}

// RefreshProxy refreshes expiration timers, returning the number of
// successful refreshes.  If the returned value is greater than zero,
// then at least one entry is unexpired and in `stateSeeking`.
// Entries are lazily created for previously unknown proxies.
func (p *proxyGroup) RefreshProxy(proxies ...string) int {
	p.Lock()
	defer p.Unlock()
	now := time.Now()
	var count int
	for _, proxy := range proxies {
		if p.refreshProxy(now, proxy) {
			count++
		}
	}
	return count
}

func (p *proxyGroup) refreshProxy(t time.Time, proxy string) (ok bool) {
	s := p.states[proxy]
	if s.transit(t, eventRefresh, &p.conf) {
		p.states[proxy] = s
		return true
	}
	return false
}

// notifyShouldSeek sets the seek channel.
func (p *proxyGroup) notifyShouldSeek() {
	select {
	case p.seekC <- p.id:
	default:
	}
}

// notifyStatus clears and sets the status channel.
func (p *proxyGroup) notifyStatus(s Status) {
	select {
	case <-p.statC:
	default:
	}
	select {
	case p.statC <- s:
	default:
	}
}

// AcquireProxy attempts to acquire the specified proxy.
func (p *proxyGroup) TryAcquireProxy(principals ...string) (ok bool) {
	p.Lock()
	defer p.Unlock()
	return p.acquireProxy(time.Now(), principals...)
}

// ReleaseProxy attempts to release the specified proxy.
func (p *proxyGroup) ReleaseProxy(principals ...string) (ok bool) {
	p.Lock()
	defer p.Unlock()
	return p.releaseProxy(time.Now(), principals...)
}

func (p *proxyGroup) acquireProxy(t time.Time, principals ...string) (ok bool) {
	if len(principals) < 1 {
		return false
	}
	name := p.resolveName(principals)
	s := p.states[name]
	if !s.transit(t, eventAcquire, &p.conf) {
		return false
	}
	p.states[name] = s
	return true
}

func (p *proxyGroup) releaseProxy(t time.Time, principals ...string) (ok bool) {
	if len(principals) < 1 {
		return false
	}
	name := p.resolveName(principals)
	s := p.states[name]
	if !s.transit(t, eventRelease, &p.conf) {
		return false
	}
	if s.state == stateSeeking {
		p.notifyShouldSeek()
	}
	p.states[name] = s
	return true
}

func (p *proxyGroup) resolveName(principals []string) string {
	// check if we're already using one of these principals
	for _, name := range principals {
		if _, ok := p.states[name]; ok {
			return name
		}
	}
	// default to using the first principal
	name := principals[0]
	// if we have a `.<cluster-name>` suffix, remove it.
	if strings.HasSuffix(name, p.id.Cluster) {
		t := strings.TrimSuffix(name, p.id.Cluster)
		if strings.HasSuffix(t, ".") {
			name = strings.TrimSuffix(t, ".")
		}
	}
	return name
}

func (p *proxyGroup) GetStates() map[string]seekState {
	p.Lock()
	defer p.Unlock()
	collector := make(map[string]seekState, len(p.states))
	for proxy, s := range p.states {
		collector[proxy] = s.state
	}
	return collector
}

// tick ticks all proxy seek states, returning a summary
// status.  This method also serves as the mechanism by which
// expired entries are removed.
func (p *proxyGroup) tick(t time.Time) Status {
	var stat Status
	for proxy, s := range p.states {
		// if proxy seeker is in expirable state, handle
		// the expiry.  expired seekers are removed, and
		// the soonest future expiry is recorded in stat.
		if exp, ok := s.expiry(p.conf.EntryExpiry); ok {
			if t.After(exp) {
				delete(p.states, proxy)
				continue
			}
		}
		// poll and record state of proxy seeker.
		switch state := s.tick(t, &p.conf); state {
		case stateSeeking:
			stat.Seeking++
		case stateClaimed:
			stat.Claimed++
		case stateBackoff:
			stat.Backoff++
		default:
			// this should never happen...
			log.Errorf("Proxy %s in invalid state %q, removing.", proxy, state)
			delete(p.states, proxy)
			continue
		}
		// seeker.tick may have affected an internal state
		// transition, so update the entry.
		p.states[proxy] = s
	}
	return stat
}
