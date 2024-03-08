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
	"github.com/gravitational/teleport/integrations/operator/controllers"
	"github.com/gravitational/teleport/integrations/operator/controllers/reconcilers"
)

// openSSHEICEServerClient implements TeleportResourceClient and offers CRUD
// methods needed to reconcile OpenSSH EC2 ICE servers.
type openSSHEICEServerClient struct {
	teleportClient *client.Client
}

// Get gets the Teleport OpenSSHEICE server of a given name.
func (r openSSHEICEServerClient) Get(ctx context.Context, name string) (types.Server, error) {
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

// Create creates a Teleport OpenSSHEICE server.
func (r openSSHEICEServerClient) Create(ctx context.Context, server types.Server) error {
	_, err := r.teleportClient.UpsertNode(ctx, server)
	return trace.Wrap(err)
}

// Update updates a Teleport OpenSSHEICE server.
func (r openSSHEICEServerClient) Update(ctx context.Context, server types.Server) error {
	_, err := r.teleportClient.UpsertNode(ctx, server)
	return trace.Wrap(err)
}

// Delete deletes a Teleport OpenSSHEICE server.
func (r openSSHEICEServerClient) Delete(ctx context.Context, name string) error {
	return trace.Wrap(r.teleportClient.DeleteNode(ctx, defaults.Namespace, name))
}

// NewOpenSSHEICEServerV2Reconciler instantiates a new Kubernetes controller
// reconciling OpenSSHEICE server resources.
func NewOpenSSHEICEServerV2Reconciler(client kclient.Client, tClient *client.Client) (controllers.Reconciler, error) {
	serverClient := &openSSHEICEServerClient{
		teleportClient: tClient,
	}

	resourceReconciler, err := reconcilers.NewTeleportResourceWithLabelsReconciler[types.Server, *resourcesv1.TeleportOpenSSHEICEServerV2](
		client,
		serverClient,
	)

	return resourceReconciler, trace.Wrap(err, "building teleport resource reconciler")
}
