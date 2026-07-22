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

	kubev1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/kube/v1"
	scopesv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/clientutils"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/itertools/stream"
	"github.com/gravitational/teleport/lib/scopes"
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
			scopes.QualifiedName{
				Scope: cluster.GetScope(),
				Name:  common.FormatResourceName(cluster, verbose),
			}.String(),

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
	sqn, err := scopes.ParseQualifiedName(ref.Name)
	if err != nil {
		sqn = scopes.QualifiedName{
			Name: ref.Name,
		}
	}
	clusters = FilterBySQNOrDiscoveredName(clusters, sqn)
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
	sqn := scopes.QualifiedName{
		Scope: cluster.GetScope(),
		Name:  cluster.GetName(),
	}
	if err := client.CreateKubernetesCluster(ctx, cluster); err != nil {
		if trace.IsAlreadyExists(err) {
			if !opts.Force {
				return trace.AlreadyExists("Kubernetes cluster %q already exists", cluster.GetName())
			}
			if err := client.UpdateKubernetesCluster(ctx, cluster); err != nil {
				return trace.Wrap(err)
			}
			fmt.Printf("Kubernetes cluster %q has been updated\n", sqn.String())
			return nil
		}
		return trace.Wrap(err)
	}
	fmt.Printf("Kubernetes cluster %q has been created\n", sqn.String())
	return nil
}

func deleteKubeCluster(ctx context.Context, client *authclient.Client, ref services.Ref) error {
	// TODO(okraport) DELETE IN v21.0.0, replace with regular Collect
	clusters, err := clientutils.CollectWithFallback(ctx, client.ListKubernetesClusters, client.GetKubernetesClusters)
	if err != nil {
		return trace.Wrap(err)
	}
	resDesc := "Kubernetes cluster"
	sqn, err := scopes.ParseQualifiedName(ref.Name)
	if err != nil {
		sqn = scopes.QualifiedName{
			Name: ref.Name,
		}
	}
	clusters = FilterBySQNOrDiscoveredName(clusters, sqn)
	name, err := GetOneResourceNameToDelete(clusters, ref, resDesc)
	if err != nil {
		return trace.Wrap(err)
	}
	if err := client.DeleteKubeCluster(ctx, kubev1.DeleteKubeClusterRequest_builder{
		Name: name,
	}.Build()); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("%s %q has been deleted\n", resDesc, name)
	return nil
}

func scopedKubeClusterHandler() ScopedHandler {
	return ScopedHandler{
		getHandler:    getScopedKubeCluster,
		createHandler: createScopedKubeCluster,
		deleteHandler: deleteScopedKubeCluster,
		updateHandler: updateScopedKubeCluster,
		description:   "A dynamic resource representing a Kubernetes cluster that can be accessed via a Kubernetes service.",
	}
}

func getScopedKubeCluster(ctx context.Context, client *authclient.Client, subKind string, sqn *scopes.QualifiedName, _ GetOpts) (Collection, error) {
	if subKind != "" {
		return nil, rejectSubKind(types.KindKubernetesCluster, subKind)
	}

	if sqn != nil {
		cluster, err := client.GetKubeCluster(ctx, kubev1.GetKubeClusterRequest_builder{
			Scope: sqn.Scope,
			Name:  sqn.Name,
		}.Build())
		if err != nil {
			return nil, trace.Wrap(err)
		}
		if cluster.GetScope() != sqn.Scope {
			return nil, scopeMismatchNotFound(types.KindKubernetesCluster, *sqn, cluster.GetScope())
		}
		return NewKubeClusterCollection([]types.KubeCluster{cluster}), nil
	}

	clusters, err := stream.Collect(clientutils.Resources(ctx, func(ctx context.Context, pageSize int, pageKey string) ([]types.KubeCluster, string, error) {
		return client.ListKubeClusters(ctx, kubev1.ListKubeClustersRequest_builder{
			PageSize:    int32(pageSize),
			PageToken:   pageKey,
			ScopeFilter: scopesv1.Filter_builder{Mode: scopesv1.Mode_MODE_ALL}.Build(),
		}.Build())
	}))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return NewKubeClusterCollection(clusters), nil
}

func createScopedKubeCluster(ctx context.Context, client *authclient.Client, raw services.UnknownResource, opts CreateOpts) error {
	cluster, err := services.UnmarshalKubeCluster(raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}

	sqn := scopes.QualifiedName{
		Scope: cluster.GetScope(),
		Name:  cluster.GetName(),
	}
	if err := client.CreateKubernetesCluster(ctx, cluster); err != nil {
		if trace.IsAlreadyExists(err) {
			if !opts.Force {
				return trace.AlreadyExists("Kubernetes cluster %q already exists", sqn.String())
			}
			if err := client.UpdateKubernetesCluster(ctx, cluster); err != nil {
				return trace.Wrap(err)
			}
			fmt.Printf("Kubernetes cluster %q has been updated\n", cluster.GetName())
			return nil
		}
		return trace.Wrap(err)
	}
	fmt.Printf("Kubernetes cluster %q has been created\n", sqn.String())
	return nil
}

func updateScopedKubeCluster(ctx context.Context, client *authclient.Client, raw services.UnknownResource, opts CreateOpts) error {
	cluster, err := services.UnmarshalKubeCluster(raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}

	if err := client.UpdateKubernetesCluster(ctx, cluster); err != nil {
		return trace.Wrap(err)
	}

	sqn := scopes.QualifiedName{
		Scope: cluster.GetScope(),
		Name:  cluster.GetName(),
	}
	fmt.Printf("%v %q has been updated\n", types.KindKubernetesCluster, sqn.String())
	return nil
}

func deleteScopedKubeCluster(ctx context.Context, client *authclient.Client, subKind string, sqn scopes.QualifiedName) error {
	if subKind != "" {
		return rejectSubKind(types.KindKubernetesCluster, subKind)
	}

	// Fetch first to verify scope before deleting.
	cluster, err := client.GetKubeCluster(ctx, kubev1.GetKubeClusterRequest_builder{
		Scope: sqn.Scope,
		Name:  sqn.Name,
	}.Build())
	if err != nil {
		return trace.Wrap(err)
	}

	if cluster.GetScope() != sqn.Scope {
		return scopeMismatchNotFound(types.KindScopedToken, sqn, cluster.GetScope())
	}

	if err := client.DeleteKubeCluster(ctx, kubev1.DeleteKubeClusterRequest_builder{
		Scope: sqn.Scope,
		Name:  sqn.Name,
	}.Build()); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf(
		"%v %q has been deleted\n",
		types.KindKubernetesCluster,
		sqn.String(),
	)
	return nil
}
