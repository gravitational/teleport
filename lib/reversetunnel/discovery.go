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

// Proxies is a list of proxies to discover
type Proxies []services.Server

// String returns text representation of the proxies
func (proxies Proxies) String() string {
	var out []string
	for _, proxy := range proxies {
		out = append(out, proxy.GetName())
	}
	return strings.Join(out, ",")
}

// Equal compares two lists of proxies as sets
func (proxies Proxies) Equal(other []services.Server) bool {
	if len(proxies) != len(other) {
		return false
	}
	proxiesMap, otherMap := make(map[string]bool), make(map[string]bool)
	for i := range proxies {
		proxiesMap[proxies[i].GetName()] = true
	}
	for i := range other {
		otherMap[other[i].GetName()] = true
	}
	for key := range otherMap {
		if !proxiesMap[key] {
			return false
		}
	}
	return true
}

func (r discoveryRequest) key() agentKey {
	return agentKey{clusterName: r.ClusterName, tunnelType: r.Type, addr: r.ClusterAddr}
}

func (r discoveryRequest) String() string {
	return fmt.Sprintf("discovery request, cluster name: %v, address: %v, proxies: %v",
		r.ClusterName, r.ClusterAddr, Proxies(r.Proxies))
}

type discoveryRequestRaw struct {
	ClusterName string            `json:"cluster_name"`
	Type        string            `json:"type"`
	Proxies     []json.RawMessage `json:"proxies"`
}

func marshalDiscoveryRequest(req discoveryRequest) ([]byte, error) {
	var out discoveryRequestRaw
	m := services.GetServerMarshaler()
	for _, p := range req.Proxies {
		data, err := m.MarshalServer(p)
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
	m := services.GetServerMarshaler()
	var out discoveryRequest
	for _, bytes := range raw.Proxies {
		proxy, err := m.UnmarshalServer([]byte(bytes), services.KindProxy, services.SkipValidation())
		if err != nil {
			return nil, trace.Wrap(err)
		}
		out.Proxies = append(out.Proxies, proxy)
	}
	out.ClusterName = raw.ClusterName
	out.Type = raw.Type
	return &out, nil
}
