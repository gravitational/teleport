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

// samlConnectorClient implements TeleportResourceClient and offers CRUD methods needed to reconcile saml_connectors
type samlConnectorClient struct {
	teleportClient *client.Client
}

// Get gets the Teleport saml_connector of a given name
func (r samlConnectorClient) Get(ctx context.Context, name string) (types.SAMLConnector, error) {
	saml, err := r.teleportClient.GetSAMLConnector(ctx, name, false /* with secrets*/)
	return saml, trace.Wrap(err)
}

// Create creates a Teleport saml_connector
func (r samlConnectorClient) Create(ctx context.Context, saml types.SAMLConnector) error {
	_, err := r.teleportClient.CreateSAMLConnector(ctx, saml)
	return trace.Wrap(err)
}

// Update updates a Teleport saml_connector
func (r samlConnectorClient) Update(ctx context.Context, saml types.SAMLConnector) error {
	_, err := r.teleportClient.UpsertSAMLConnector(ctx, saml)
	return trace.Wrap(err)
}

// Delete deletes a Teleport saml_connector
func (r samlConnectorClient) Delete(ctx context.Context, name string) error {
	return trace.Wrap(r.teleportClient.DeleteSAMLConnector(ctx, name))
}

// NewSAMLConnectorReconciler instantiates a new Kubernetes controller reconciling saml_connector resources
func NewSAMLConnectorReconciler(client kclient.Client, tClient *client.Client) (controllers.Reconciler, error) {
	samlClient := &samlConnectorClient{
		teleportClient: tClient,
	}

	resourceReconciler, err := reconcilers.NewTeleportResourceWithoutLabelsReconciler[types.SAMLConnector, *resourcesv2.TeleportSAMLConnector](
		client,
		samlClient,
	)

	return resourceReconciler, trace.Wrap(err, "building teleport resource reconciler")
}
