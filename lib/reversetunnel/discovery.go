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
	// TunnelID identifies who the tunnel is connected to. For trusted clusters,
	// the TunnelID is the name of the remote cluster (like example.com). For
	// nodes, it is the nodeID (like 4a050852-23b5-4d6d-a45f-bed02792d453.example.com).
	TunnelID string `json:"cluster_name"`

	// Type is the type of tunnel, is either node or proxy.
	Type string `json:"type"`

	// ClusterAddr is the address of the cluster.
	ClusterAddr utils.NetAddr `json:"-"`

	// Proxies are the missing proxies.
	Proxies []services.Server `json:"proxies"`
}

type Proxies []services.Server

func (proxies Proxies) String() string {
	var out []string
	for _, proxy := range proxies {
		out = append(out, proxy.GetName())
	}
	return strings.Join(out, ",")
}

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
	return agentKey{tunnelID: r.TunnelID, tunnelType: r.Type, addr: r.ClusterAddr}
}

func (r discoveryRequest) String() string {
	return fmt.Sprintf("discovery request, cluster name: %v, address: %v, proxies: %v",
		r.TunnelID, r.ClusterAddr, Proxies(r.Proxies))
}

type discoveryRequestRaw struct {
	TunnelID string            `json:"cluster_name"`
	Type     string            `json:"type"`
	Proxies  []json.RawMessage `json:"proxies"`
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
	out.TunnelID = req.TunnelID
	out.Type = req.Type
	return json.Marshal(out)
}

func unmarshalDiscoveryRequest(data []byte) (*discoveryRequest, error) {
	if len(data) == 0 {
		return nil, trace.BadParameter("missing payload")
	}
	var raw discoveryRequestRaw
	err := json.Unmarshal(data, &raw)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	m := services.GetServerMarshaler()
	var out discoveryRequest
	for _, bytes := range raw.Proxies {
		proxy, err := m.UnmarshalServer([]byte(bytes), services.KindProxy)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		out.Proxies = append(out.Proxies, proxy)
	}
	out.TunnelID = raw.TunnelID
	out.Type = raw.Type
	return &out, nil
}
