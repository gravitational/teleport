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
	"github.com/gravitational/teleport/api/types"
	resourcesv2 "github.com/gravitational/teleport/integrations/operator/apis/resources/v2"
	"github.com/gravitational/teleport/integrations/operator/controllers"
	"github.com/gravitational/teleport/integrations/operator/controllers/reconcilers"
)

// lockClient implements TeleportResourceClient and offers CRUD methods needed to reconcile locks
type lockClient struct {
	teleportClient *client.Client
}

// Get gets the Teleport lock of a given name
func (r lockClient) Get(ctx context.Context, name string) (types.Lock, error) {
	lock, err := r.teleportClient.GetLock(ctx, name)
	return lock, trace.Wrap(err)
}

// Create creates a Teleport lock
func (r lockClient) Create(ctx context.Context, lock types.Lock) error {
	return trace.Wrap(r.teleportClient.UpsertLock(ctx, lock))
}

// Update updates a Teleport lock
func (r lockClient) Update(ctx context.Context, lock types.Lock) error {
	return trace.Wrap(r.teleportClient.UpsertLock(ctx, lock))
}

// Delete deletes a Teleport lock
func (r lockClient) Delete(ctx context.Context, name string) error {
	return trace.Wrap(r.teleportClient.DeleteLock(ctx, name))
}

// NewLockReconciler instantiates a new Kubernetes controller reconciling lock resources
func NewLockReconciler(client kclient.Client, tClient *client.Client) (controllers.Reconciler, error) {
	lockClient := &lockClient{
		teleportClient: tClient,
	}

	resourceReconciler, err := reconcilers.NewTeleportResourceWithoutLabelsReconciler[types.Lock, *resourcesv2.TeleportLock](
		client,
		lockClient,
	)

	return resourceReconciler, trace.Wrap(err, "building teleport resource reconciler")
}
