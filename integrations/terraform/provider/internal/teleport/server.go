// Teleport
// Copyright (C) 2026 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package teleport

import (
	"context"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/client"
	presencev1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/presence/v1"
	"github.com/gravitational/teleport/api/types"

	"github.com/gravitational/teleport/integrations/terraform/provider/internal/tfdriver"
)

// NewServerClient returns a Teleport client to interact with SSH server resources.
func NewServerClient(c *client.Client) ServerClient {
	return ServerClient{client: c}
}

// ServerClient manages SSH server resources in Teleport.
type ServerClient struct {
	client *client.Client
}

// Get reads an SSH server by scope-qualified name.
func (r ServerClient) Get(ctx context.Context, id tfdriver.ScopeQualifiedNameIdentifier) (*types.ServerV2, error) {
	server, err := r.client.GetSSHServer(ctx, presencev1.GetSSHServerRequest_builder{
		Name:  id.Name,
		Scope: id.Scope,
	}.Build())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	serverV2, ok := server.(*types.ServerV2)
	if !ok {
		return nil, trace.BadParameter("unexpected SSH server type: %T", server)
	}

	return serverV2, nil
}

// Create creates an SSH server resource in Teleport.
func (r ServerClient) Create(ctx context.Context, server *types.ServerV2) error {
	_, err := r.client.UpsertNode(ctx, server)
	return trace.Wrap(err)
}

// Upsert updates an SSH server resource in Teleport.
func (r ServerClient) Upsert(ctx context.Context, server *types.ServerV2) error {
	_, err := r.client.UpsertNode(ctx, server)
	return trace.Wrap(err)
}

// Delete deletes an SSH server resource from Teleport by scope-qualified name.
func (r ServerClient) Delete(ctx context.Context, id tfdriver.ScopeQualifiedNameIdentifier) error {
	return trace.Wrap(r.client.DeleteSSHServer(ctx, presencev1.DeleteSSHServerRequest_builder{
		Name:  id.Name,
		Scope: id.Scope,
	}.Build()))
}
