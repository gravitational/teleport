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
	"github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	resourcesv1 "github.com/gravitational/teleport/integrations/operator/apis/resources/v1"
)

// opensshEICEServerClient implements TeleportResourceClient and offers CRUD methods needed to reconcile provision tokens
type opensshEICEServerClient struct {
	teleportClient *client.Client
}

// Get gets the Teleport provision token of a given name
func (r opensshEICEServerClient) Get(ctx context.Context, name string) (types.Server, error) {
	server, err := r.teleportClient.GetNode(ctx, defaults.Namespace, name)
	if err != nil {
		return server, trace.Wrap(err)
	}
	if subKind := server.GetSubKind(); subKind != types.SubKindOpenSSHEICENode {
		return nil, trace.CompareFailed(
			"Wrong server subKind, was expecting %q, got %q",
			types.SubKindOpenSSHEICENode,
			subKind,
		)
	}
	return server, nil
}

// Create creates a Teleport provision token
func (r opensshEICEServerClient) Create(ctx context.Context, server types.Server) error {
	_, err := r.teleportClient.UpsertNode(ctx, server)
	return trace.Wrap(err)
}

// Update updates a Teleport provision token
func (r opensshEICEServerClient) Update(ctx context.Context, server types.Server) error {
	_, err := r.teleportClient.UpsertNode(ctx, server)
	return trace.Wrap(err)
}

// Delete deletes a Teleport provision token
func (r opensshEICEServerClient) Delete(ctx context.Context, name string) error {
	return trace.Wrap(r.teleportClient.DeleteNode(ctx, defaults.Namespace, name))
}

// NewOpensshEICEServerV2Reconciler instantiates a new Kubernetes controller reconciling provision token resources
func NewOpensshEICEServerV2Reconciler(client kclient.Client, tClient *client.Client) (Reconciler, error) {
	serverClient := &opensshEICEServerClient{
		teleportClient: tClient,
	}

	resourceReconciler, err := NewTeleportResourceReconciler[types.Server, *resourcesv1.TeleportOpensshEICEServerV2](
		client,
		serverClient,
	)

	return resourceReconciler, trace.Wrap(err, "building teleport resource reconciler")
}
