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
	"strings"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
)

// httpfallback.go holds endpoints that have been converted to gRPC
// but still need http fallback logic in the old client.

// GetReverseTunnels returns the list of created reverse tunnels
// TODO(noah): DELETE IN 18.0.0
func (c *Client) GetReverseTunnels(ctx context.Context) ([]types.ReverseTunnel, error) {
	var rcs []types.ReverseTunnel
	pageToken := ""
	for {
		page, nextToken, err := c.APIClient.ListReverseTunnels(ctx, 0, pageToken)
		if err != nil {
			if trace.IsNotImplemented(err) {
				return c.getReverseTunnelsLegacy(ctx)
			}
			return nil, trace.Wrap(err)
		}
		rcs = append(rcs, page...)
		if nextToken == "" {
			return rcs, nil
		}
		pageToken = nextToken
	}
}

func (c *Client) getReverseTunnelsLegacy(ctx context.Context) ([]types.ReverseTunnel, error) {
	out, err := c.Get(ctx, c.Endpoint("reversetunnels"), url.Values{})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var items []json.RawMessage
	if err := json.Unmarshal(out.Bytes(), &items); err != nil {
		return nil, trace.Wrap(err)
	}
	tunnels := make([]types.ReverseTunnel, len(items))
	for i, raw := range items {
		tunnel, err := services.UnmarshalReverseTunnel(raw)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		tunnels[i] = tunnel
	}
	return tunnels, nil
}

// UpsertReverseTunnel upserts a reverse tunnel
// TODO: DELETE IN 18.0.0
func (c *Client) UpsertReverseTunnel(ctx context.Context, tunnel types.ReverseTunnel) error {
	_, err := c.APIClient.UpsertReverseTunnel(ctx, tunnel)
	if err == nil {
		return nil
	}
	if !trace.IsNotImplemented(err) {
		return trace.Wrap(err)
	}
	return c.upsertReverseTunnelLegacy(context.Background(), tunnel)
}

type upsertReverseTunnelRawReq struct {
	ReverseTunnel json.RawMessage `json:"reverse_tunnel"`
}

func (c *Client) upsertReverseTunnelLegacy(ctx context.Context, tunnel types.ReverseTunnel) error {
	data, err := services.MarshalReverseTunnel(tunnel)
	if err != nil {
		return trace.Wrap(err)
	}
	args := &upsertReverseTunnelRawReq{
		ReverseTunnel: data,
	}
	_, err = c.PostJSON(ctx, c.Endpoint("reversetunnels"), args)
	return trace.Wrap(err)
}

// DeleteReverseTunnel deletes reverse tunnel by name
// TODO(noah): DELETE IN 18.0.0
func (c *Client) DeleteReverseTunnel(ctx context.Context, name string) error {
	err := c.APIClient.DeleteReverseTunnel(ctx, name)
	if err == nil {
		return nil
	}
	if !trace.IsNotImplemented(err) {
		return trace.Wrap(err)
	}
	return c.deleteReverseTunnelLegacy(ctx, name)
}

func (c *Client) deleteReverseTunnelLegacy(ctx context.Context, domainName string) error {
	// this is to avoid confusing error in case if domain empty for example
	// HTTP route will fail producing generic not found error
	// instead we catch the error here
	if strings.TrimSpace(domainName) == "" {
		return trace.BadParameter("empty domain name")
	}
	_, err := c.Delete(ctx, c.Endpoint("reversetunnels", domainName))
	return trace.Wrap(err)
}
