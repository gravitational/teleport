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
	"fmt"
	"strings"

	"github.com/gravitational/teleport/lib/auth/resource"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"
)

// discoveryRequest is a request sent from a connected proxy with the missing proxies.
type discoveryRequest struct {
	// ClusterName is the name of the cluster that sends the discovery request.
	ClusterName string `json:"cluster_name"`

	// Type is the type of tunnel, is either node or proxy.
	Type string `json:"type"`

	// ClusterAddr is the address of the cluster.
	ClusterAddr utils.NetAddr `json:"-"`

	// Proxies is a list of proxies in the cluster sending the discovery request.
	Proxies []services.Server `json:"proxies"`
}

func (r discoveryRequest) String() string {
	proxyNames := make([]string, 0, len(r.Proxies))
	for _, p := range r.Proxies {
		proxyNames = append(proxyNames, p.GetName())
	}
	return fmt.Sprintf("discovery request, cluster name: %v, address: %v, proxies: %v",
		r.ClusterName, r.ClusterAddr, strings.Join(proxyNames, ","))
}

type discoveryRequestRaw struct {
	ClusterName string            `json:"cluster_name"`
	Type        string            `json:"type"`
	Proxies     []json.RawMessage `json:"proxies"`
}

func marshalDiscoveryRequest(req discoveryRequest) ([]byte, error) {
	var out discoveryRequestRaw
	for _, p := range req.Proxies {
		// Clone the server value to avoid a potential race
		// since the proxies are shared.
		// Marshaling attempts to enforce defaults which modifies
		// the original value.
		p = p.DeepCopy()
		data, err := resource.MarshalServer(p)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		out.Proxies = append(out.Proxies, data)
	}
	out.ClusterName = req.ClusterName
	out.Type = req.Type
	return json.Marshal(out)
}

func unmarshalDiscoveryRequest(data []byte) (*discoveryRequest, error) {
	if len(data) == 0 {
		return nil, trace.BadParameter("missing payload in discovery request")
	}
	var raw discoveryRequestRaw
	err := utils.FastUnmarshal(data, &raw)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var out discoveryRequest
	for _, bytes := range raw.Proxies {
		proxy, err := resource.UnmarshalServer([]byte(bytes), services.KindProxy, resource.SkipValidation())
		if err != nil {
			return nil, trace.Wrap(err)
		}
		out.Proxies = append(out.Proxies, proxy)
	}
	out.ClusterName = raw.ClusterName
	out.Type = raw.Type
	return &out, nil
}
