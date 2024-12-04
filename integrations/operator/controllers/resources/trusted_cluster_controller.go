/*
 * Teleport
 * Copyright (C) 2024 Gravitational, Inc.
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

	"github.com/gravitational/trace"
	kclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/types"
	resourcesv1 "github.com/gravitational/teleport/integrations/operator/apis/resources/v1"
	"github.com/gravitational/teleport/integrations/operator/controllers"
	"github.com/gravitational/teleport/integrations/operator/controllers/reconcilers"
)

// trustedClusterClient implements TeleportResourceClient and offers CRUD
// methods needed to reconcile trusted_clusters.
type trustedClusterClient struct {
	teleportClient *client.Client
}

// Get gets the Teleport trusted_cluster of a given name.
func (r trustedClusterClient) Get(ctx context.Context, name string) (types.TrustedCluster, error) {
	trustedCluster, err := r.teleportClient.GetTrustedCluster(ctx, name)
	return trustedCluster, trace.Wrap(err)
}

// Create creates a Teleport trusted_cluster.
func (r trustedClusterClient) Create(ctx context.Context, trustedCluster types.TrustedCluster) error {
	_, err := r.teleportClient.UpsertTrustedCluster(ctx, trustedCluster)
	return trace.Wrap(err)
}

// Update updates a Teleport trusted_cluster.
func (r trustedClusterClient) Update(ctx context.Context, trustedCluster types.TrustedCluster) error {
	_, err := r.teleportClient.UpsertTrustedCluster(ctx, trustedCluster)
	return trace.Wrap(err)
}

// Delete deletes a Teleport trusted_cluster.
func (r trustedClusterClient) Delete(ctx context.Context, name string) error {
	return trace.Wrap(r.teleportClient.DeleteTrustedCluster(ctx, name))
}

// NewTrustedClusterV2Reconciler instantiates a new Kubernetes controller reconciling trusted_cluster v2 resources
func NewTrustedClusterV2Reconciler(client kclient.Client, tClient *client.Client) (controllers.Reconciler, error) {
	trustedClusterClient := &trustedClusterClient{
		teleportClient: tClient,
	}

	resourceReconciler, err := reconcilers.NewTeleportResourceWithoutLabelsReconciler[types.TrustedCluster, *resourcesv1.TeleportTrustedClusterV2](
		client,
		trustedClusterClient,
	)

	return resourceReconciler, trace.Wrap(err, "building teleport resource reconciler")
}
