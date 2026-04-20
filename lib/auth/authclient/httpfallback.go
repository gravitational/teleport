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

	presencev1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/presence/v1"
	trustpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/trust/v1"
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

// UpsertTunnelConnection creates or updates a tunnel connection record.
//
// TODO(strideynet): DELETE IN v20.0.0
func (c *Client) UpsertTunnelConnection(ctx context.Context, conn types.TunnelConnection) error {
	connV2, ok := conn.(*types.TunnelConnectionV2)
	if !ok {
		return trace.BadParameter("unsupported tunnel connection type %T", conn)
	}
	_, err := c.TrustClient().UpsertTunnelConnection(ctx, trustpb.UpsertTunnelConnectionRequest_builder{
		TunnelConnection: connV2,
	}.Build())
	if err != nil {
		if trace.IsNotImplemented(err) {
			return trace.Wrap(c.HTTPClient.upsertTunnelConnection(ctx, conn))
		}
		return trace.Wrap(err)
	}
	return nil
}

// TODO(strideynet): DELETE IN v20.0.0
func (c *HTTPClient) upsertTunnelConnection(ctx context.Context, conn types.TunnelConnection) error {
	data, err := services.MarshalTunnelConnection(conn)
	if err != nil {
		return trace.Wrap(err)
	}
	args := &struct {
		TunnelConnection json.RawMessage `json:"tunnel_connection"`
	}{
		TunnelConnection: data,
	}
	_, err = c.PostJSON(ctx, c.Endpoint("tunnelconnections"), args)
	return trace.Wrap(err)
}

// DeleteTunnelConnection removes a tunnel connection by cluster and connection name.
//
// TODO(strideynet): DELETE IN v20.0.0
func (c *Client) DeleteTunnelConnection(ctx context.Context, clusterName, connName string) error {
	_, err := c.TrustClient().DeleteTunnelConnection(ctx, trustpb.DeleteTunnelConnectionRequest_builder{
		ClusterName:    clusterName,
		ConnectionName: connName,
	}.Build())
	if err != nil {
		if trace.IsNotImplemented(err) {
			return trace.Wrap(c.HTTPClient.deleteTunnelConnection(ctx, clusterName, connName))
		}
		return trace.Wrap(err)
	}
	return nil
}

// TODO(strideynet): DELETE IN v20.0.0
func (c *HTTPClient) deleteTunnelConnection(ctx context.Context, clusterName, connName string) error {
	if clusterName == "" {
		return trace.BadParameter("missing parameter cluster name")
	}
	if connName == "" {
		return trace.BadParameter("missing parameter connection name")
	}
	_, err := c.Delete(ctx, c.Endpoint("tunnelconnections", clusterName, connName))
	return trace.Wrap(err)
}

// GetTunnelConnections returns all tunnel connections for a given cluster.
//
// TODO(noah): DELETE IN 21.0.0 — inline the gRPC page-loop directly once the
// HTTP fallback is removed; keep the method signature.
func (c *Client) GetTunnelConnections(ctx context.Context, clusterName string) ([]types.TunnelConnection, error) {
	return c.listAllTunnelConnections(ctx, clusterName)
}

// GetAllTunnelConnections returns all tunnel connections across all clusters.
//
// TODO(noah): DELETE IN 21.0.0
func (c *Client) GetAllTunnelConnections(ctx context.Context) ([]types.TunnelConnection, error) {
	return c.listAllTunnelConnections(ctx, "")
}

// TODO(noah): DELETE IN 21.0.0
func (c *Client) listAllTunnelConnections(ctx context.Context, clusterName string) ([]types.TunnelConnection, error) {
	var filter *trustpb.ListTunnelConnectionsFilter
	if clusterName != "" {
		filter = &trustpb.ListTunnelConnectionsFilter{ClusterName: clusterName}
	}
	var all []types.TunnelConnection
	var pageToken string
	for {
		page, next, err := c.ListTunnelConnections(ctx, 0, pageToken, filter)
		if err != nil {
			if trace.IsNotImplemented(err) {
				if clusterName != "" {
					return c.HTTPClient.getTunnelConnectionsLegacy(ctx, clusterName)
				}
				return c.HTTPClient.getAllTunnelConnectionsLegacy(ctx)
			}
			return nil, trace.Wrap(err)
		}
		all = append(all, page...)
		if next == "" {
			return all, nil
		}
		pageToken = next
	}
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

// UpsertProxyServerWithoutReturn registers a proxy server heartbeat. It calls
// the gRPC PresenceService and falls back to the legacy HTTP endpoint if the
// server does not yet implement the gRPC RPC. The upserted proxy server is not
// returned because the HTTP fallback path cannot provide it; once the fallback
// is removed in v20 this can be replaced with a method that returns the
// upserted server.
//
// TODO(noah): DELETE IN v20.0.0
func (c *Client) UpsertProxyServerWithoutReturn(ctx context.Context, s types.Server) error {
	serverV2, ok := s.(*types.ServerV2)
	if !ok {
		return trace.BadParameter("unsupported proxy server type %T", s)
	}
	_, err := c.APIClient.PresenceServiceClient().UpsertProxyServer(ctx, presencev1.UpsertProxyServerRequest_builder{
		Server: serverV2,
	}.Build())
	if err == nil {
		return nil
	}
	if !trace.IsNotImplemented(err) {
		return trace.Wrap(err)
	}
	return c.HTTPClient.upsertProxyServerLegacy(ctx, s)
}

// upsertProxyServerLegacy registers a proxy server heartbeat via the legacy
// HTTP endpoint.
//
// TODO(noah): DELETE IN v20.0.0
func (c *HTTPClient) upsertProxyServerLegacy(ctx context.Context, s types.Server) error {
	data, err := services.MarshalServer(s)
	if err != nil {
		return trace.Wrap(err)
	}
	args := &upsertServerRawReq{
		Server: data,
	}
	_, err = c.PostJSON(ctx, c.Endpoint("proxies"), args)
	return trace.Wrap(err)
}

// DeleteProxyServer deletes a proxy server heartbeat by name. It calls the
// gRPC PresenceService and falls back to the legacy HTTP endpoint if the
// server does not yet implement the gRPC RPC.
//
// TODO(noah): DELETE IN v20.0.0
func (c *Client) DeleteProxyServer(ctx context.Context, name string) error {
	_, err := c.APIClient.PresenceServiceClient().DeleteProxyServer(ctx, presencev1.DeleteProxyServerRequest_builder{
		Name: name,
	}.Build())
	if err == nil {
		return nil
	}
	if !trace.IsNotImplemented(err) {
		return trace.Wrap(err)
	}
	return c.HTTPClient.deleteProxyServerLegacy(ctx, name)
}

// deleteProxyServerLegacy deletes proxy by name via the legacy HTTP endpoint.
//
// TODO(noah): DELETE IN v20.0.0
func (c *HTTPClient) deleteProxyServerLegacy(ctx context.Context, name string) error {
	if name == "" {
		return trace.BadParameter("missing parameter name")
	}
	_, err := c.Delete(ctx, c.Endpoint("proxies", name))
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
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
