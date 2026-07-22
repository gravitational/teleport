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
	resourcesv1 "github.com/gravitational/teleport/integrations/operator/apis/resources/v1"
	"github.com/gravitational/teleport/integrations/operator/controllers"
	"github.com/gravitational/teleport/integrations/operator/controllers/reconcilers"
)

// samlIdPServiceProviderClient implements TeleportResourceClient and offers
// CRUD methods needed to reconcile saml_idp_service_providers.
type samlIdPServiceProviderClient struct {
	teleportClient *client.Client
}

// Get gets the Teleport saml_idp_service_provider of a given name.
func (r samlIdPServiceProviderClient) Get(ctx context.Context, key reconcilers.ResourceKey) (types.SAMLIdPServiceProvider, error) {
	sp, err := r.teleportClient.GetSAMLIdPServiceProvider(ctx, key.Name)
	return sp, trace.Wrap(err)
}

// Create creates a Teleport saml_idp_service_provider.
func (r samlIdPServiceProviderClient) Create(ctx context.Context, sp types.SAMLIdPServiceProvider) error {
	return trace.Wrap(r.teleportClient.CreateSAMLIdPServiceProvider(ctx, sp))
}

// Update updates a Teleport saml_idp_service_provider.
func (r samlIdPServiceProviderClient) Update(ctx context.Context, sp types.SAMLIdPServiceProvider) error {
	return trace.Wrap(r.teleportClient.UpdateSAMLIdPServiceProvider(ctx, sp))
}

// Delete deletes a Teleport saml_idp_service_provider.
func (r samlIdPServiceProviderClient) Delete(ctx context.Context, key reconcilers.ResourceKey) error {
	return trace.Wrap(r.teleportClient.DeleteSAMLIdPServiceProvider(ctx, key.Name))
}

// NewSAMLIdPServiceProviderV1Reconciler instantiates a new Kubernetes controller
// reconciling saml_idp_service_provider resources.
func NewSAMLIdPServiceProviderV1Reconciler(client kclient.Client, tClient *client.Client) (controllers.Reconciler, error) {
	spClient := &samlIdPServiceProviderClient{
		teleportClient: tClient,
	}

	resourceReconciler, err := reconcilers.NewTeleportResourceWithLabelsReconciler[types.SAMLIdPServiceProvider, *resourcesv1.TeleportSAMLIdPServiceProviderV1](
		client,
		spClient,
		// Although the WebUi doesn't show "SAML Application (Generic)" for
		// oss builds when adding a resource due to the BuildType() check in
		// lib/auth/auth_with_roles.go, the API allows creating
		// saml_idp_service_provider objects using tctl for any build. We
		// therefore enable it here unconditionally to mirror tctl behavior.
		reconcilers.Config{},
	)

	return resourceReconciler, trace.Wrap(err, "building teleport resource reconciler")
}
