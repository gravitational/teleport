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
func marshalDiscoveryRequest(proxies []types.Server) ([]byte, error) {
	out := discoveryRequestRaw{
		Proxies: make([]json.RawMessage, 0, len(proxies)),
	}
	for _, p := range proxies {
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

// unmarshalDiscoveryRequest exercises the legacy method of unmarshaling a
// discoveryRequest, returning a slice with the names of the unmarshaled
// types.Server resources.
func unmarshalDiscoveryRequest(data []byte) ([]string, error) {
	if len(data) == 0 {
		return nil, trace.BadParameter("missing payload in discovery request")
	}

	var raw discoveryRequestRaw
	if err := utils.FastUnmarshal(data, &raw); err != nil {
		return nil, trace.Wrap(err)
	}

	out := make([]string, 0, len(raw.Proxies))
	for _, bytes := range raw.Proxies {
		var v struct {
			Version string `json:"version"`
		}
		if err := utils.FastUnmarshal(bytes, &v); err != nil {
			return nil, trace.Wrap(err)
		}

		if v.Version != types.V2 {
			return nil, trace.BadParameter("server resource version %q is not supported", v.Version)
		}

		proxy, err := services.UnmarshalServer(bytes, types.KindProxy)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		out = append(out, proxy.GetName())
	}

	return out, nil
}

func TestDiscoveryRequestMarshalling(t *testing.T) {
	const proxyCount = 10

	// prepare some random proxies for the discovery request
	proxies := make([]types.Server, 0, proxyCount)
	for i := 0; i < proxyCount; i++ {
		p, err := types.NewServer(uuid.New().String(), types.KindProxy, types.ServerSpecV2{})
		require.NoError(t, err)
		proxies = append(proxies, p)
	}

	// create the request
	var req discoveryRequest
	req.SetProxies(proxies)

	// test marshaling the request with the legacy mechanism and unmarshaling
	// with the new mechanism
	t.Run("marshal=legacy unmarshal=new", func(t *testing.T) {
		payload, err := marshalDiscoveryRequest(proxies)
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

		require.Empty(t, cmp.Diff(req.ProxyNames(), got))
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
		payload, err := marshalDiscoveryRequest(proxies)
		require.NoError(t, err)

		got, err := unmarshalDiscoveryRequest(payload)
		require.NoError(t, err)

		require.Empty(t, cmp.Diff(req.ProxyNames(), got))
	})
}
