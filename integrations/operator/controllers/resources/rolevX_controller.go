/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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
	resourcesv5 "github.com/gravitational/teleport/integrations/operator/apis/resources/v5"
	"github.com/gravitational/teleport/integrations/operator/controllers"
	"github.com/gravitational/teleport/integrations/operator/controllers/reconcilers"
)

// roleClient implements TeleportResourceClient and offers CRUD methods needed to reconcile roles
// Currently the same client is used by all role versions. If we need to treat
// them differently at some point, for example by adding a Mutate function
// functions, we can always split the client into separate clients.
type roleClient struct {
	teleportClient *client.Client
}

// Get gets the Teleport role of a given name
func (r roleClient) Get(ctx context.Context, name string) (types.Role, error) {
	role, err := r.teleportClient.GetRole(ctx, name)
	return role, trace.Wrap(err)
}

// Create creates a Teleport role
func (r roleClient) Create(ctx context.Context, role types.Role) error {
	_, err := r.teleportClient.UpsertRole(ctx, role)
	return trace.Wrap(err)
}

// Update updates a Teleport role
func (r roleClient) Update(ctx context.Context, role types.Role) error {
	_, err := r.teleportClient.UpsertRole(ctx, role)
	return trace.Wrap(err)
}

// Delete deletes a Teleport role
func (r roleClient) Delete(ctx context.Context, name string) error {
	return trace.Wrap(r.teleportClient.DeleteRole(ctx, name))
}

// NewRoleReconciler instantiates a new Kubernetes controller reconciling legacy role v5 resources
func NewRoleReconciler(client kclient.Client, tClient *client.Client) (controllers.Reconciler, error) {
	roleClient := &roleClient{
		teleportClient: tClient,
	}

	resourceReconciler, err := reconcilers.NewTeleportResourceWithLabelsReconciler[types.Role, *resourcesv5.TeleportRole](
		client,
		roleClient,
	)

	return resourceReconciler, trace.Wrap(err, "building teleport resource reconciler")
}

// NewRoleV6Reconciler instantiates a new Kubernetes controller reconciling role v6 resources
func NewRoleV6Reconciler(client kclient.Client, tClient *client.Client) (controllers.Reconciler, error) {
	roleClient := &roleClient{
		teleportClient: tClient,
	}

	resourceReconciler, err := reconcilers.NewTeleportResourceWithLabelsReconciler[types.Role, *resourcesv1.TeleportRoleV6](
		client,
		roleClient,
	)

	return resourceReconciler, trace.Wrap(err, "building teleport resource reconciler")
}

// NewRoleV7Reconciler instantiates a new Kubernetes controller reconciling role v7 resources
func NewRoleV7Reconciler(client kclient.Client, tClient *client.Client) (controllers.Reconciler, error) {
	roleClient := &roleClient{
		teleportClient: tClient,
	}

	resourceReconciler, err := reconcilers.NewTeleportResourceWithLabelsReconciler[types.Role, *resourcesv1.TeleportRoleV7](
		client,
		roleClient,
	)

	return resourceReconciler, trace.Wrap(err, "building teleport resource reconciler")
}

// NewRoleV8Reconciler instantiates a new Kubernetes controller reconciling role v8 resources
func NewRoleV8Reconciler(client kclient.Client, tClient *client.Client) (controllers.Reconciler, error) {
	roleClient := &roleClient{
		teleportClient: tClient,
	}

	resourceReconciler, err := reconcilers.NewTeleportResourceWithLabelsReconciler[types.Role, *resourcesv1.TeleportRoleV8](
		client,
		roleClient,
	)

	return resourceReconciler, trace.Wrap(err, "building teleport resource reconciler")
}
