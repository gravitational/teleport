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

// provisionTokenClient implements TeleportResourceClient and offers CRUD methods needed to reconcile provision tokens
type provisionTokenClient struct {
	TeleportClientAccessor sidecar.ClientAccessor
}

// Get gets the Teleport provision token of a given name
func (r provisionTokenClient) Get(ctx context.Context, name string) (types.ProvisionToken, error) {
	teleportClient, err := r.TeleportClientAccessor(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	token, err := teleportClient.GetToken(ctx, name)
	return token, trace.Wrap(err)
}

// Create creates a Teleport provision token
func (r provisionTokenClient) Create(ctx context.Context, token types.ProvisionToken) error {
	teleportClient, err := r.TeleportClientAccessor(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	return trace.Wrap(teleportClient.UpsertToken(ctx, token))
}

// Update updates a Teleport provision token
func (r provisionTokenClient) Update(ctx context.Context, token types.ProvisionToken) error {
	teleportClient, err := r.TeleportClientAccessor(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	return trace.Wrap(teleportClient.UpsertToken(ctx, token))
}

// Delete deletes a Teleport provision token
func (r provisionTokenClient) Delete(ctx context.Context, name string) error {
	teleportClient, err := r.TeleportClientAccessor(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	return trace.Wrap(teleportClient.DeleteToken(ctx, name))
}

// NewProvisionTokenReconciler instantiates a new Kubernetes controller reconciling provision token resources
func NewProvisionTokenReconciler(client kclient.Client, accessor sidecar.ClientAccessor) *TeleportResourceReconciler[types.ProvisionToken, *resourcesv2.TeleportProvisionToken] {
	tokenClient := &provisionTokenClient{
		TeleportClientAccessor: accessor,
	}

	resourceReconciler := NewTeleportResourceReconciler[types.ProvisionToken, *resourcesv2.TeleportProvisionToken](
		client,
		tokenClient,
	)

	return resourceReconciler
}
