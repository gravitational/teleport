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

package authclient

import (
	"context"
	"encoding/json"
	"net/url"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
)

// httpfallback.go holds endpoints that have been converted to gRPC
// but still need http fallback logic in the old client.

// ValidateTrustedCluster is called by the proxy on behalf of a cluster that
// wishes to join another as a leaf cluster.
func (c *Client) ValidateTrustedCluster(ctx context.Context, validateRequest *ValidateTrustedClusterRequest) (*ValidateTrustedClusterResponse, error) {
	protoReq, err := validateRequest.ToProto()
	if err != nil {
		return nil, trace.Wrap(err, "converting native ValidateTrustedClusterRequest to proto")
	}
	protoResp, err := c.APIClient.ValidateTrustedCluster(ctx, protoReq)
	if err != nil {
		if trace.IsNotImplemented(err) {
			return c.HTTPClient.validateTrustedCluster(ctx, validateRequest)
		}
		return nil, trace.Wrap(err, "calling ValidateTrustedCluster on gRPC client")
	}
	return ValidateTrustedClusterResponseFromProto(protoResp), nil
}

// TODO(noah): DELETE IN 21.0.0
func (c *HTTPClient) validateTrustedCluster(ctx context.Context, validateRequest *ValidateTrustedClusterRequest) (*ValidateTrustedClusterResponse, error) {
	validateRequestRaw, err := validateRequest.ToRaw()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	out, err := c.PostJSON(ctx, c.Endpoint("trustedclusters", "validate"), validateRequestRaw)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var validateResponseRaw ValidateTrustedClusterResponseRaw
	err = json.Unmarshal(out.Bytes(), &validateResponseRaw)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	validateResponse, err := validateResponseRaw.ToNative()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return validateResponse, nil
}

// GetAuthServers returns the list of auth servers registered in the cluster.
//
// Deprecated: Prefer paginated variant [APIClient.ListAuthServers].
//
// TODO(kiosion): DELETE IN 21.0.0
func (c *HTTPClient) GetAuthServers() ([]types.Server, error) {
	out, err := c.Get(context.TODO(), c.Endpoint("authservers"), url.Values{})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var items []json.RawMessage
	if err := json.Unmarshal(out.Bytes(), &items); err != nil {
		return nil, trace.Wrap(err)
	}
	re := make([]types.Server, len(items))
	for i, raw := range items {
		server, err := services.UnmarshalServer(raw, types.KindAuthServer)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		re[i] = server
	}
	return re, nil
}

// GetProxies returns the list of auth servers registered in the cluster.
//
// Deprecated: Prefer paginated variant [APIClient.ListProxyServers].
//
// TODO(kiosion): DELETE IN 21.0.0
func (c *HTTPClient) GetProxies() ([]types.Server, error) {
	out, err := c.Get(context.TODO(), c.Endpoint("proxies"), url.Values{})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var items []json.RawMessage
	if err := json.Unmarshal(out.Bytes(), &items); err != nil {
		return nil, trace.Wrap(err)
	}
	re := make([]types.Server, len(items))
	for i, raw := range items {
		server, err := services.UnmarshalServer(raw, types.KindProxy)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		re[i] = server
	}
	return re, nil
}
