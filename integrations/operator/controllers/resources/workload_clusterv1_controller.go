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

	"github.com/gravitational/trace"
	kclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gravitational/teleport/api/client"
	workloadclusterv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadcluster/v1"
	resourcesv1 "github.com/gravitational/teleport/integrations/operator/apis/resources/v1"
	"github.com/gravitational/teleport/integrations/operator/controllers"
	"github.com/gravitational/teleport/integrations/operator/controllers/reconcilers"
)

// TODO(dustin.specker): figure out how to test this controller
// The workload cluster service requires running in an enterprise Teleport instance on
// Teleport Cloud with the WorkloadClusters entitlement enabled.

// TODO(dustin.specker): improve Teleport controller design to support setting
// status fields on the Custom Resources based on fields found in the Teleport Resource.

// workloadClusterClient implements TeleportResourceClient and offers CRUD
// methods needed to reconcile WorkloadCluster
type workloadClusterClient struct {
	teleportClient *client.Client
}

// Get gets the Teleport WorkloadCluster of a given name
func (l workloadClusterClient) Get(
	ctx context.Context, name string,
) (*workloadclusterv1.WorkloadCluster, error) {
	resp, err := l.teleportClient.
		WorkloadClustersClient().
		GetWorkloadCluster(
			ctx, &workloadclusterv1.GetWorkloadClusterRequest{Name: name},
		)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return resp, nil
}

// Create creates a Teleport WorkloadCluster
func (l workloadClusterClient) Create(
	ctx context.Context, resource *workloadclusterv1.WorkloadCluster,
) error {
	_, err := l.teleportClient.
		WorkloadClustersClient().
		CreateWorkloadCluster(
			ctx,
			&workloadclusterv1.CreateWorkloadClusterRequest{
				Cluster: resource,
			},
		)

	// TODO(dustin.specker): support requeuing resources to check for status updates
	// from Workload Cluster service/Teleport Cloud

	return trace.Wrap(err)
}

// Update updates a Teleport WorkloadCluster
func (l workloadClusterClient) Update(
	ctx context.Context, resource *workloadclusterv1.WorkloadCluster,
) error {
	_, err := l.teleportClient.
		WorkloadClustersClient().
		UpsertWorkloadCluster(
			ctx,
			&workloadclusterv1.UpsertWorkloadClusterRequest{
				Cluster: resource,
			},
		)
	return trace.Wrap(err)
}

// Delete deletes a Teleport WorkloadCluster
func (l workloadClusterClient) Delete(ctx context.Context, name string) error {
	_, err := l.teleportClient.
		WorkloadClustersClient().
		DeleteWorkloadCluster(
			ctx, &workloadclusterv1.DeleteWorkloadClusterRequest{Name: name},
		)
	return trace.Wrap(err)
}

// NewWorkloadClusterV1Reconciler instantiates a new Kubernetes controller
// reconciling WorkloadCluster resources
func NewWorkloadClusterV1Reconciler(client kclient.Client, tClient *client.Client) (controllers.Reconciler, error) {
	workloadClusterClient := &workloadClusterClient{
		teleportClient: tClient,
	}

	resourceReconciler, err := reconcilers.NewTeleportResource153Reconciler[
		*workloadclusterv1.WorkloadCluster, *resourcesv1.TeleportWorkloadClusterV1,
	](
		client,
		workloadClusterClient,
	)

	return resourceReconciler, trace.Wrap(err, "building teleport resource reconciler")
}
