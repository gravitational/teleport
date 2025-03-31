/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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
	workloadidentityv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadidentity/v1"
	resourcesv1 "github.com/gravitational/teleport/integrations/operator/apis/resources/v1"
	"github.com/gravitational/teleport/integrations/operator/controllers"
	"github.com/gravitational/teleport/integrations/operator/controllers/reconcilers"
)

// workloadIdentityClient implements TeleportResourceClient and offers CRUD
// methods needed to reconcile WorkloadIdentity
type workloadIdentityClient struct {
	teleportClient *client.Client
}

// Get gets the Teleport WorkloadIdentity of a given name
func (l workloadIdentityClient) Get(
	ctx context.Context, name string,
) (*workloadidentityv1.WorkloadIdentity, error) {
	resp, err := l.teleportClient.
		WorkloadIdentityResourceServiceClient().
		GetWorkloadIdentity(
			ctx, &workloadidentityv1.GetWorkloadIdentityRequest{Name: name},
		)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return resp, nil
}

// Create creates a Teleport WorkloadIdentity
func (l workloadIdentityClient) Create(
	ctx context.Context, resource *workloadidentityv1.WorkloadIdentity,
) error {
	_, err := l.teleportClient.
		WorkloadIdentityResourceServiceClient().
		CreateWorkloadIdentity(
			ctx,
			&workloadidentityv1.CreateWorkloadIdentityRequest{
				WorkloadIdentity: resource,
			},
		)
	return trace.Wrap(err)
}

// Update updates a Teleport WorkloadIdentity
func (l workloadIdentityClient) Update(
	ctx context.Context, resource *workloadidentityv1.WorkloadIdentity,
) error {
	_, err := l.teleportClient.
		WorkloadIdentityResourceServiceClient().
		UpsertWorkloadIdentity(
			ctx,
			&workloadidentityv1.UpsertWorkloadIdentityRequest{
				WorkloadIdentity: resource,
			},
		)
	return trace.Wrap(err)
}

// Delete deletes a Teleport WorkloadIdentity
func (l workloadIdentityClient) Delete(ctx context.Context, name string) error {
	_, err := l.teleportClient.
		WorkloadIdentityResourceServiceClient().
		DeleteWorkloadIdentity(
			ctx, &workloadidentityv1.DeleteWorkloadIdentityRequest{Name: name},
		)
	return trace.Wrap(err)
}

// NewWorkloadIdentityV1Reconciler instantiates a new Kubernetes controller
// reconciling WorkloadIdentity resources
func NewWorkloadIdentityV1Reconciler(client kclient.Client, tClient *client.Client) (controllers.Reconciler, error) {
	workloadIdentityClient := &workloadIdentityClient{
		teleportClient: tClient,
	}

	resourceReconciler, err := reconcilers.NewTeleportResource153Reconciler[
		*workloadidentityv1.WorkloadIdentity, *resourcesv1.TeleportWorkloadIdentityV1,
	](
		client,
		workloadIdentityClient,
	)

	return resourceReconciler, trace.Wrap(err, "building teleport resource reconciler")
}
