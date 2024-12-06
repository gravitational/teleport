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
	"github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	resourcesv1 "github.com/gravitational/teleport/integrations/operator/apis/resources/v1"
	"github.com/gravitational/teleport/integrations/operator/controllers"
	"github.com/gravitational/teleport/integrations/operator/controllers/reconcilers"
)

// databaseClient implements TeleportResourceClient and offers CRUD methods
// needed to reconcile databases.
type databaseClient struct {
	teleportClient *client.Client
}

// Get gets the Teleport database of a given name.
func (r databaseClient) Get(ctx context.Context, name string) (types.Database, error) {
	database, err := r.teleportClient.GetDatabase(ctx, name)
	if err != nil {
		return database, trace.Wrap(err)
	}
	return database, nil
}

// Create creates a Teleport database.
func (r databaseClient) Create(ctx context.Context, database types.Database) error {
	err := r.teleportClient.CreateDatabase(ctx, database)
	return trace.Wrap(err)
}

// Update updates a Teleport database.
func (r databaseClient) Update(ctx context.Context, database types.Database) error {
	err := r.teleportClient.UpdateDatabase(ctx, database)
	return trace.Wrap(err)
}

// Delete deletes a Teleport database.
func (r databaseClient) Delete(ctx context.Context, name string) error {
	return trace.Wrap(r.teleportClient.DeleteNode(ctx, defaults.Namespace, name))
}

// NewDatabaseV3Reconciler instantiates a new Kubernetes controller
// reconciling database resources.
func NewDatabaseV3Reconciler(client kclient.Client, tClient *client.Client) (controllers.Reconciler, error) {
	databaseClient := &databaseClient{
		teleportClient: tClient,
	}

	resourceReconciler, err := reconcilers.NewTeleportResourceWithLabelsReconciler[types.Database, *resourcesv1.TeleportDatabaseV3](
		client,
		databaseClient,
	)

	return resourceReconciler, trace.Wrap(err, "building teleport resource reconciler")
}
