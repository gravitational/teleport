/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package reversetunnel

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/reversetunnel/track"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/readonly"
)

// discoveryRequest is the minimal structure that can be exchanged as JSON as a
// valid gossip message according to the reverse tunnel discovery protocol.
type discoveryRequest struct {
	Proxies []discoveryProxy `json:"proxies"`
}

// discoveryProxy is the minimal structure that can be exchanged as JSON as a
// valid representation of a proxy in [discoveryRequest] according to the
// reverse tunnel discovery protocol. The Version field should be set to V2.
//
// The ProxyGroupID and ProxyGroupGeneration fields are used to pass the
// teleport.internal/proxygroup-id and teleport.internal/proxygroup-gen labels
// of a proxy without having to transfer the full label name.
type discoveryProxy struct {
	Version  string `json:"version"`
	Metadata struct {
		Name string `json:"name"`
	} `json:"metadata"`

	ProxyGroupID         string `json:"gid,omitempty"`
	ProxyGroupGeneration uint64 `json:"ggen,omitempty"`
	// TTL is used by the agent [track.Tracker] for proxy expiry. This allows
	// server side override of the trackers default expiry.
	TTL time.Duration `json:"ttl,omitempty"`
}

// ProxyNames returns the names of all proxies carried in the request.
func (r *discoveryRequest) ProxyNames() []string {
	names := make([]string, 0, len(r.Proxies))
	for _, p := range r.Proxies {
		names = append(names, p.Metadata.Name)
	}

	return names
}

// TrackProxies copies the list of proxies received in the request as a slice of
// [track.Proxy].
func (r *discoveryRequest) TrackProxies() []track.Proxy {
	tp := make([]track.Proxy, 0, len(r.Proxies))
	for _, p := range r.Proxies {
		tp = append(tp, track.Proxy{
			Name:       p.Metadata.Name,
			Group:      p.ProxyGroupID,
			Generation: p.ProxyGroupGeneration,
			TTL:        p.TTL,
		})
	}
	return tp
}

func (r *discoveryRequest) String() string {
	var b strings.Builder
	b.WriteRune('[')
	for i, p := range r.Proxies {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(p.Metadata.Name)
		if p.ProxyGroupID != "" || p.ProxyGroupGeneration != 0 {
			b.WriteRune('(')
			b.WriteString(p.ProxyGroupID)
			b.WriteRune('@')
			fmt.Fprintf(&b, "%v", p.ProxyGroupGeneration)
			b.WriteRune(')')
		}
	}
	b.WriteRune(']')
	return b.String()
}

// proxyDiscoverySubscriber is a subscriber to proxy discovery events.
type proxyDiscoverySubscriber struct {
	pb      *proxyDiscoveryPublisher
	notify  chan struct{}
	version uint64
}

// Wait returns a channel which is notified when there is an event to fetch.
func (s *proxyDiscoverySubscriber) Wait() <-chan struct{} {
	return s.notify
}

// Get returns each [discoveryProxy] fetches the latest set of proxies. If compaction
// is enabled ony the changes since the last fetch is returned.
func (s *proxyDiscoverySubscriber) Get() []discoveryProxy {
	return s.pb.get(s, discoGetParams{sinceLastVersion: true})
}

// GetAll returns all [discoveryProxy]s.
func (s *proxyDiscoverySubscriber) GetAll() []discoveryProxy {
	return s.pb.get(s, discoGetParams{sinceLastVersion: false})
}

// proxyDiscoveryPublisher broadcasts proxy watch events to many subscribers.
type proxyDiscoveryPublisher struct {
	ctx     context.Context
	cancel  func()
	watcher *services.GenericWatcher[types.Server, readonly.Server]
	compact bool
	log     *slog.Logger

	// state points to the current immutable discovery snapshot.
	// writers must publish a fully rebuilt discoveryState and never mutate it.
	state atomic.Pointer[discoveryState]
	// alwaysClosed is a channel sent to subscribers when they are first created
	// to notify them immediately to fetch the latest state. it is always closed
	// and never replaced.
	alwaysClosed chan struct{}
}

// discoveryState is a single published discovery snapshot.
// its fields are read without locking and must be treated as immutable.
type discoveryState struct {
	// proxies is the lastest set of servers received from the watcher.
	proxies []types.Server
	// proxy version associates a proxy by name with the version at which its
	// expiry last changed.
	versions map[string]proxyversion
	// version is incremented each time a proxy watch event is receieved.
	version uint64
	// notify is closed when this state is replaced to notify subscribers.
	notify chan struct{}
}

type proxyversion struct {
	version uint64
	expiry  time.Time
	updated time.Time
}

// newProxyDiscoveryPublisher constructs a [proxyDiscoveryPublisher] using the given [services.GenericWatcher].
func newProxyDiscoveryPublisher(ctx context.Context, watcher *services.GenericWatcher[types.Server, readonly.Server], logger *slog.Logger) *proxyDiscoveryPublisher {
	ctx, cancel := context.WithCancel(ctx)
	v := os.Getenv("TELEPORT_UNSTABLE_PROXY_COMPACT_DISCOVERY")
	compact, _ := strconv.ParseBool(v)

	dp := &proxyDiscoveryPublisher{
		ctx:          ctx,
		cancel:       cancel,
		watcher:      watcher,
		state:        atomic.Pointer[discoveryState]{},
		alwaysClosed: make(chan struct{}),
		compact:      compact,
		log:          logger,
	}
	close(dp.alwaysClosed)
	dp.state.Store(&discoveryState{
		versions: map[string]proxyversion{},
		notify:   make(chan struct{}),
	})
	go dp.run()
	return dp
}

func (dp *proxyDiscoveryPublisher) discoFromServer(s types.Server, ttl time.Duration) discoveryProxy {
	p := discoveryProxy{
		Version: types.V2,
	}
	p.Metadata.Name = s.GetName()
	p.TTL = ttl
	p.ProxyGroupID, _ = s.GetLabel(types.ProxyGroupIDLabel)
	proxyGroupGeneration, ok := s.GetLabel(types.ProxyGroupGenerationLabel)
	if !ok {
		return p
	}

	var err error
	p.ProxyGroupGeneration, err = strconv.ParseUint(proxyGroupGeneration, 10, 64)
	if err != nil {
		// ParseUint can return the maximum uint64 on ErrRange
		p.ProxyGroupGeneration = 0
		dp.log.DebugContext(dp.ctx, "Failed to parse proxy group generation", "error", err, "value", proxyGroupGeneration)
	}
	return p
}

func (dp *proxyDiscoveryPublisher) run() {
	for {
		select {
		case <-dp.ctx.Done():
			return
		case servers, ok := <-dp.watcher.ResourcesC:
			if !ok {
				dp.log.WarnContext(dp.ctx, "Proxy discovery watcher closed unexpectedly")
				return
			}
			now := dp.watcher.Clock.Now()

			prev := dp.state.Load()
			next := &discoveryState{
				proxies:  servers,
				versions: make(map[string]proxyversion, len(servers)),
				version:  prev.version + 1,
				notify:   make(chan struct{}),
			}
			for _, server := range servers {
				nextpv := proxyversion{
					version: next.version,
					expiry:  server.Expiry(),
					updated: now,
				}
				prevpv, ok := prev.versions[server.GetName()]
				if ok && prevpv.expiry.Equal(nextpv.expiry) {
					// Preserve previous version and updated timestamp if the
					// server hasn't heartbeated.
					nextpv.version = prevpv.version
					nextpv.updated = prevpv.updated
				}
				next.versions[server.GetName()] = nextpv
			}
			dp.state.Store(next)
			close(prev.notify)
		}
	}
}

// Subscribe returns a new [proxyDiscoverySubscriber] for receiving proxy event updates.
func (dp *proxyDiscoveryPublisher) Subscribe() *proxyDiscoverySubscriber {
	s := &proxyDiscoverySubscriber{
		pb:     dp,
		notify: dp.alwaysClosed,
	}
	state := dp.state.Load()
	if state.version == 0 {
		s.notify = state.notify
	}
	return s
}

// Close cleans up resources allocated by a [proxyDiscoveryPublisher].
func (dp *proxyDiscoveryPublisher) Close() {
	dp.cancel()
	dp.watcher.Close()
}

// discoGetParams contains parameters for [proxyDiscoveryPublisher.get].
type discoGetParams struct {
	// sinceLastVersion indicates that only proxies that have been fetched since
	// the last get by the subscriber will be returned.
	sinceLastVersion bool
}

// get fetches the latest set of [discoveryProxy]s.
func (dp *proxyDiscoveryPublisher) get(sub *proxyDiscoverySubscriber, params discoGetParams) []discoveryProxy {
	compact := dp.compact
	state := dp.state.Load()

	var ttl time.Duration
	if compact {
		ttl = defaults.ProxyAnnounceTTL()
	}

	now := dp.watcher.Clock.Now()
	disco := make([]discoveryProxy, 0, len(state.proxies))
	prevVersion := sub.version
	sub.version = state.version
	sub.notify = state.notify
	for _, proxy := range state.proxies {
		if compact {
			pv, ok := state.versions[proxy.GetName()]
			if !ok {
				continue
			}
			if pv.updated.Add(defaults.ProxyAnnounceTTL()).Before(now) {
				continue
			}
			if params.sinceLastVersion && prevVersion >= pv.version {
				continue
			}
		}
		disco = append(disco, dp.discoFromServer(proxy, ttl))
	}
	return disco
}
