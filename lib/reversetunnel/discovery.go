package reversetunnel

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"
)

type discoveryRequest struct {
	ClusterName string            `json:"-"`
	ClusterAddr utils.NetAddr     `json:"-"`
	Proxies     []services.Server `json:"proxies"`
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
	return agentKey{domainName: r.ClusterName, addr: r.ClusterAddr}
}

func (r discoveryRequest) String() string {
	return fmt.Sprintf("discovery request, cluster name: %v, address: %v, proxies: %v",
		r.ClusterName, r.ClusterAddr, Proxies(r.Proxies))
}

type discoveryRequestRaw struct {
	Proxies []json.RawMessage `json:"proxies"`
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
	return &out, nil
}
