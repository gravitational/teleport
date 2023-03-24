/*
Copyright 2022 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package resources

import (
	"context"

	"github.com/gravitational/trace"
	kclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gravitational/teleport/api/types"
	resourcesv3 "github.com/gravitational/teleport/integrations/operator/apis/resources/v3"
	"github.com/gravitational/teleport/integrations/operator/sidecar"
)

// oidcConnectorClient implements TeleportResourceClient and offers CRUD methods needed to reconcile oidc_connectors
type oidcConnectorClient struct {
	TeleportClientAccessor sidecar.ClientAccessor
}

// Get gets the Teleport oidc_connector of a given name
func (r oidcConnectorClient) Get(ctx context.Context, name string) (types.OIDCConnector, error) {
	teleportClient, err := r.TeleportClientAccessor(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	oidc, err := teleportClient.GetOIDCConnector(ctx, name, false /* with secrets*/)
	return oidc, trace.Wrap(err)
}

// Create creates a Teleport oidc_connector
func (r oidcConnectorClient) Create(ctx context.Context, oidc types.OIDCConnector) error {
	teleportClient, err := r.TeleportClientAccessor(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	return trace.Wrap(teleportClient.UpsertOIDCConnector(ctx, oidc))
}

// Update updates a Teleport oidc_connector
func (r oidcConnectorClient) Update(ctx context.Context, oidc types.OIDCConnector) error {
	teleportClient, err := r.TeleportClientAccessor(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	return trace.Wrap(teleportClient.UpsertOIDCConnector(ctx, oidc))
}

// Delete deletes a Teleport oidc_connector
func (r oidcConnectorClient) Delete(ctx context.Context, name string) error {
	teleportClient, err := r.TeleportClientAccessor(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	return trace.Wrap(teleportClient.DeleteOIDCConnector(ctx, name))
}

// NewOIDCConnectorReconciler instantiates a new Kubernetes controller reconciling oidc_connector resources
func NewOIDCConnectorReconciler(client kclient.Client, accessor sidecar.ClientAccessor) *TeleportResourceReconciler[types.OIDCConnector, *resourcesv3.TeleportOIDCConnector] {
	oidcClient := &oidcConnectorClient{
		TeleportClientAccessor: accessor,
	}

	resourceReconciler := NewTeleportResourceReconciler[types.OIDCConnector, *resourcesv3.TeleportOIDCConnector](
		client,
		oidcClient,
	)

	return resourceReconciler
}
