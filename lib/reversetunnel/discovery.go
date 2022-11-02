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

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
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
	Proxies []types.Server `json:"proxies"`
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
	out := discoveryRequestRaw{
		Proxies: make([]json.RawMessage, 0, len(req.Proxies)),
	}
	for _, p := range req.Proxies {
		// create a new server that clones only the id and kind as that's all we need
		// to propagate
		srv, err := types.NewServer(p.GetName(), p.GetKind(), types.ServerSpecV2{})
		if err != nil {
			return nil, trace.Wrap(err)
		}

		data, err := utils.FastMarshal(srv)
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
	if err := utils.FastUnmarshal(data, &raw); err != nil {
		return nil, trace.Wrap(err)
	}

	out := discoveryRequest{
		Proxies: make([]types.Server, 0, len(raw.Proxies)),
	}
	for _, bytes := range raw.Proxies {
		proxy, err := services.UnmarshalServer(bytes, types.KindProxy)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		out.Proxies = append(out.Proxies, proxy)
	}
	out.ClusterName = raw.ClusterName
	out.Type = raw.Type
	return &out, nil
}
