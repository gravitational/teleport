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
	"fmt"
	"strconv"
	"strings"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/reversetunnel/track"
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
