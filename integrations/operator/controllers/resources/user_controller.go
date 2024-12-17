/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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
	resourcesv2 "github.com/gravitational/teleport/integrations/operator/apis/resources/v2"
	"github.com/gravitational/teleport/integrations/operator/controllers"
	"github.com/gravitational/teleport/integrations/operator/controllers/reconcilers"
)

// userClient implements TeleportResourceClient and offers CRUD methods needed to reconcile users
type userClient struct {
	teleportClient *client.Client
}

// Get gets the Teleport user of a given name
func (r userClient) Get(ctx context.Context, name string) (types.User, error) {
	user, err := r.teleportClient.GetUser(ctx, name, false /* with secrets*/)
	return user, trace.Wrap(err)
}

// Create creates a Teleport user
func (r userClient) Create(ctx context.Context, user types.User) error {
	_, err := r.teleportClient.CreateUser(ctx, user)
	return trace.Wrap(err)
}

// Update updates a Teleport user
func (r userClient) Update(ctx context.Context, user types.User) error {
	_, err := r.teleportClient.UpdateUser(ctx, user)
	return trace.Wrap(err)
}

// Delete deletes a Teleport user
func (r userClient) Delete(ctx context.Context, name string) error {
	return trace.Wrap(r.teleportClient.DeleteUser(ctx, name))
}

// Mutate ensures the spec.createdBy property is persisted
func (r userClient) Mutate(_ context.Context, newUser, existingUser types.User, _ kclient.ObjectKey) error {
	if existingUser != nil {
		newUser.SetCreatedBy(existingUser.GetCreatedBy())
	}
	return nil
}

// NewUserReconciler instantiates a new Kubernetes controller reconciling user resources
func NewUserReconciler(client kclient.Client, tClient *client.Client) (controllers.Reconciler, error) {
	userClient := &userClient{
		teleportClient: tClient,
	}

	resourceReconciler, err := reconcilers.NewTeleportResourceWithLabelsReconciler[types.User, *resourcesv2.TeleportUser](
		client,
		userClient,
	)

	return resourceReconciler, trace.Wrap(err, "building teleport resource reconciler")
}
