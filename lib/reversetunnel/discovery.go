/*
Copyright 2016-2019 Gravitational, Inc.

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

package reversetunnel

import (
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
	ProxyGroupGeneration string `json:"ggen,omitempty"`
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
		d.ProxyGroupGeneration, _ = proxy.GetLabel(types.ProxyGroupGenerationLabel)

		r.Proxies = append(r.Proxies, d)
	}
}

// ProxyNames returns the names of all proxies carried in the request
func (r discoveryRequest) ProxyNames() []string {
	names := make([]string, 0, len(r.Proxies))
	for _, p := range r.Proxies {
		names = append(names, p.Metadata.Name)
	}

	return names
}

func (r discoveryRequest) TrackProxies() []track.Proxy {
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

func (r discoveryRequest) String() string {
	var b strings.Builder
	b.WriteRune('[')
	for i, p := range r.Proxies {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(p.Metadata.Name)
		if p.ProxyGroupID != "" || p.ProxyGroupGeneration != "" {
			b.WriteRune('(')
			b.WriteString(p.ProxyGroupID)
			b.WriteRune('@')
			b.WriteString(p.ProxyGroupGeneration)
			b.WriteRune(')')
		}
	}
	b.WriteRune(']')
	return b.String()
}
