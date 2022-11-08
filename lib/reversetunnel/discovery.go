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
	"encoding/json"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"
)

// discoveryRequest is a request sent from a connected proxy with the missing proxies.
type discoveryRequest struct {
	// Proxies is a list of proxies in the cluster sending the discovery request.
	Proxies []types.Server `json:"proxies"`
}

// ProxyNames returns the names of all proxies carried in the request
func (r *discoveryRequest) ProxyNames() []string {
	names := make([]string, 0, len(r.Proxies))
	for _, p := range r.Proxies {
		names = append(names, p.GetName())
	}

	return names
}

// MarshalJSON creates a minimal JSON representation of a discoveryRequest
// by converting the Proxies from types.Server to discoveryProxy.
// The minification is useful since only the Proxy ID is to be consumed
// by the agents. This is needed to maintain backward compatibility
// but should be replaced in the future by a message which
// only contains the Proxy IDs.
func (r *discoveryRequest) MarshalJSON() ([]byte, error) {
	var out struct {
		Proxies []discoveryProxy `json:"proxies"`
	}

	out.Proxies = make([]discoveryProxy, 0, len(r.Proxies))

	for _, p := range r.Proxies {
		out.Proxies = append(out.Proxies, discoveryProxy(p.GetName()))
	}

	return json.Marshal(out)
}

func (r *discoveryRequest) UnmarshalJSON(data []byte) error {
	if len(data) == 0 {
		return trace.BadParameter("missing payload in discovery request")
	}

	var in struct {
		Proxies []json.RawMessage `json:"proxies"`
	}

	if err := utils.FastUnmarshal(data, &in); err != nil {
		return trace.Wrap(err)
	}

	d := discoveryRequest{
		Proxies: make([]types.Server, 0, len(in.Proxies)),
	}

	for _, bytes := range in.Proxies {
		proxy, err := services.UnmarshalServer(bytes, types.KindProxy)
		if err != nil {
			return trace.Wrap(err)
		}

		d.Proxies = append(d.Proxies, proxy)
	}

	*r = d
	return nil
}

// discoveryProxy is a wrapper around a Proxy ID that
// can be marshaled to json in the minimal representation
// of a types.Server that will still be correctly unmarshalled
// as a types.Server. Backwards compatibility requires a types.Server
// to be included in a discoveryRequest when in reality only
// the Proxy ID needs to be communicated to agents.
//
// This should eventually be replaced by a newer version of
// messages used by agents to indicate they can support discovery
// requests which only contain Proxy IDs.
type discoveryProxy string

// MarshalJSON creates a minimum representation of types.Server
// such that (*discoveryRequest) UnmarshalJSON will successfully
// unmarshal this as a types.Server. This allows the discoveryRequest
// to be four and a half times smaller when marshaled.
func (s discoveryProxy) MarshalJSON() ([]byte, error) {
	var p struct {
		Version  string `json:"version"`
		Metadata struct {
			Name string `json:"name"`
		} `json:"metadata"`
	}
	p.Version = types.V2
	p.Metadata.Name = string(s)
	return json.Marshal(p)
}
