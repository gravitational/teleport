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
	resourcesv2 "github.com/gravitational/teleport/integrations/operator/apis/resources/v2"
	"github.com/gravitational/teleport/integrations/operator/sidecar"
)

// samlConnectorClient implements TeleportResourceClient and offers CRUD methods needed to reconcile saml_connectors
type samlConnectorClient struct {
	TeleportClientAccessor sidecar.ClientAccessor
}

// Get gets the Teleport saml_connector of a given name
func (r samlConnectorClient) Get(ctx context.Context, name string) (types.SAMLConnector, error) {
	teleportClient, err := r.TeleportClientAccessor(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	saml, err := teleportClient.GetSAMLConnector(ctx, name, false /* with secrets*/)
	return saml, trace.Wrap(err)
}

// Create creates a Teleport saml_connector
func (r samlConnectorClient) Create(ctx context.Context, saml types.SAMLConnector) error {
	teleportClient, err := r.TeleportClientAccessor(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	return trace.Wrap(teleportClient.UpsertSAMLConnector(ctx, saml))
}

// Update updates a Teleport saml_connector
func (r samlConnectorClient) Update(ctx context.Context, saml types.SAMLConnector) error {
	teleportClient, err := r.TeleportClientAccessor(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	return trace.Wrap(teleportClient.UpsertSAMLConnector(ctx, saml))
}

// Delete deletes a Teleport saml_connector
func (r samlConnectorClient) Delete(ctx context.Context, name string) error {
	teleportClient, err := r.TeleportClientAccessor(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	return trace.Wrap(teleportClient.DeleteSAMLConnector(ctx, name))
}

// NewSAMLConnectorReconciler instantiates a new Kubernetes controller reconciling saml_connector resources
func NewSAMLConnectorReconciler(client kclient.Client, accessor sidecar.ClientAccessor) *TeleportResourceReconciler[types.SAMLConnector, *resourcesv2.TeleportSAMLConnector] {
	samlClient := &samlConnectorClient{
		TeleportClientAccessor: accessor,
	}

	resourceReconciler := NewTeleportResourceReconciler[types.SAMLConnector, *resourcesv2.TeleportSAMLConnector](
		client,
		samlClient,
	)

	return resourceReconciler
}
