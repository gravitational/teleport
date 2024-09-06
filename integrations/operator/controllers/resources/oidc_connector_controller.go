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
	resourcesv3 "github.com/gravitational/teleport/integrations/operator/apis/resources/v3"
	"github.com/gravitational/teleport/integrations/operator/controllers"
	"github.com/gravitational/teleport/integrations/operator/controllers/reconcilers"
)

// oidcConnectorClient implements TeleportResourceClient and offers CRUD methods needed to reconcile oidc_connectors
type oidcConnectorClient struct {
	teleportClient *client.Client
}

// Get gets the Teleport oidc_connector of a given name
func (r oidcConnectorClient) Get(ctx context.Context, name string) (types.OIDCConnector, error) {
	oidc, err := r.teleportClient.GetOIDCConnector(ctx, name, false /* with secrets*/)
	return oidc, trace.Wrap(err)
}

// Create creates a Teleport oidc_connector
func (r oidcConnectorClient) Create(ctx context.Context, oidc types.OIDCConnector) error {
	_, err := r.teleportClient.CreateOIDCConnector(ctx, oidc)
	return trace.Wrap(err)
}

// Update updates a Teleport oidc_connector
func (r oidcConnectorClient) Update(ctx context.Context, oidc types.OIDCConnector) error {
	_, err := r.teleportClient.UpsertOIDCConnector(ctx, oidc)
	return trace.Wrap(err)
}

// Delete deletes a Teleport oidc_connector
func (r oidcConnectorClient) Delete(ctx context.Context, name string) error {
	return trace.Wrap(r.teleportClient.DeleteOIDCConnector(ctx, name))
}

// NewOIDCConnectorReconciler instantiates a new Kubernetes controller reconciling oidc_connector resources
func NewOIDCConnectorReconciler(client kclient.Client, tClient *client.Client) (controllers.Reconciler, error) {
	oidcClient := &oidcConnectorClient{
		teleportClient: tClient,
	}

	resourceReconciler, err := reconcilers.NewTeleportResourceWithoutLabelsReconciler[types.OIDCConnector, *resourcesv3.TeleportOIDCConnector](
		client,
		oidcClient,
	)

	return resourceReconciler, trace.Wrap(err, "building teleport resource reconciler")
}
