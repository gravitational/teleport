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
	"github.com/gravitational/teleport/integrations/operator/controllers"
	"github.com/gravitational/teleport/integrations/operator/controllers/reconcilers"
)

// appClient implements TeleportResourceClient and offers CRUD methods needed to reconcile apps
// Currently the same client is used by all app versions. If we need to treat
// them differently at some point, for example by adding a Mutate function
// functions, we can always split the client into separate clients.
type appClient struct {
	teleportClient *client.Client
}

// Get gets the Teleport app of a given name
func (r appClient) Get(ctx context.Context, name string) (types.Application, error) {
	app, err := r.teleportClient.GetApp(ctx, name)
	return app, trace.Wrap(err)
}

// Create creates a Teleport app
func (r appClient) Create(ctx context.Context, app types.Application) error {
	return trace.Wrap(r.teleportClient.CreateApp(ctx, app))
}

// Update updates a Teleport app
func (r appClient) Update(ctx context.Context, app types.Application) error {
	return trace.Wrap(r.teleportClient.UpdateApp(ctx, app))
}

// Delete deletes a Teleport app
func (r appClient) Delete(ctx context.Context, name string) error {
	return trace.Wrap(r.teleportClient.DeleteApp(ctx, name))
}

// NewAppV3Reconciler instantiates a new Kubernetes controller reconciling app v6 resources
func NewAppV3Reconciler(client kclient.Client, tClient *client.Client) (controllers.Reconciler, error) {
	appClient := &appClient{
		teleportClient: tClient,
	}

	resourceReconciler, err := reconcilers.NewTeleportResourceWithLabelsReconciler[types.Application, *resourcesv1.TeleportAppV3](
		client,
		appClient,
	)

	return resourceReconciler, trace.Wrap(err, "building teleport resource reconciler")
}
