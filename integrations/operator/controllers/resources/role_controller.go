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

	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/types"
	v5 "github.com/gravitational/teleport/integrations/operator/apis/resources/v5"
)

// roleClient implements TeleportResourceClient and offers CRUD methods needed to reconcile roles
type roleClient struct {
	teleportClient *client.Client
}

// Get gets the Teleport role of a given name
func (r roleClient) Get(ctx context.Context, name string) (types.Role, error) {
	role, err := r.teleportClient.GetRole(ctx, name)
	return role, trace.Wrap(err)
}

// Create creates a Teleport role
func (r roleClient) Create(ctx context.Context, role types.Role) error {
	_, err := r.teleportClient.UpsertRole(ctx, role)
	return trace.Wrap(err)
}

// Update updates a Teleport role
func (r roleClient) Update(ctx context.Context, role types.Role) error {
	_, err := r.teleportClient.UpsertRole(ctx, role)
	return trace.Wrap(err)
}

// Delete deletes a Teleport role
func (r roleClient) Delete(ctx context.Context, name string) error {
	return trace.Wrap(r.teleportClient.DeleteRole(ctx, name))
}

// NewRoleReconciler instantiates a new Kubernetes controller reconciling role resources
func NewRoleReconciler(client kclient.Client, tClient *client.Client) *TeleportResourceReconciler[types.Role, *v5.TeleportRole] {
	roleClient := &roleClient{
		teleportClient: tClient,
	}

	resourceReconciler := NewTeleportResourceReconciler[types.Role, *v5.TeleportRole](
		client,
		roleClient,
	)

	return resourceReconciler
}
