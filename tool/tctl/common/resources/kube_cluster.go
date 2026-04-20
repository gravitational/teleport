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
	"github.com/gravitational/teleport/api/utils/clientutils"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/tool/common"
)

type kubeClusterCollection struct {
	clusters []types.KubeCluster
}

func NewKubeClusterCollection(clusters []types.KubeCluster) Collection {
	return &kubeClusterCollection{clusters: clusters}
}

func (c *kubeClusterCollection) Resources() (r []types.Resource) {
	for _, resource := range c.clusters {
		r = append(r, resource)
	}
	return r
}

// writeText formats the dynamic kube clusters into a table and writes them into w.
// Name          Labels
// ------------- ----------------------------------------------------------------------------------------------------------
// cluster1      region=eastus,resource-group=cluster1,subscription-id=subID
// cluster2      region=westeurope,resource-group=cluster2,subscription-id=subID
// cluster3      region=northcentralus,resource-group=cluster3,subscription-id=subID
// cluster4      owner=cluster4,region=southcentralus,resource-group=cluster4,subscription-id=subID
// If verbose is disabled, labels column can be truncated to fit into the console.
func (c *kubeClusterCollection) WriteText(w io.Writer, verbose bool) error {
	var rows [][]string
	for _, cluster := range c.clusters {
		labels := common.FormatLabels(cluster.GetAllLabels(), verbose)
		rows = append(rows, []string{
			common.FormatResourceName(cluster, verbose),
			labels,
		})
	}
	headers := []string{"Name", "Labels"}
	var t asciitable.Table
	if verbose {
		t = asciitable.MakeTable(headers, rows...)
	} else {
		t = asciitable.MakeTableWithTruncatedColumn(headers, rows, "Labels")
	}
	// stable sort by name.
	t.SortRowsBy([]int{0}, true)
	_, err := t.AsBuffer().WriteTo(w)
	return trace.Wrap(err)
}

func kubeClusterHandler() Handler {
	return Handler{
		getHandler:    getKubeCluster,
		createHandler: createKubeCluster,
		deleteHandler: deleteKubeCluster,
		description:   "A dynamic resource representing a Kubernetes cluster that can be accessed via a Kubernetes service.",
	}
}

func getKubeCluster(ctx context.Context, client *authclient.Client, ref services.Ref, opts GetOpts) (Collection, error) {
	// TODO(okraport) DELETE IN v21.0.0, replace with regular Collect
	clusters, err := clientutils.CollectWithFallback(ctx, client.ListKubernetesClusters, client.GetKubernetesClusters)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if ref.Name == "" {
		return NewKubeClusterCollection(clusters), nil
	}
	clusters = FilterByNameOrDiscoveredName(clusters, ref.Name)
	if len(clusters) == 0 {
		return nil, trace.NotFound("Kubernetes cluster %q not found", ref.Name)
	}
	return NewKubeClusterCollection(clusters), nil
}

func createKubeCluster(ctx context.Context, client *authclient.Client, raw services.UnknownResource, opts CreateOpts) error {
	cluster, err := services.UnmarshalKubeCluster(raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}
	if err := client.CreateKubernetesCluster(ctx, cluster); err != nil {
		if trace.IsAlreadyExists(err) {
			if !opts.Force {
				return trace.AlreadyExists("Kubernetes cluster %q already exists", cluster.GetName())
			}
			if err := client.UpdateKubernetesCluster(ctx, cluster); err != nil {
				return trace.Wrap(err)
			}
			fmt.Printf("Kubernetes cluster %q has been updated\n", cluster.GetName())
			return nil
		}
		return trace.Wrap(err)
	}
	fmt.Printf("Kubernetes cluster %q has been created\n", cluster.GetName())
	return nil
}

func deleteKubeCluster(ctx context.Context, client *authclient.Client, ref services.Ref) error {
	// TODO(okraport) DELETE IN v21.0.0, replace with regular Collect
	clusters, err := clientutils.CollectWithFallback(ctx, client.ListKubernetesClusters, client.GetKubernetesClusters)
	if err != nil {
		return trace.Wrap(err)
	}
	resDesc := "Kubernetes cluster"
	clusters = FilterByNameOrDiscoveredName(clusters, ref.Name)
	name, err := GetOneResourceNameToDelete(clusters, ref, resDesc)
	if err != nil {
		return trace.Wrap(err)
	}
	if err := client.DeleteKubernetesCluster(ctx, name); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("%s %q has been deleted\n", resDesc, name)
	return nil
}
