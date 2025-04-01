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

// samlidpClient implements TeleportResourceClient for SAML IdP Service Providers.
type samlidpClient struct {
	teleportClient *client.Client
}

// Get retrieves the Teleport SAML IdP Service Provider by name.
func (r samlidpClient) Get(ctx context.Context, name string) (types.SAMLIdPServiceProvider, error) {
	obj, err := r.teleportClient.GetSAMLIdPServiceProvider(ctx, name)
	return obj, trace.Wrap(err)
}

// Create creates a new Teleport SAML IdP Service Provider.
func (r samlidpClient) Create(ctx context.Context, obj types.SAMLIdPServiceProvider) error {
	err := r.teleportClient.CreateSAMLIdPServiceProvider(ctx, obj)
	return trace.Wrap(err)
}

// Update updates an existing Teleport SAML IdP Service Provider.
func (r samlidpClient) Update(ctx context.Context, obj types.SAMLIdPServiceProvider) error {
	err := r.teleportClient.UpdateSAMLIdPServiceProvider(ctx, obj)
	return trace.Wrap(err)
}

// Delete deletes a Teleport SAML IdP Service Provider by name.
func (r samlidpClient) Delete(ctx context.Context, name string) error {
	return trace.Wrap(r.teleportClient.DeleteSAMLIdPServiceProvider(ctx, name))
}

// NewSAMLIdPProviderReconciler instantiates a new Kubernetes controller reconciling Teleport
// SAML IdP Service Provider custom resources.
func NewSAMLIdPProviderReconciler(client kclient.Client, tClient *client.Client) (controllers.Reconciler, error) {
	samlidpClient := &samlidpClient{
		teleportClient: tClient,
	}

	resourceReconciler, err := reconcilers.NewTeleportResourceWithLabelsReconciler[types.SAMLIdPServiceProvider, *resourcesv1.TeleportSAMLIdPServiceProvider](
		client,
		samlidpClient,
	)
	return resourceReconciler, trace.Wrap(err, "building teleport resource reconciler")
}
