// Copyright 2022 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package reversetunnel

import (
	"encoding/json"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
)

// discoveryRequestRaw is the legacy type that was used
// as the payload for discoveryRequests. It exists
// here for the sake of ensuring backward compatibility.
type discoveryRequestRaw struct {
	ClusterName string            `json:"cluster_name"`
	Type        string            `json:"type"`
	Proxies     []json.RawMessage `json:"proxies"`
}

// marshalDiscoveryRequest is the legacy method of marshaling a discoveryRequest
func marshalDiscoveryRequest(req discoveryRequest) ([]byte, error) {
	out := discoveryRequestRaw{
		Proxies: make([]json.RawMessage, 0, len(req.Proxies)),
	}
	for _, p := range req.Proxies {
		// Clone the server value to avoid a potential race
		// since the proxies are shared.
		// Marshaling attempts to enforce defaults which modifies
		// the original value.
		p = p.DeepCopy()
		data, err := services.MarshalServer(p)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		out.Proxies = append(out.Proxies, data)
	}

	return json.Marshal(out)
}

// unmarshalDiscoveryRequest is the legacy method of unmarshaling a discoveryRequest
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

	return &out, nil
}

func TestDiscoveryRequestMarshalling(t *testing.T) {
	const proxyCount = 10

	// create a discovery request
	req := discoveryRequest{
		Proxies: make([]types.Server, 0, proxyCount),
	}

	// populate the proxies
	for i := 0; i < proxyCount; i++ {
		p, err := types.NewServer(uuid.New().String(), types.KindProxy, types.ServerSpecV2{})
		require.NoError(t, err)
		req.Proxies = append(req.Proxies, p)
	}

	// test marshaling the request with the legacy mechanism and unmarshaling
	// with the new mechanism
	t.Run("marshal=legacy unmarshal=new", func(t *testing.T) {
		payload, err := marshalDiscoveryRequest(req)
		require.NoError(t, err)

		var got discoveryRequest
		require.NoError(t, json.Unmarshal(payload, &got))

		require.Empty(t, cmp.Diff(req.ProxyNames(), got.ProxyNames()))
	})

	// test marshaling the request with the new mechanism and unmarshaling
	// with the legacy mechanism
	t.Run("marshal=new unmarshal=legacy", func(t *testing.T) {
		payload, err := json.Marshal(req)
		require.NoError(t, err)

		got, err := unmarshalDiscoveryRequest(payload)
		require.NoError(t, err)

		require.Empty(t, cmp.Diff(req.ProxyNames(), got.ProxyNames()))
	})

	// test marshaling and unmarshaling the request with the new mechanism
	t.Run("marshal=new unmarshal=new", func(t *testing.T) {
		payload, err := json.Marshal(req)
		require.NoError(t, err)

		var got discoveryRequest
		require.NoError(t, json.Unmarshal(payload, &got))

		require.Empty(t, cmp.Diff(req.ProxyNames(), got.ProxyNames()))
	})

	// test marshaling and unmarshaling the request with the legacy mechanism
	t.Run("marshal=legacy unmarshal=legacy", func(t *testing.T) {
		payload, err := marshalDiscoveryRequest(req)
		require.NoError(t, err)

		got, err := unmarshalDiscoveryRequest(payload)
		require.NoError(t, err)

		require.Empty(t, cmp.Diff(req.ProxyNames(), got.ProxyNames()))
	})
}
