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
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	apiutils "github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/services"
)

type remoteClusterCollection struct {
	remoteClusters []types.RemoteCluster
}

func (c *remoteClusterCollection) Resources() (r []types.Resource) {
	for _, resource := range c.remoteClusters {
		r = append(r, resource)
	}
	return r
}

func (c *remoteClusterCollection) WriteText(w io.Writer, verbose bool) error {
	t := asciitable.MakeTable([]string{"Name", "Status", "Last Heartbeat"})
	for _, cluster := range c.remoteClusters {
		lastHeartbeat := cluster.GetLastHeartbeat()
		t.AddRow([]string{cluster.GetName(), cluster.GetConnectionStatus(), formatLastHeartbeat(lastHeartbeat)})
	}
	_, err := t.AsBuffer().WriteTo(w)
	return trace.Wrap(err)
}

func formatLastHeartbeat(t time.Time) string {
	if t.IsZero() {
		return "not available"
	}
	return apiutils.HumanTimeFormat(t)
}

func remoteClusterHandler() Handler {
	return Handler{
		getHandler:    getRemoteCluster,
		deleteHandler: deleteRemoteCluster,
		description:   "Represents a Teleport cluster (leaf) trusting the current cluster (root).",
	}
}

func getRemoteCluster(ctx context.Context, client *authclient.Client, ref services.Ref, opts GetOpts) (Collection, error) {
	if ref.Name == "" {
		remoteClusters, err := client.GetRemoteClusters(ctx)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return &remoteClusterCollection{remoteClusters: remoteClusters}, nil
	}
	remoteCluster, err := client.GetRemoteCluster(ctx, ref.Name)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &remoteClusterCollection{remoteClusters: []types.RemoteCluster{remoteCluster}}, nil
}

func deleteRemoteCluster(ctx context.Context, client *authclient.Client, ref services.Ref) error {
	if err := client.DeleteRemoteCluster(ctx, ref.Name); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("remote cluster %q has been deleted\n", ref.Name)
	return nil
}
