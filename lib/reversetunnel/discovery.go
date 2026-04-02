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
	"os"
	"strconv"
	"strings"
	"sync"
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

	ProxyGroupID         string        `json:"gid,omitempty"`
	ProxyGroupGeneration uint64        `json:"ggen,omitempty"`
	TTL                  time.Duration `json:"ttl,omitempty"`
}

// SetProxies overwrites the proxy list in the discoveryRequest with data from
// the slice of [types.Server]s.
func (r *discoveryRequest) SetProxies(proxies []types.Server) {
	r.Proxies = make([]discoveryProxy, 0, len(proxies))
	for _, proxy := range proxies {
		d := discoveryProxy{
			Version: types.V2,
		}
		d.Metadata.Name = proxy.GetName()
		d.ProxyGroupID, _ = proxy.GetLabel(types.ProxyGroupIDLabel)
		proxyGroupGeneration, _ := proxy.GetLabel(types.ProxyGroupGenerationLabel)
		var err error
		d.ProxyGroupGeneration, err = strconv.ParseUint(proxyGroupGeneration, 10, 64)
		if err != nil {
			// ParseUint can return the maximum uint64 on ErrRange
			d.ProxyGroupGeneration = 0
		}

		r.Proxies = append(r.Proxies, d)
	}
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

// discoSub is a subscriber to proxy discovery events.
type discoSub struct {
	pb      *discoPub
	notify  chan struct{}
	version atomic.Uint64
}

// Wait returns a channel which is notified when there is an event to fetch.
func (s *discoSub) Wait() <-chan struct{} {
	return s.notify
}

// Get returns each [discoveryProxy] fetches the latest set of proxies. If compaction
// is enabled ony the changes since the last fetch is returned.
func (s *discoSub) Get() []discoveryProxy {
	return s.pb.get(s, true)
}

// GetAll returns all [discoveryProxy]s.
func (s *discoSub) GetAll() []discoveryProxy {
	return s.pb.get(s, false)
}

// Close cleans up resources allocated for the subscriber.
func (s *discoSub) Close() {
	s.pb.unsubscribe(s)
	// close the channel AFTER unsubscribing to ensure no sends after close.
	close(s.notify)
}

// discoPub broadcasts proxy watch events to many subscribers.
type discoPub struct {
	ctx     context.Context
	cancel  func()
	watcher *services.GenericWatcher[types.Server, readonly.Server]
	compact bool

	// pm manages access to [discoPub.proxies], [discoPub.versions], [discoPub.version].
	pm       sync.RWMutex
	proxies  []types.Server
	versions map[string]proxyversion
	version  uint64

	// sm manages access to [discoPub.subs].
	sm   sync.Mutex
	subs map[*discoSub]struct{}
}

type proxyversion struct {
	version uint64
	expiry  time.Time
	updated time.Time
}

// newDiscoPub constructs a [discoPub] using the given [services.GenericWatcher].
func newDiscoPub(ctx context.Context, watcher *services.GenericWatcher[types.Server, readonly.Server]) *discoPub {
	ctx, cancel := context.WithCancel(ctx)
	v := os.Getenv("TELEPORT_UNSTABLE_PROXY_COMPACT_DISCOVERY")
	compact, _ := strconv.ParseBool(v)

	pb := &discoPub{
		ctx:     ctx,
		cancel:  cancel,
		watcher: watcher,
		subs:    make(map[*discoSub]struct{}),
		compact: compact,
	}
	go pb.run()
	return pb
}

func discoFromServer(s types.Server, ttl time.Duration) discoveryProxy {
	p := discoveryProxy{
		Version: types.V2,
	}
	p.Metadata.Name = s.GetName()
	p.TTL = ttl
	p.ProxyGroupID, _ = s.GetLabel(types.ProxyGroupIDLabel)
	proxyGroupGeneration, _ := s.GetLabel(types.ProxyGroupGenerationLabel)
	var err error
	p.ProxyGroupGeneration, err = strconv.ParseUint(proxyGroupGeneration, 10, 64)
	if err != nil {
		// ParseUint can return the maximum uint64 on ErrRange
		p.ProxyGroupGeneration = 0
	}
	return p
}

func (pb *discoPub) run() {
	for {
		select {
		case <-pb.ctx.Done():
			return
		case servers, ok := <-pb.watcher.ResourcesC:
			if !ok {
				return
			}
			now := pb.watcher.Clock.Now()
			pb.pm.Lock()
			pb.version++
			pb.proxies = servers
			prevVersions := pb.versions
			pb.versions = make(map[string]proxyversion, len(servers))
			for _, server := range servers {
				pv, ok := prevVersions[server.GetName()]
				if !ok || !server.Expiry().Equal(pv.expiry) {
					pv.version = pb.version
					pv.updated = now
				}
				pv.expiry = server.Expiry()
				pb.versions[server.GetName()] = pv
			}
			pb.pm.Unlock()

			pb.sm.Lock()
			for s := range pb.subs {
				select {
				case s.notify <- struct{}{}:
				default:
				}
			}
			pb.sm.Unlock()
		}
	}
}

// Subscribe returns a new [discoSub] for receiving proxy event updates.
func (pb *discoPub) Subscribe() *discoSub {
	s := &discoSub{
		pb:     pb,
		notify: make(chan struct{}, 1),
	}

	pb.sm.Lock()
	pb.subs[s] = struct{}{}
	pb.sm.Unlock()

	pb.pm.RLock()
	version := pb.version
	pb.pm.RUnlock()

	if version > 0 {
		select {
		case s.notify <- struct{}{}:
		default:
		}
	}
	return s
}

// Close cleans up resources allocated by a [discoPub].
func (pb *discoPub) Close() {
	pb.cancel()
	pb.watcher.Close()
}

// get fetches the latest set of [discoveryProxy]s. If sinceLastVersion is true
// only proxies that have updated their expiry since the last get will be returned.
func (pb *discoPub) get(s *discoSub, sinceLastVersion bool) []discoveryProxy {
	pb.pm.RLock()
	proxies := pb.proxies
	version := pb.version
	versions := pb.versions
	compact := pb.compact
	pb.pm.RUnlock()

	var ttl time.Duration
	if compact {
		ttl = defaults.ProxyAnnounceTTL()
	}

	now := pb.watcher.Clock.Now()
	disco := make([]discoveryProxy, 0, len(proxies))
	lastVersion := s.version.Swap(version)
	for _, proxy := range proxies {
		if compact {
			pv, ok := versions[proxy.GetName()]
			if !ok {
				continue
			}
			if pv.updated.Add(defaults.ProxyAnnounceTTL()).Before(now) {
				continue
			}
			if sinceLastVersion && lastVersion >= pv.version {
				continue
			}
		}
		disco = append(disco, discoFromServer(proxy, ttl))
	}
	return disco
}

func (pb *discoPub) unsubscribe(s *discoSub) {
	pb.sm.Lock()
	defer pb.sm.Unlock()
	delete(pb.subs, s)
}
