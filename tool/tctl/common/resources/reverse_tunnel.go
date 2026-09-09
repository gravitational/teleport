/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package resources

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/clientutils"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/itertools/stream"
	"github.com/gravitational/teleport/lib/services"
)

type reverseTunnelCollection struct {
	tunnels []types.ReverseTunnel
}

func (c *reverseTunnelCollection) Resources() (res []types.Resource) {
	for _, resource := range c.tunnels {
		res = append(res, resource)
	}
	return res
}

func (c *reverseTunnelCollection) WriteText(w io.Writer, verbose bool) error {
	t := asciitable.MakeTable([]string{"Cluster Name", "Dial Addresses"})
	for _, tunnel := range c.tunnels {
		t.AddRow([]string{
			tunnel.GetClusterName(), strings.Join(tunnel.GetDialAddrs(), ","),
		})
	}
	_, err := t.AsBuffer().WriteTo(w)
	return trace.Wrap(err)
}

func reverseTunnelHandler() Handler {
	return Handler{
		getHandler:    getReverseTunnel,
		deleteHandler: deleteReverseTunnel,
		description:   "Represents a reverse tunnel from a trusted cluster. Used for diagnostics.",
	}
}

func getReverseTunnel(ctx context.Context, client *authclient.Client, ref services.Ref, opts GetOpts) (Collection, error) {
	if ref.Name != "" {
		return nil, trace.BadParameter("reverse tunnel cannot be searched by name")
	}

	tunnels, err := stream.Collect(clientutils.Resources(ctx, client.ListReverseTunnels))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &reverseTunnelCollection{tunnels: tunnels}, nil
}

func deleteReverseTunnel(ctx context.Context, client *authclient.Client, ref services.Ref) error {
	if err := client.DeleteReverseTunnel(ctx, ref.Name); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("reverse tunnel %v has been deleted\n", ref.Name)
	return nil
}
