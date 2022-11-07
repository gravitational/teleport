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
	resourcesv3 "github.com/gravitational/teleport/operator/apis/resources/v3"
	"github.com/gravitational/teleport/operator/sidecar"
)

// GithubConnectorClient implements TeleportResourceClient and offers CRUD methods needed to reconcile github_connectors
type GithubConnectorClient struct {
	TeleportClientAccessor sidecar.ClientAccessor
}

// Get the Teleport github_connector of a given name
func (r GithubConnectorClient) Get(ctx context.Context, name string) (types.GithubConnector, error) {
	teleportClient, err := r.TeleportClientAccessor(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return teleportClient.GetGithubConnector(ctx, name, false /* with secrets*/)
}

// Create a Teleport github_connector
func (r GithubConnectorClient) Create(ctx context.Context, github types.GithubConnector) error {
	teleportClient, err := r.TeleportClientAccessor(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	return teleportClient.UpsertGithubConnector(ctx, github)
}

// Update a Teleport github_connector
func (r GithubConnectorClient) Update(ctx context.Context, github types.GithubConnector) error {
	teleportClient, err := r.TeleportClientAccessor(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	return teleportClient.UpsertGithubConnector(ctx, github)
}

// Delete a Teleport github_connector
func (r GithubConnectorClient) Delete(ctx context.Context, name string) error {
	teleportClient, err := r.TeleportClientAccessor(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	return teleportClient.DeleteGithubConnector(ctx, name)
}

// NewGithubConnectorReconciler instantiates a new Kubernetes controller reconciling github_connector resources
func NewGithubConnectorReconciler(client kclient.Client, accessor sidecar.ClientAccessor) *TeleportResourceReconciler[types.GithubConnector, *resourcesv3.TeleportGithubConnector] {
	githubClient := &GithubConnectorClient{
		TeleportClientAccessor: accessor,
	}

	resourceReconciler := NewTeleportResourceReconciler[types.GithubConnector, *resourcesv3.TeleportGithubConnector](
		client,
		githubClient,
		&resourcesv3.TeleportGithubConnector{},
	)

	return resourceReconciler
}
