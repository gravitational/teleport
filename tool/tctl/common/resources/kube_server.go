// Teleport
// Copyright (C) 2026 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package resources

import (
	"context"
	"fmt"
	"io"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/tool/common"
)

type kubeServerCollection struct {
	servers []types.KubeServer
}

func NewKubeServerCollection(servers []types.KubeServer) Collection {
	return &kubeServerCollection{servers: servers}
}

func (c *kubeServerCollection) Resources() (r []types.Resource) {
	for _, resource := range c.servers {
		r = append(r, resource)
	}
	return r
}

func (c *kubeServerCollection) WriteText(w io.Writer, verbose bool) error {
	var rows [][]string
	for _, server := range c.servers {
		kube := server.GetCluster()
		if kube == nil {
			continue
		}
		labels := common.FormatLabels(kube.GetAllLabels(), verbose)
		rows = append(rows, []string{
			common.FormatResourceName(kube, verbose),
			labels,
			server.GetTeleportVersion(),
		})

	}
	headers := []string{"Cluster", "Labels", "Version"}
	var t asciitable.Table
	if verbose {
		t = asciitable.MakeTable(headers, rows...)
	} else {
		t = asciitable.MakeTableWithTruncatedColumn(headers, rows, "Labels")
	}
	// stable sort by cluster name.
	t.SortRowsBy([]int{0}, true)

	_, err := t.AsBuffer().WriteTo(w)
	return trace.Wrap(err)
}

func kubeServerHandler() Handler {
	return Handler{
		getHandler:    getKubeServer,
		deleteHandler: deleteKubeServer,
		description:   "Represents a Kubernetes service instance that proxies access to Kubernetes clusters.",
	}
}

func getKubeServer(ctx context.Context, client *authclient.Client, ref services.Ref, opts GetOpts) (Collection, error) {
	servers, err := client.GetKubernetesServers(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if ref.Name == "" {
		return NewKubeServerCollection(servers), nil
	}
	altNameFn := func(r types.KubeServer) string {
		return r.GetHostname()
	}
	servers = FilterByNameOrDiscoveredName(servers, ref.Name, altNameFn)
	if len(servers) == 0 {
		return nil, trace.NotFound("Kubernetes server %q not found", ref.Name)
	}
	return NewKubeServerCollection(servers), nil
}

func deleteKubeServer(ctx context.Context, client *authclient.Client, ref services.Ref) error {
	servers, err := client.GetKubernetesServers(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	resDesc := "Kubernetes server"
	servers = FilterByNameOrDiscoveredName(servers, ref.Name)
	name, err := GetOneResourceNameToDelete(servers, ref, resDesc)
	if err != nil {
		return trace.Wrap(err)
	}
	for _, s := range servers {
		err := client.DeleteKubernetesServer(ctx, s.GetHostID(), name)
		if err != nil {
			return trace.Wrap(err)
		}
	}
	fmt.Printf("%s %q has been deleted\n", resDesc, name)
	return nil
}
