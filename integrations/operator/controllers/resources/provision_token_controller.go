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

// provisionTokenClient implements TeleportResourceClient and offers CRUD methods needed to reconcile provision tokens
type provisionTokenClient struct {
	teleportClient *client.Client
}

// Get gets the Teleport provision token of a given name
func (r provisionTokenClient) Get(ctx context.Context, name string) (types.ProvisionToken, error) {
	token, err := r.teleportClient.GetToken(ctx, name)
	return token, trace.Wrap(err)
}

// Create creates a Teleport provision token
func (r provisionTokenClient) Create(ctx context.Context, token types.ProvisionToken) error {
	return trace.Wrap(r.teleportClient.UpsertToken(ctx, token))
}

// Update updates a Teleport provision token
func (r provisionTokenClient) Update(ctx context.Context, token types.ProvisionToken) error {
	return trace.Wrap(r.teleportClient.UpsertToken(ctx, token))
}

// Delete deletes a Teleport provision token
func (r provisionTokenClient) Delete(ctx context.Context, name string) error {
	return trace.Wrap(r.teleportClient.DeleteToken(ctx, name))
}

// NewProvisionTokenReconciler instantiates a new Kubernetes controller reconciling provision token resources
func NewProvisionTokenReconciler(client kclient.Client, tClient *client.Client) (controllers.Reconciler, error) {
	tokenClient := &provisionTokenClient{
		teleportClient: tClient,
	}

	resourceReconciler, err := reconcilers.NewTeleportResourceWithoutLabelsReconciler[types.ProvisionToken, *resourcesv2.TeleportProvisionToken](
		client,
		tokenClient,
	)

	return resourceReconciler, trace.Wrap(err, "building teleport resource reconciler")
}
