package reversetunnel

import (
	"encoding/json"

	"github.com/gravitational/teleport/lib/services"

	"github.com/gravitational/trace"
)

type discoveryRequest struct {
	Proxies []services.Server `json:"proxies"`
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
