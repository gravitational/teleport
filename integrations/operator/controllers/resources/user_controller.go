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

// userClient implements TeleportResourceClient and offers CRUD methods needed to reconcile users
type userClient struct {
	TeleportClientAccessor sidecar.ClientAccessor
}

// Get gets the Teleport user of a given name
func (r userClient) Get(ctx context.Context, name string) (types.User, error) {
	teleportClient, err := r.TeleportClientAccessor(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	user, err := teleportClient.GetUser(name, false /* with secrets*/)
	return user, trace.Wrap(err)
}

// Create creates a Teleport user
func (r userClient) Create(ctx context.Context, user types.User) error {
	teleportClient, err := r.TeleportClientAccessor(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	return trace.Wrap(teleportClient.CreateUser(ctx, user))
}

// Update updates a Teleport user
func (r userClient) Update(ctx context.Context, user types.User) error {
	teleportClient, err := r.TeleportClientAccessor(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	return trace.Wrap(teleportClient.UpdateUser(ctx, user))
}

// Delete deletes a Teleport user
func (r userClient) Delete(ctx context.Context, name string) error {
	teleportClient, err := r.TeleportClientAccessor(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	return trace.Wrap(teleportClient.DeleteUser(ctx, name))
}

// Mutate ensures the spec.createdBy property is persisted
func (r userClient) Mutate(newUser, existingUser types.User) {
	if existingUser != nil {
		newUser.SetCreatedBy(existingUser.GetCreatedBy())
	}
}

// NewUserReconciler instantiates a new Kubernetes controller reconciling user resources
func NewUserReconciler(client kclient.Client, accessor sidecar.ClientAccessor) *TeleportResourceReconciler[types.User, *resourcesv2.TeleportUser] {
	userClient := &userClient{
		TeleportClientAccessor: accessor,
	}

	resourceReconciler := NewTeleportResourceReconciler[types.User, *resourcesv2.TeleportUser](
		client,
		userClient,
	)

	return resourceReconciler
}
