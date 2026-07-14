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

package common

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/accessrequest"
	apiclient "github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proto"
	componentfeaturesv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/componentfeatures/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/componentfeatures"
)

// ClusterSupportClient is the subset of the auth client needed to verify
// constraint support within one cluster: its Auth/Proxy presence and its
// unified resources.
type ClusterSupportClient interface {
	componentfeatures.AuthProxyServersLister
	ListUnifiedResources(ctx context.Context, req *proto.ListUnifiedResourcesRequest) (*proto.ListUnifiedResourcesResponse, error)
}

// ClusterClientGetter returns a client for verifying constraint support of
// resources in the named cluster, or nil to skip per-cluster verification
// (e.g. tctl, which cannot dial leaf clusters; Auth-side validation and
// fail-closed enforcement still apply there).
type ClusterClientGetter func(ctx context.Context, clusterName string) (ClusterSupportClient, error)

// VerifyConstraintSupport checks that every constrained resource in raids can
// be served with its constraints enforced, mirroring the Web UI's
// feature-advertisement gating (RFD 0230): the features advertised by the
// root cluster's Auth and Proxy servers are intersected with those of the
// agent(s) serving each constrained resource, and RESOURCE_CONSTRAINTS_V1
// must be present in every intersection. Resources in leaf clusters are
// fetched from their own cluster, and the leaf's Auth and Proxy presence is
// intersected as well, since those components also sit on the access path.
// Requests without constraints skip the check.
func VerifyConstraintSupport(ctx context.Context, logger *slog.Logger, rootClusterName string, rootClt componentfeatures.AuthProxyServersLister, cltForCluster ClusterClientGetter, raids []types.ResourceAccessID) error {
	byCluster := make(map[string][]types.ResourceAccessID)
	for _, r := range raids {
		if r.GetConstraints() != nil {
			cluster := r.GetResourceID().ClusterName
			byCluster[cluster] = append(byCluster[cluster], r)
		}
	}
	if len(byCluster) == 0 {
		return nil
	}

	rootFeatures := componentfeatures.GetClusterAuthProxyServerFeatures(ctx, rootClt, logger)
	if !componentfeatures.InAllSets(componentfeatures.FeatureResourceConstraintsV1, rootFeatures) {
		return trace.BadParameter("this cluster's Auth or Proxy servers do not support resource constraints; retry without constraints, or upgrade the cluster")
	}

	for clusterName, constrained := range byCluster {
		clt, err := cltForCluster(ctx, clusterName)
		if err != nil {
			return trace.Wrap(err, "connecting to cluster %q to verify constraint support", clusterName)
		}
		if clt == nil {
			continue
		}

		clusterFeatures := rootFeatures
		if clusterName != rootClusterName {
			// A leaf's Auth and Proxy servers also sit on the access path for
			// its resources, so their advertised support is required too.
			leafFeatures := componentfeatures.GetClusterAuthProxyServerFeatures(ctx, clt, logger)
			if !componentfeatures.InAllSets(componentfeatures.FeatureResourceConstraintsV1, leafFeatures) {
				return trace.BadParameter("cluster %q's Auth or Proxy servers do not support resource constraints; retry without constraints, or upgrade that cluster", clusterName)
			}
			clusterFeatures = componentfeatures.Intersect(rootFeatures, leafFeatures)
		}

		for _, r := range constrained {
			id := r.GetResourceID()
			enriched, err := apiclient.GetAllUnifiedResources(ctx, clt, &proto.ListUnifiedResourcesRequest{
				Kinds:               []string{id.Kind},
				PredicateExpression: fmt.Sprintf("name == %q", id.Name),
				UseSearchAsRoles:    true,
			})
			if err != nil {
				return trace.Wrap(err, "fetching resource %q to verify constraint support", types.ResourceIDToString(id))
			}

			// One resource may be served by several agents (e.g. HA app
			// servers); any of them may serve the connection, so all must
			// support constraints.
			sets := []*componentfeaturesv1.ComponentFeatures{clusterFeatures}
			found := false
			for _, er := range enriched {
				leaf, err := accessrequest.MapListResourcesResultToLeafResource(er.ResourceWithLabels, id.Kind)
				if err != nil || leaf.GetName() != id.Name {
					continue
				}
				found = true
				vc, ok := er.ResourceWithLabels.(interface {
					GetTeleportVersion() string
					GetComponentFeatures() *componentfeaturesv1.ComponentFeatures
				})
				if !ok {
					// No feature advertisement on this resource type: fail closed.
					sets = append(sets, nil)
					continue
				}
				sets = append(sets, componentfeatures.GetEffectiveServerFeatures(vc))
			}
			if !found {
				return trace.NotFound("resource %q was not found; cannot verify constraint support", types.ResourceIDToString(id))
			}
			if !componentfeatures.InAllSets(componentfeatures.FeatureResourceConstraintsV1, sets...) {
				return trace.BadParameter("resource %q does not support the requested constraints (its agent or this cluster's components are too old); retry without constraints", types.ResourceIDToString(id))
			}
		}
	}
	return nil
}
